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
	"fmt"
	"path/filepath"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/errdefs"
	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/paths"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
)

// ApplyExtendsToLayer resolves every `extends` directive in layer's services
// block by walking the inheritance chain, merging base + derived service
// nodes through override.MergeNode at path "services.x", and stripping the
// `extends` field from the result.
//
// extends.file referencing another compose file produces an on-the-fly
// Layer for that file (parse + reset/override resolution + alias
// normalization) so the base service is available for merging. Those
// sub-layers are discarded after the merge; only the resulting merged
// service node is grafted back into the original layer's tree.
//
// Cycles are detected via the standard cycleTracker keyed by (file, name);
// the same value used by the v2 ApplyExtends so existing fixtures keep
// triggering the same diagnostics.
//
// ApplyExtendsToLayer mutates layer in place. It does not perform cross-
// file merge, interpolation, or path resolution; those run in subsequent
// phases of the orchestrator.
func ApplyExtendsToLayer(ctx context.Context, layer *node.Layer, opts *Options, tracker *cycleTracker) error {
	services := layerMappingField(layer.Node, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		merged, err := applyServiceExtendsNode(ctx, layer, name, services, opts, tracker)
		if err != nil {
			return err
		}
		if merged != nil {
			services.Content[i+1] = merged
		}
	}
	return nil
}

// applyServiceExtendsNode resolves the extends chain for a single service
// in siblingServices. It returns the merged service node or the original
// node when no extends directive is present.
//
// The base service is located either in siblingServices (same layer) or in
// a freshly loaded Layer for the file referenced by extends.file. extends
// is applied recursively to the base so a chain of N levels resolves
// before the final merge fires.
func applyServiceExtendsNode(
	ctx context.Context,
	layer *node.Layer,
	name string,
	siblingServices *yaml.Node,
	opts *Options,
	tracker *cycleTracker,
) (*yaml.Node, error) {
	service := mappingValueByKey(siblingServices, name)
	if service == nil {
		return nil, nil
	}
	// A YAML null value (`name:` with no body) is treated as an empty
	// service — same as v2, where the empty mapping contributes no fields
	// to a downstream extends merge but is otherwise valid.
	if service.Kind == yaml.ScalarNode && service.Tag == "!!null" {
		return service, nil
	}
	if service.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("services.%s must be a mapping", name)
	}
	extendsNode := mappingValueByKey(service, "extends")
	if extendsNode == nil {
		return service, nil
	}

	ref, file, err := parseExtendsRef(name, extendsNode, opts)
	if err != nil {
		return nil, err
	}

	currentFile := layer.Context.File
	baseSiblings := siblingServices
	childOpts := opts
	originalLayer := layer
	if file != "" {
		baseLayer, childOptsLoaded, err := loadExtendsBaseLayer(ctx, layer, file, opts)
		if err != nil {
			return nil, err
		}
		baseSiblings = layerMappingField(baseLayer.Node, "services")
		if baseSiblings == nil {
			return nil, fmt.Errorf("cannot extend service %q in %s: no services section", name, file)
		}
		currentFile = baseLayer.Context.File
		// Reuse layer so the recursion sees the base layer's tree, but
		// keep the child-scoped opts so further extends.file references
		// resolve against the extended file's directory rather than the
		// project root.
		layer = baseLayer
		childOpts = childOptsLoaded
	}

	if mappingValueByKey(baseSiblings, ref) == nil {
		diag := &errdefs.Diagnostic{
			Path:  fmt.Sprintf("services.%s.extends", name),
			Cause: errString(fmt.Sprintf("cannot extend service %q in %s: service %q not found", name, layer.Context.File, ref)),
		}
		if extendsNode != nil {
			diag.File = originalLayer.Context.File
			diag.Line = extendsNode.Line
			diag.Column = extendsNode.Column
		}
		return nil, diag
	}

	tracker, err = tracker.Add(currentFile, name)
	if err != nil {
		return nil, err
	}

	// Recurse into the base to resolve its own extends chain first.
	base, err := applyServiceExtendsNode(ctx, layer, ref, baseSiblings, childOpts, tracker)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return service, nil
	}
	// Mutate the sibling services mapping so the resolved base replaces
	// its original entry. Subsequent top-level iterations over the same
	// services mapping see the already-resolved base and skip re-entering
	// the extends chain — mirrors the v2 `services[name] = merged` side
	// effect that keeps Listener event counts deterministic.
	setMappingValue(baseSiblings, ref, base)

	// Apply the parent layer's recorded !reset / !override paths to the
	// cloned base BEFORE merging it with the derived service. Mirrors v2
	// applyServiceExtends, which calls processor.Apply on the wrapped base
	// to drop any path that the derived service marked with !reset or
	// !override — so the override entry from the derived service wins
	// outright once mergeSpecials kicks in. The consumed paths are then
	// removed from the layer's resetPaths so the orchestrator post-merge
	// ApplyResetPaths does not delete them again from the final tree.
	clonedBase := deepCloneNode(base)
	consumed := resetParentPaths(clonedBase, name, originalLayer.ResetPaths())
	if len(consumed) > 0 {
		originalLayer.SetResetPaths(diffPaths(originalLayer.ResetPaths(), consumed))
	}

	// Merge base + service through the standard service-level rules. The
	// canonical merge path is "services.x" — same key used by the v2
	// override.ExtendService.
	merged, err := override.MergeNode(clonedBase, service, tree.NewPath("services", "x"))
	if err != nil {
		return nil, err
	}
	deleteMappingKey(merged, "extends")
	// When extends went through an extends.file (loaded a sub-layer),
	// rewrite relative paths in the merged service against the sub-file's
	// working directory. Matches v2 getExtendsBaseFromFile semantics where
	// paths accumulate the file's relative dir as the chain unwinds.
	if file != "" {
		// resolveExtendedServicePaths uses the relative form preferred
		// by v2 so paths stamped on the merged service look "as if" the
		// caller had declared them at the parent layer's working dir.
		// Fall back to the absolute WorkingDir when the relative form
		// is empty (remote loaders that did not stash a relative form).
		extendsWD := childOpts.extendsRelativeDir
		if extendsWD == "" {
			extendsWD = layer.Context.WorkingDir
		}
		if err := resolveExtendedServicePaths(merged, extendsWD, childOpts); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

// parseExtendsRef extracts the (service, file) tuple from an extends value
// and fires the "extends" Listener event with a v2-compatible payload so
// downstream consumers (telemetry, dependency analysis) keep observing the
// same callback signature as before the refactor. The short form (a bare
// scalar) names a sibling service; the long form is a mapping with
// required `service` and optional `file`.
func parseExtendsRef(name string, extendsNode *yaml.Node, opts *Options) (string, string, error) {
	switch extendsNode.Kind {
	case yaml.ScalarNode:
		opts.ProcessEvent("extends", map[string]any{"service": extendsNode.Value})
		return extendsNode.Value, "", nil
	case yaml.MappingNode:
		var ref, file string
		payload := map[string]any{}
		if r := mappingValueByKey(extendsNode, "service"); r != nil && r.Kind == yaml.ScalarNode {
			ref = r.Value
			payload["service"] = r.Value
		}
		if f := mappingValueByKey(extendsNode, "file"); f != nil && f.Kind == yaml.ScalarNode {
			file = f.Value
			payload["file"] = f.Value
		}
		opts.ProcessEvent("extends", payload)
		if ref == "" {
			return "", "", fmt.Errorf("extends.%s.service is required", name)
		}
		return ref, file, nil
	}
	return "", "", fmt.Errorf("services.%s.extends must be a string or a mapping", name)
}

// loadExtendsBaseLayer loads the file referenced by extends.file into a
// stand-alone Layer that carries the file's own SourceContext (working dir,
// environment). The returned layer is meant for a single read of one
// service definition and is discarded after the merge.
//
// Relative paths are resolved through the configured ResourceLoaders, so
// remote loaders (oci://, https://, ...) registered on opts also work for
// extends.file references.
//
// The function also returns child-scoped Options whose ResourceLoaders are
// re-rooted at the extended file's directory. Recursive extends inside the
// loaded layer (extends.file pointing at a sibling file) are then resolved
// against the file's own directory rather than the project root, matching
// v2 getExtendsBaseFromFile behavior.
func loadExtendsBaseLayer(ctx context.Context, parent *node.Layer, file string, opts *Options) (*node.Layer, *Options, error) {
	// Resolve extends.file against the *parent layer* working directory so
	// extends declared inside an included file pick up the include's own
	// project_directory rather than the outer project root. This makes
	// nested `include -> extends -> extends.file` work the same way v2
	// does, where each recursive load uses ResourceLoaders pinned to the
	// current file's directory.
	parentOpts := opts
	if parent.Context.WorkingDir != "" {
		parentOpts = opts.clone()
		parentOpts.ResourceLoaders = append(opts.RemoteResourceLoaders(), localResourceLoader{WorkingDir: parent.Context.WorkingDir})
	}
	loader, fullPath, err := resolveResourceWithLoader(ctx, parentOpts, file)
	if err != nil {
		return nil, nil, err
	}
	// absLocalDir is the directory of the extended file (always absolute).
	// localDir is the relative form returned by loader.Dir, used as the
	// base for in-tree path resolution so the resulting paths match the
	// v2 relative form (paths look like "testdata/subdir/extra.env"
	// rather than absolute paths until the outer pass absolutizes them).
	// We store absLocalDir on the SourceContext so chained extends /
	// include extends always find files relative to a real directory,
	// and keep localDir as a side-table on Options for the per-merge
	// path resolution call below.
	localDir := loader.Dir(file)
	absLocalDir := filepath.Dir(fullPath)
	sc := &node.SourceContext{
		File:        fullPath,
		WorkingDir:  absLocalDir,
		Environment: parent.Context.Environment,
		Parent:      parent.Context,
	}
	childOpts := opts.clone()
	childOpts.ResourceLoaders = append(opts.RemoteResourceLoaders(), localResourceLoader{WorkingDir: absLocalDir})
	childOpts.extendsRelativeDir = localDir
	layers, err := LoadLayer(ctx, types.ConfigFile{Filename: fullPath}, sc, childOpts)
	if err != nil {
		return nil, nil, err
	}
	if len(layers) == 0 {
		return nil, nil, fmt.Errorf("extends.file %s yields no document", fullPath)
	}
	return layers[0], childOpts, nil
}

// resolveExtendedServicePaths runs path resolution on the merged service
// node using workingDir as the base, mimicking the v2 paths.ResolveRelative
// Paths call inside getExtendsBaseFromFile. Each extends.file level rewrites
// the paths against its own relative dir, so nested extends accumulate the
// expected relative form (sibling.yaml's `.` becomes `testdata/extends`
// when extended from base.yaml which lives there).
func resolveExtendedServicePaths(merged *yaml.Node, workingDir string, opts *Options) error {
	if workingDir == "" {
		return nil
	}
	var remotes []paths.RemoteResource
	for _, loader := range opts.RemoteResourceLoaders() {
		remotes = append(remotes, loader.Accept)
	}
	// Wrap the merged service node in a synthetic "services.x" mapping so
	// the path patterns (which all start at the root) match against it.
	wrapper := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "services"},
			{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
				Content: []*yaml.Node{
					{Kind: yaml.ScalarNode, Tag: "!!str", Value: "x"},
					merged,
				},
			},
		},
	}
	return paths.ResolveRelativePathsNode(wrapper, paths.NodeResolverOptions{
		WorkingDir: workingDir,
		Remotes:    remotes,
	})
}

// resetParentPaths removes mapping keys in serviceNode that match a recorded
// !reset / !override path under services.<serviceName>. Mirrors the
// applyNullOverrides traversal v2 does on processor.Apply, but scoped to a
// single service's body so it can run on the cloned base before extends
// merge fires. Returns the list of paths that were consumed so the caller
// can clear them from the layer's master list to avoid double-application
// during the orchestrator post-merge sweep.
func resetParentPaths(serviceNode *yaml.Node, serviceName string, resetPaths []tree.Path) []tree.Path {
	if serviceNode == nil || serviceNode.Kind != yaml.MappingNode || len(resetPaths) == 0 {
		return nil
	}
	prefix := tree.NewPath("services", serviceName)
	var consumed []tree.Path
	for _, p := range resetPaths {
		rel := relativePath(p, prefix)
		if rel == "" {
			continue
		}
		deleteAtPath(serviceNode, rel)
		consumed = append(consumed, p)
	}
	return consumed
}

// diffPaths returns the elements of all not present in remove, preserving
// the original order. Used to drop !override paths that have already been
// honored by extends so they don't get re-applied by ApplyResetPaths on the
// merged tree.
func diffPaths(all []tree.Path, remove []tree.Path) []tree.Path {
	if len(all) == 0 || len(remove) == 0 {
		return all
	}
	removed := make(map[tree.Path]bool, len(remove))
	for _, r := range remove {
		removed[r] = true
	}
	out := all[:0]
	for _, p := range all {
		if removed[p] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// relativePath returns the portion of p that follows prefix, or "" when p
// is not rooted at prefix. Comparison treats prefix parts as literal (no
// wildcard expansion).
func relativePath(p, prefix tree.Path) tree.Path {
	pParts := p.Parts()
	prefixParts := prefix.Parts()
	if len(pParts) <= len(prefixParts) {
		return ""
	}
	for i, part := range prefixParts {
		if pParts[i] != part {
			return ""
		}
	}
	return tree.NewPath(pParts[len(prefixParts):]...)
}

// deleteAtPath removes the entry at a relative path inside n (a Mapping
// Node). Only the first segment is followed at each step; intermediate
// segments must reference Mapping keys, otherwise the function is a no-op.
func deleteAtPath(n *yaml.Node, p tree.Path) {
	parts := p.Parts()
	if len(parts) == 0 || n == nil {
		return
	}
	if len(parts) == 1 {
		deleteMappingKey(n, parts[0])
		return
	}
	child := mappingValueByKey(n, parts[0])
	deleteAtPath(child, tree.NewPath(parts[1:]...))
}

// setMappingValue replaces (or appends) the entry whose key matches in a
// MappingNode. Used by applyServiceExtendsNode to commit the resolved base
// service back into the siblings mapping so further iterations observe the
// updated tree.
func setMappingValue(n *yaml.Node, key string, value *yaml.Node) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content[i+1] = value
			return
		}
	}
	n.Content = append(n.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}

// mappingValueByKey returns the value Node for a key inside a MappingNode,
// or nil when absent. Shared by the include and extends paths because both
// need to look up service entries inside the services mapping.
func mappingValueByKey(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// deleteMappingKey removes the (key, value) pair whose key matches the
// given string from a MappingNode. No-op when the node is not a mapping or
// the key is absent.
func deleteMappingKey(n *yaml.Node, key string) {
	if n == nil || n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			n.Content = append(n.Content[:i], n.Content[i+2:]...)
			return
		}
	}
}

// deepCloneNode returns a structural copy of n with nested Content cloned.
// Used to avoid mutating the base service node while merging it into a
// derived service (the same base may be reused by other extends chains in
// the same load).
func deepCloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        n.Kind,
		Tag:         n.Tag,
		Value:       n.Value,
		Style:       n.Style,
		Anchor:      n.Anchor,
		Alias:       n.Alias,
		Line:        n.Line,
		Column:      n.Column,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
	}
	if len(n.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			clone.Content[i] = deepCloneNode(c)
		}
	}
	return clone
}
