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
// ConfigDetails and returns the merged compose model decoded into a
// map[string]any.
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
//  10. normalize defaults via NormalizeNode;
//  11. decode the final yaml.Node tree into map[string]any so the existing
//     ModelToProject (mapstructure) can finish the projection.
//
// Step 11 disappears in Phase D when types gain UnmarshalYAML methods and
// the final decode can go directly to *types.Project. The map detour is the
// last remaining v2 bridge.
//
// LoadV3 does not yet replace LoadWithContext; that cutover lands in the
// next commit once differential testing confirms parity with the existing
// fixture suite.
func LoadV3(ctx context.Context, cd types.ConfigDetails, opts *Options) (map[string]any, error) {
	if opts == nil {
		opts = &Options{}
	}
	// Mirror the v2 ToOptions behavior: always append a localResourceLoader
	// rooted at the project working directory so include / extends paths
	// fall back to a working loader when the caller did not configure any.
	if !hasLocalLoader(opts.ResourceLoaders) {
		opts.ResourceLoaders = append(opts.ResourceLoaders, localResourceLoader{WorkingDir: cd.WorkingDir})
	}
	// Ensure Interpolate is non-nil: projectName extraction and the
	// interpolate-merged pass both dereference *opts.Interpolate. Callers
	// that go through ToOptions already have it set; the defensive init
	// covers tests that build Options literals directly.
	if opts.Interpolate == nil {
		opts.Interpolate = &interp.Options{
			Substitute:      template.Substitute,
			LookupValue:     cd.LookupEnv,
			TypeCastMapping: interpolateTypeCastMapping,
		}
	}
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

	if !opts.SkipExtends {
		tracker := &cycleTracker{}
		for _, layer := range allLayers {
			if err := ApplyExtendsToLayer(ctx, layer, opts, tracker); err != nil {
				return nil, err
			}
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
			resolveDefaultBuildContext(merged.Node, cd.WorkingDir)
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

	var dict map[string]any
	if err := merged.Node.Decode(&dict); err != nil {
		return nil, fmt.Errorf("loadV3: decode merged tree: %w", err)
	}
	if len(dict) == 0 {
		return nil, errors.New("empty compose file")
	}
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
// with the project working directory. Tightly scoped to avoid the
// double-resolution problem that a generic post-defaults sweep would
// introduce on relative paths already resolved by the earlier pass.
func resolveDefaultBuildContext(root *yaml.Node, projectWD string) {
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	services := mappingValueByKey(target, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 1; i < len(services.Content); i += 2 {
		svc := services.Content[i]
		build := mappingValueByKey(svc, "build")
		if build == nil || build.Kind != yaml.MappingNode {
			continue
		}
		ctx := mappingValueByKey(build, "context")
		if ctx == nil || ctx.Kind != yaml.ScalarNode {
			continue
		}
		if ctx.Value == "." || ctx.Value == "" {
			ctx.Value = projectWD
		}
	}
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
// the root path. The accumulated reset / override paths are returned so
// the orchestrator can apply them after merge.
func mergeLayers(layers []*node.Layer) (*node.Layer, []tree.Path, error) {
	acc := layers[0]
	var resetPaths []tree.Path
	resetPaths = append(resetPaths, acc.ResetPaths()...)
	for _, layer := range layers[1:] {
		out, err := override.MergeNode(acc.Node, layer.Node, tree.NewPath())
		if err != nil {
			return nil, nil, err
		}
		acc.Node = out
		resetPaths = append(resetPaths, layer.ResetPaths()...)
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
		if ctx := origins[n]; ctx != nil && ctx.WorkingDir != "" {
			return ctx.WorkingDir
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
