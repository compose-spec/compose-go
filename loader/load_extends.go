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

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/override"
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
	if service.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("services.%s must be a mapping", name)
	}
	extendsNode := mappingValueByKey(service, "extends")
	if extendsNode == nil {
		return service, nil
	}

	ref, file, err := parseExtendsRef(name, extendsNode)
	if err != nil {
		return nil, err
	}

	currentFile := layer.Context.File
	baseSiblings := siblingServices
	if file != "" {
		baseLayer, err := loadExtendsBaseLayer(ctx, layer, file, opts)
		if err != nil {
			return nil, err
		}
		baseSiblings = layerMappingField(baseLayer.Node, "services")
		if baseSiblings == nil {
			return nil, fmt.Errorf("cannot extend service %q in %s: no services section", name, file)
		}
		currentFile = baseLayer.Context.File
	}

	if mappingValueByKey(baseSiblings, ref) == nil {
		return nil, fmt.Errorf("cannot extend service %q in %s: service %q not found", name, layer.Context.File, ref)
	}

	tracker, err = tracker.Add(currentFile, name)
	if err != nil {
		return nil, err
	}

	// Recurse into the base to resolve its own extends chain first.
	base, err := applyServiceExtendsNode(ctx, layer, ref, baseSiblings, opts, tracker)
	if err != nil {
		return nil, err
	}
	if base == nil {
		return service, nil
	}

	// Merge base + service through the standard service-level rules. The
	// canonical merge path is "services.x" — same key used by the v2
	// override.ExtendService.
	merged, err := override.MergeNode(deepCloneNode(base), service, tree.NewPath("services", "x"))
	if err != nil {
		return nil, err
	}
	deleteMappingKey(merged, "extends")
	return merged, nil
}

// parseExtendsRef extracts the (service, file) tuple from an extends value.
// The short form (a bare scalar) names a sibling service. The long form is
// a mapping with required `service` and optional `file`.
func parseExtendsRef(name string, extendsNode *yaml.Node) (string, string, error) {
	switch extendsNode.Kind {
	case yaml.ScalarNode:
		return extendsNode.Value, "", nil
	case yaml.MappingNode:
		var ref, file string
		if r := mappingValueByKey(extendsNode, "service"); r != nil && r.Kind == yaml.ScalarNode {
			ref = r.Value
		}
		if f := mappingValueByKey(extendsNode, "file"); f != nil && f.Kind == yaml.ScalarNode {
			file = f.Value
		}
		if ref == "" {
			return "", "", fmt.Errorf("services.%s.extends.service is required", name)
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
func loadExtendsBaseLayer(ctx context.Context, parent *node.Layer, file string, opts *Options) (*node.Layer, error) {
	fullPath, err := resolveResourcePath(ctx, opts, file)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(parent.Context.WorkingDir, fullPath)
	}
	sc := &node.SourceContext{
		File:        fullPath,
		WorkingDir:  filepath.Dir(fullPath),
		Environment: parent.Context.Environment,
		Parent:      parent.Context,
	}
	layers, err := LoadLayer(ctx, types.ConfigFile{Filename: fullPath}, sc, opts)
	if err != nil {
		return nil, err
	}
	if len(layers) == 0 {
		return nil, fmt.Errorf("extends.file %s yields no document", fullPath)
	}
	return layers[0], nil
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
