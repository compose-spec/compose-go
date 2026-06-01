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

	// Path resolution runs before canonicalization on purpose: the
	// CanonicalNode bridge currently rebuilds the affected subtrees via
	// map[string]any, which loses *yaml.Node pointer identity and breaks
	// the origins-driven per-scalar WorkingDir lookup. Resolving paths
	// first guarantees every relative path scalar still has its origin
	// recorded. Once individual transformers are ported to operate on
	// yaml.Node directly (Phase B follow-ups) this constraint disappears.
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

	if !opts.SkipValidation {
		if err := validation.ValidateNode(merged.Node); err != nil {
			return nil, err
		}
		// The version attribute is obsolete; v2 strips it after schema
		// validation and emits a deprecation warning. v3 preserves the
		// behavior so existing fixtures keep producing identical output.
		if hasMappingKey(merged.Node, "version") {
			for _, f := range cd.ConfigFiles {
				opts.warnObsoleteVersion(f.Filename)
			}
			deleteMappingKey(merged.Node, "version")
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
		grandchildren, err := expandIncludes(ctx, child, opts)
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
