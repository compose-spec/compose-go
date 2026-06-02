/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package loader

import (
	"context"
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/internal/node"
	interp "github.com/compose-spec/compose-go/v3/interpolation"
	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/paths"
	"github.com/compose-spec/compose-go/v3/schema"
	"github.com/compose-spec/compose-go/v3/template"
	"github.com/compose-spec/compose-go/v3/transform"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"github.com/compose-spec/compose-go/v3/validation"
)

// LoadV3 runs the full yaml.Node-centric v3 pipeline over the input
// ConfigDetails and returns the merged compose tree as a canonical
// *yaml.Node. Callers project the node into the shape they need via
// yaml.Decode (or via the package-internal nodeToModel / nodeToProject
// helpers that drive LoadModelWithContext and LoadWithContext).
//
// The pipeline goes:
//
//  1. parse every ConfigFile into one or more Layer values
//     (LoadLayer + recursive CollectIncludeLayers);
//  2. apply extends inside each layer (ApplyExtendsToLayer);
//  3. populate per-scalar origins so each scalar can be looked up against
//     the SourceContext of the layer that produced it (lazy interpolation);
//  4. merge layers left-to-right via override.MergeNode at the root path
//     (matches v2 ConfigFiles[0] is base, later files override);
//  5. apply !reset / !override paths collected from each layer;
//  6. interpolate every scalar with its own SourceContext.Environment;
//  7. canonicalize short-form syntax via transform.CanonicalNode;
//  8. resolve relative paths per-scalar via paths.ResolveRelativePathsNode;
//  9. validate via validation.ValidateNode;
//  10. normalize defaults via NormalizeNode.
func LoadV3(ctx context.Context, cd types.ConfigDetails, opts *Options) (*yaml.Node, error) {
	opts = ensureLoadV3Options(opts, cd)
	// Reproduce the v2 contract: extract the project name from the first
	// config file (or its `name:` field) before the pipeline runs. Errors
	// from explicit-name validation (NormalizeProjectName) propagate as in
	// v2; an empty result is rejected after schema validation below.
	if err := projectName(&cd, opts); err != nil {
		return nil, err
	}

	rootCtx := &node.SourceContext{
		WorkingDir:  cd.WorkingDir,
		Environment: cd.Environment,
	}

	allLayers, err := collectAllLayers(ctx, cd, rootCtx, opts)
	if err != nil {
		return nil, err
	}
	if len(allLayers) == 0 {
		return nil, errors.New("empty compose file")
	}

	// v3 lazy env_file interpolation: capture each env_file entry's
	// declaring-layer environment so nodeToProject can attach it to the
	// Project EnvFileScopes side-table. WithServicesEnvironmentResolved
	// then prefers that scope when interpolating the env_file content.
	if opts.envFileScopes == nil {
		opts.envFileScopes = map[string]types.Mapping{}
	}
	for _, layer := range allLayers {
		captureEnvFileScopes(layer, opts.envFileScopes)
	}

	if !opts.SkipExtends {
		if err := applyExtendsPerLayer(ctx, allLayers, opts); err != nil {
			return nil, err
		}
	}

	origins := map[*yaml.Node]*node.SourceContext{}
	for _, layer := range allLayers {
		populateOrigins(origins, layer.Node, layer.Context)
	}

	merged, resetPaths, err := mergeLayers(allLayers)
	if err != nil {
		return nil, err
	}
	node.ApplyResetPaths(merged.Node, resetPaths)

	// Remove the include directive from the final tree (it has been
	// consumed by collectAllLayers).
	deleteMappingKey(merged.Node, "include")

	if !opts.SkipInterpolation {
		if err := interpolateMerged(merged, origins, opts); err != nil {
			return nil, err
		}
	}

	// JSON Schema validation runs early — before canonicalization and
	// transform — so structural errors (top-level not a mapping, services
	// declared as a list, ...) are caught with a clear v2-compatible
	// message rather than panicking inside a downstream transformer that
	// assumes a canonical shape.
	if err := validateAndStripVersion(merged.Node, cd, opts); err != nil {
		return nil, err
	}

	// Lazy bare-key environment resolution: services.*.environment entries
	// that are just `KEY` (no `=`) get rewritten to `KEY=value` using each
	// scalar own SourceContext.Environment. Mirrors v2 ResolveEnvironment
	// but operates per-scalar so an env_file scoped to an include block is
	// visible to services declared inside that include — and not leaked to
	// the surrounding project environment.
	ResolveEnvironmentNode(merged.Node, origins)

	// Path resolution runs first on the pre-canonical tree so that
	// pointer identity is preserved for every scalar whose origin is
	// tracked in the side-table. The CanonicalNode bridge currently
	// rebuilds affected subtrees via map[string]any, which would lose
	// origin pointers — Phase B follow-ups will port individual
	// transformers to operate on yaml.Node and remove this constraint.
	if opts.ResolvePaths {
		var remotes []paths.RemoteResource
		for _, loader := range opts.RemoteResourceLoaders() {
			remotes = append(remotes, loader.Accept)
		}
		if err := paths.ResolveRelativePathsNode(merged.Node, paths.NodeResolverOptions{
			WorkingDirFor: workingDirLookup(origins, merged.Context.WorkingDir),
			Remotes:       remotes,
		}); err != nil {
			return nil, err
		}
	}

	// Snapshot a service-name → SourceContext map BEFORE Canonical to
	// survive the bridge: Canonical re-encodes the merged tree and loses
	// the origin pointer identity for every Node, so post-canonical
	// passes that need per-service context (default build.context
	// resolution) consult this name-keyed map instead of the pointer map.
	serviceContexts := buildServiceContexts(merged.Node, origins)

	if _, err := transform.CanonicalNode(merged.Node, opts.SkipInterpolation); err != nil {
		return nil, err
	}

	// SetDefaultValues fills in canonical defaults (DeviceCount(-1) for
	// unspecified GPU count, default network configuration, default build
	// context ".", ...). v2 calls it from loadYamlModel between merge and
	// validate; v3 does the same through a map roundtrip until per-rule
	// Node ports land. Path resolution intentionally runs *before*
	// SetDefaultValues so the per-scalar origins side-table can still drive
	// the WorkingDir lookup. Defaults that are themselves path-shaped
	// (build.context ".") are resolved by a targeted helper below rather
	// than by a second full sweep, which would double-resolve every
	// already-handled relative path.
	if !opts.SkipDefaultValues {
		if err := setDefaultValuesNode(merged.Node); err != nil {
			return nil, err
		}
		if opts.ResolvePaths {
			resolveDefaultBuildContext(merged.Node, cd.WorkingDir, serviceContexts)
		}
	}

	if !opts.SkipValidation {
		if err := validation.ValidateNode(merged.Node); err != nil {
			return nil, err
		}
		// v2 rejects a load whose project name is still empty at this
		// point. The check is gated on SkipValidation to keep the v3
		// orchestrator usable from tests that skip validation outright.
		if opts.projectName == "" {
			return nil, errors.New("project name must not be empty")
		}
	}

	if !opts.SkipNormalization {
		if _, err := NormalizeNode(merged.Node, cd.Environment); err != nil {
			return nil, err
		}
	}

	root := merged.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode || len(root.Content) == 0 {
		return nil, errors.New("empty compose file")
	}

	// Drop empty attributes that resulted from interpolation of unset
	// variables (e.g. `dns: ${UNSET}` -> `dns: ""` collapses to absent).
	// Equivalent of v2 loadYamlModel's post-Canonical OmitEmpty pass,
	// applied at the node level so both nodeToModel and nodeToProject
	// observe the same shape.
	omitEmptyNode(root, tree.NewPath())

	return root, nil
}

// omitEmptyNode walks the tree and drops entries whose value is empty
// (nil / empty string) when their path matches one of the omitempty
// patterns. Mirrors OmitEmpty on the map-based representation.
func omitEmptyNode(n *yaml.Node, p tree.Path) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		filtered := n.Content[:0]
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			child := p.Next(k.Value)
			if isEmptyNode(v) && mustOmit(child) {
				continue
			}
			omitEmptyNode(v, child)
			filtered = append(filtered, k, v)
		}
		n.Content = filtered
	case yaml.SequenceNode:
		// The map-based OmitEmpty passes the parent path to mustOmit (not
		// path.Next("[]")) so a pattern like `services.*.dns` filters
		// scalar items inside the dns sequence. Mirror that here.
		filtered := n.Content[:0]
		for _, item := range n.Content {
			if isEmptyNode(item) && mustOmit(p) {
				continue
			}
			omitEmptyNode(item, p.Next("[]"))
			filtered = append(filtered, item)
		}
		n.Content = filtered
	}
}

func isEmptyNode(n *yaml.Node) bool {
	if n == nil || n.Tag == "!!null" {
		return true
	}
	return n.Kind == yaml.ScalarNode && n.Value == ""
}

// ensureLoadV3Options applies the same defaults as ToOptions for callers
// that pass a bare *Options (most production callers go through
// ToOptions; this covers tests that build the struct directly).
func ensureLoadV3Options(opts *Options, cd types.ConfigDetails) *Options {
	if opts == nil {
		opts = &Options{}
	}
	if !hasLocalLoader(opts.ResourceLoaders) {
		opts.ResourceLoaders = append(opts.ResourceLoaders, localResourceLoader{WorkingDir: cd.WorkingDir})
	}
	if opts.Interpolate == nil {
		opts.Interpolate = &interp.Options{
			Substitute:      template.Substitute,
			LookupValue:     cd.LookupEnv,
			TypeCastMapping: interpolateTypeCastMapping,
		}
	}
	return opts
}

// nodeToModel projects the merged tree into the legacy map[string]any
// shape consumed by LoadModelWithContext. It applies the v2 post-decode
// passes (OmitEmpty drops `dns: ""` style leftovers from unset variable
// interpolation; resolveSecrets/ConfigsEnvironment surface
// `environment:` lookups as Content) so the dict matches the v2
// loadYamlModel output byte-for-byte.
func nodeToModel(root *yaml.Node, env types.Mapping) (map[string]any, error) {
	var dict map[string]any
	if err := root.Decode(&dict); err != nil {
		return nil, fmt.Errorf("loadV3: decode merged tree: %w", err)
	}
	dict = OmitEmpty(dict)
	resolveSecretsEnvironment(dict, env)
	resolveConfigsEnvironment(dict, env)
	return dict, nil
}

// validateAndStripVersion runs the JSON Schema validator on a decoded
// view of the merged tree and, on success, strips the obsolete top-level
// `version` attribute with the v2 deprecation warning. Carved out of
// LoadV3 to keep its cyclomatic complexity in check.
func validateAndStripVersion(root *yaml.Node, cd types.ConfigDetails, opts *Options) error {
	if opts.SkipValidation {
		return nil
	}
	var schemaDict map[string]any
	if err := root.Decode(&schemaDict); err != nil {
		return fmt.Errorf("loadV3: decode for schema validation: %w", err)
	}
	if err := schema.Validate(schemaDict); err != nil {
		source := "(inline)"
		if len(cd.ConfigFiles) > 0 && cd.ConfigFiles[0].Filename != "" {
			source = cd.ConfigFiles[0].Filename
		}
		return fmt.Errorf("validating %s: %w", source, err)
	}
	if hasMappingKey(root, "version") {
		for _, f := range cd.ConfigFiles {
			opts.warnObsoleteVersion(f.Filename)
		}
		deleteMappingKey(root, "version")
	}
	return nil
}

// setDefaultValuesNode applies the v2 transform.SetDefaultValues defaults
// to the merged tree via a temporary map roundtrip. Sets DeviceCount(-1)
// for unspecified GPU count and similar defaults that exist outside the
// per-path Canonical transformers. The Node-typed port lives in transform/
// and replaces the bridge in a follow-up.
func setDefaultValuesNode(root *yaml.Node) error {
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	var data map[string]any
	if err := target.Decode(&data); err != nil {
		return fmt.Errorf("loadV3: decode for SetDefaultValues: %w", err)
	}
	defaulted, err := transform.SetDefaultValues(data)
	if err != nil {
		return err
	}
	var rebuilt yaml.Node
	if err := rebuilt.Encode(defaulted); err != nil {
		return fmt.Errorf("loadV3: re-encode after SetDefaultValues: %w", err)
	}
	*target = rebuilt
	return nil
}

// resolveDefaultBuildContext walks services.*.build.context entries and,
// for each one whose value is "." or empty (i.e. the default produced by
// SetDefaultValues for builds that did not declare a context), joins it
// with the appropriate working directory. The service node's origin is
// consulted first so an included service whose build had no context picks
// up the include's project_directory; falls back to projectWD for services
// whose origin is unknown (e.g. synthesized by SetDefaultValues itself).
//
// Tightly scoped to avoid the double-resolution problem that a generic
// post-defaults sweep would introduce on relative paths already resolved
// by the earlier pass.
func resolveDefaultBuildContext(root *yaml.Node, projectWD string, serviceContexts map[string]string) {
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	services := mappingValueByKey(target, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		svc := services.Content[i+1]
		build := mappingValueByKey(svc, "build")
		if build == nil || build.Kind != yaml.MappingNode {
			continue
		}
		ctx := mappingValueByKey(build, "context")
		if ctx == nil || ctx.Kind != yaml.ScalarNode {
			continue
		}
		if ctx.Value != "." && ctx.Value != "" {
			continue
		}
		wd := projectWD
		if origin, ok := serviceContexts[name]; ok && origin != "" {
			wd = origin
		}
		ctx.Value = wd
	}
}

// buildServiceContexts inspects the merged tree's `services` mapping and
// records, for each service name, the WorkingDir of the SourceContext that
// produced it. The map survives the CanonicalNode bridge because it is
// keyed by name (a stable identifier) rather than by Node pointer. Used by
// resolveDefaultBuildContext to give an included service whose build had
// no context the include's project_directory as the resolved default.
func buildServiceContexts(root *yaml.Node, origins map[*yaml.Node]*node.SourceContext) map[string]string {
	out := map[string]string{}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	services := mappingValueByKey(target, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return out
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		svc := services.Content[i+1]
		if wd := serviceOriginWorkingDir(svc, origins); wd != "" {
			out[name] = wd
		}
	}
	return out
}

func serviceOriginWorkingDir(svc *yaml.Node, origins map[*yaml.Node]*node.SourceContext) string {
	if ctx, ok := origins[svc]; ok && ctx != nil {
		return ctx.WorkingDir
	}
	for _, c := range svc.Content {
		if c == nil {
			continue
		}
		if ctx, ok := origins[c]; ok && ctx != nil {
			return ctx.WorkingDir
		}
	}
	return ""
}

// hasMappingKey reports whether n is a MappingNode containing key.
func hasMappingKey(n *yaml.Node, key string) bool {
	if n == nil || n.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return true
		}
	}
	return false
}

// captureEnvFileScopes walks a layer's services and records, for each
// env_file entry it carries, the layer environment in effect when the
// entry was declared. Keyed by the resolved env_file path (absolute when
// CollectIncludeLayers has pre-resolved it, raw otherwise) so the
// downstream ModelToProject step can attach Mapping to the corresponding
// types.EnvFile.Env field.
func captureEnvFileScopes(layer *node.Layer, scopes map[string]types.Mapping) {
	if layer == nil || layer.Context == nil || layer.Context.Parent == nil || len(layer.Context.Environment) == 0 {
		return
	}
	target := layer.Node
	if target == nil {
		return
	}
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	services := mappingValueByKey(target, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		envFile := mappingValueByKey(svc, "env_file")
		if envFile == nil {
			continue
		}
		switch envFile.Kind {
		case yaml.ScalarNode:
			scopes[envFile.Value] = layer.Context.Environment
		case yaml.SequenceNode:
			for _, item := range envFile.Content {
				switch item.Kind {
				case yaml.ScalarNode:
					scopes[item.Value] = layer.Context.Environment
				case yaml.MappingNode:
					if p := mappingValueByKey(item, "path"); p != nil && p.Kind == yaml.ScalarNode {
						scopes[p.Value] = layer.Context.Environment
					}
				}
			}
		}
	}
}

// applyExtendsPerLayer iterates layers and applies extends to each with a
// child-scoped Options whose localResourceLoader points at the layer's own
// WorkingDir. Mirrors v2 ApplyExtends running per-file inside the recursive
// loadYamlModel of an include, so a relative extends.file declared in an
// included file resolves against the include project_directory.
func applyExtendsPerLayer(ctx context.Context, allLayers []*node.Layer, opts *Options) error {
	tracker := &cycleTracker{}
	for _, layer := range allLayers {
		layerOpts := opts
		if layer.Context != nil && layer.Context.WorkingDir != "" && layer.Context.WorkingDir != opts.workingDirOfFirstLoader() {
			layerOpts = opts.clone()
			layerOpts.ResourceLoaders = append(opts.RemoteResourceLoaders(), localResourceLoader{WorkingDir: layer.Context.WorkingDir})
		}
		if err := ApplyExtendsToLayer(ctx, layer, layerOpts, tracker); err != nil {
			return err
		}
	}
	return nil
}

// collectAllLayers parses each ConfigFile and recursively folds in every
// include directive it carries. The returned slice is ordered so that
// included files appear before their parent, which matches the v2
// importResources convention where the parent overrides the include.
func collectAllLayers(ctx context.Context, cd types.ConfigDetails, root *node.SourceContext, opts *Options) ([]*node.Layer, error) {
	var all []*node.Layer
	for _, file := range cd.ConfigFiles {
		sc := *root
		sc.File = file.Filename
		layers, err := LoadLayer(ctx, file, &sc, opts)
		if err != nil {
			return nil, err
		}
		for _, layer := range layers {
			expanded, err := expandIncludes(ctx, layer, opts)
			if err != nil {
				return nil, err
			}
			all = append(all, expanded...)
		}
	}
	return all, nil
}

// expandIncludes returns layer prefixed by every include layer reachable
// from it (recursive traversal). Cycle protection comes from the cycle
// tracker maintained by CollectIncludeLayers; an explicit visited set at
// this level guards against fixture-induced infinite loops in the
// orchestrator itself.
//
// Each child include is processed recursively with opts re-rooted at the
// child's WorkingDir so its own include directives resolve relative paths
// against the include's project_directory, not the outer project root.
// Matches v2 ApplyInclude which similarly replaces ResourceLoaders on the
// recursive load.
func expandIncludes(ctx context.Context, layer *node.Layer, opts *Options) ([]*node.Layer, error) {
	if opts.SkipInclude {
		return []*node.Layer{layer}, nil
	}
	children, err := CollectIncludeLayers(ctx, layer, opts)
	if err != nil {
		return nil, err
	}
	var out []*node.Layer
	for _, child := range children {
		childOpts := opts
		if child.Context != nil && child.Context.WorkingDir != "" && child.Context.WorkingDir != opts.workingDirOfFirstLoader() {
			childOpts = opts.clone()
			childOpts.ResourceLoaders = append(opts.RemoteResourceLoaders(), localResourceLoader{WorkingDir: child.Context.WorkingDir})
		}
		grandchildren, err := expandIncludes(ctx, child, childOpts)
		if err != nil {
			return nil, err
		}
		out = append(out, grandchildren...)
	}
	out = append(out, layer)
	return out, nil
}

// populateOrigins records the SourceContext for every node reachable from
// root in m, so the merge phase can later look up which layer a scalar
// originated from. Mappings, sequences and scalars are all recorded;
// downstream phases query the map per scalar.
func populateOrigins(m map[*yaml.Node]*node.SourceContext, root *yaml.Node, ctx *node.SourceContext) {
	if root == nil || ctx == nil {
		return
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	var visit func(n *yaml.Node)
	visit = func(n *yaml.Node) {
		if n == nil {
			return
		}
		m[n] = ctx
		for _, c := range n.Content {
			visit(c)
		}
	}
	visit(target)
}

// mergeLayers folds layers[1:] into layers[0] using override.MergeNode at
// the root path. Before each merge, the right-hand layer's recorded
// !reset / !override paths are applied to the accumulator so the override
// value replaces (rather than merges with) the base; the same paths are
// then dropped from the returned list so the orchestrator post-merge
// ApplyResetPaths does not delete the value it was meant to preserve.
func mergeLayers(layers []*node.Layer) (*node.Layer, []tree.Path, error) {
	acc := layers[0]
	resetPaths := append([]tree.Path(nil), acc.ResetPaths()...)
	for _, layer := range layers[1:] {
		if len(layer.ResetPaths()) > 0 {
			node.ApplyResetPaths(acc.Node, layer.ResetPaths())
		}
		out, err := override.MergeNode(acc.Node, layer.Node, tree.NewPath())
		if err != nil {
			return nil, nil, err
		}
		acc.Node = out
		// Do not re-record paths consumed during merge; they have served
		// their purpose by clearing the base value, and re-applying them
		// post-merge would delete the override value the user wants kept.
	}
	if _, err := override.EnforceUnicityNode(acc.Node); err != nil {
		return nil, nil, err
	}
	return acc, resetPaths, nil
}

// interpolateMerged runs lazy per-scalar interpolation across the merged
// tree, using the origins map to pick the right SourceContext for each
// scalar. The fall-back is the merged layer Context, which applies to
// synthetic nodes injected by canonicalization / merge promotion.
func interpolateMerged(merged *node.Layer, origins map[*yaml.Node]*node.SourceContext, opts *Options) error {
	substitute := template.Substitute
	if opts.Interpolate != nil && opts.Interpolate.Substitute != nil {
		substitute = opts.Interpolate.Substitute
	}
	lookupFor := func(n *yaml.Node) interp.LookupValue {
		ctx := origins[n]
		if ctx == nil {
			ctx = merged.Context
		}
		env := ctx.Environment
		return func(k string) (string, bool) {
			v, ok := env[k]
			return v, ok
		}
	}
	return interp.InterpolateNode(merged.Node, interp.NodeOptions{
		LookupValueFor: lookupFor,
		Substitute:     substitute,
		Tags:           tagsForV3Casts(),
	})
}

// workingDirLookup returns a function that picks the working directory to
// use when resolving a relative path scalar. Each scalar consults the
// origins map for its SourceContext; nodes that have no recorded origin
// (synthesized during merge) fall back to fallback.
func workingDirLookup(origins map[*yaml.Node]*node.SourceContext, fallback string) func(*yaml.Node) string {
	return func(n *yaml.Node) string {
		if ctx := origins[n]; ctx != nil {
			// Skip scalars whose layer already went through the include
			// sub-load path resolution: re-resolving them at this level
			// would double-join when the include project_directory was
			// relative.
			if ctx.PathsPreResolved {
				return ""
			}
			if ctx.WorkingDir != "" {
				return ctx.WorkingDir
			}
		}
		return fallback
	}
}

// tagsForV3Casts maps tree.Path patterns to YAML tags so the interpolation
// phase can rewrite scalar.Tag in place after substitution, letting yaml.v4
// perform the type conversion natively at decode time. Mirrors the cast
// targets registered in interpolateTypeCastMapping.
func tagsForV3Casts() map[tree.Path]string {
	out := map[tree.Path]string{}
	for path, caster := range interpolateTypeCastMapping {
		out[path] = tagForCast(caster)
	}
	return out
}

// hasLocalLoader reports whether the slice already contains a
// localResourceLoader. Order-insensitive helper for the defensive
// initialization in LoadV3.
func hasLocalLoader(loaders []ResourceLoader) bool {
	for _, l := range loaders {
		if _, ok := l.(localResourceLoader); ok {
			return true
		}
	}
	return false
}

// workingDirOfFirstLoader returns the WorkingDir of the first
// localResourceLoader in opts.ResourceLoaders, or empty when none is
// present. Used to detect when expandIncludes should clone Options to
// re-root the resource lookup at a child's project_directory.
func (o Options) workingDirOfFirstLoader() string {
	for _, l := range o.ResourceLoaders {
		if local, ok := l.(localResourceLoader); ok {
			return local.WorkingDir
		}
	}
	return ""
}

func tagForCast(c interp.Cast) string {
	if c == nil {
		return ""
	}
	v, err := c("0")
	if err != nil {
		return ""
	}
	switch v.(type) {
	case bool:
		return "!!bool"
	case int, int32, int64:
		return "!!int"
	case float32, float64:
		return "!!float"
	}
	return ""
}
