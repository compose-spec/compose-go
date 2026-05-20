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
	"slices"

	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// applyExtendsNode resolves extends: directives within the services mapping
// of a layer. The merging is performed on yaml.Node trees; relative paths in
// the inherited service are left untouched. They will be resolved against
// the included file's NodeContext.WorkingDir by the path resolution pass
// (Phase 6).
func (m *ComposeModel) applyExtendsNode(ctx context.Context, layer *Layer) error {
	root := unwrapDocument(layer.Root)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	_, services := override.FindKey(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}

	resolved := map[string]bool{}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		if err := m.resolveServiceExtends(ctx, layer, services, name, resolved, nil); err != nil {
			return err
		}
	}
	return nil
}

// resolveServiceExtends resolves the extends: directive of a single service.
// chain is the running list of "file:service" identifiers used for cycle
// detection. resolved memoizes services already processed within services
// so we do not re-merge a service that was extended several times.
func (m *ComposeModel) resolveServiceExtends(ctx context.Context, layer *Layer, services *yaml.Node, name string, resolved map[string]bool, chain []string) error {
	if resolved[name] {
		return nil
	}
	chainID := layer.Context.Source + ":" + name
	if slices.Contains(chain, chainID) {
		return fmt.Errorf("circular reference with extends: %s", chainID)
	}
	chain = append(chain, chainID)

	_, svcNode := override.FindKey(services, name)
	if svcNode == nil {
		return nil
	}
	_, extendsNode := override.FindKey(svcNode, "extends")
	if extendsNode == nil {
		resolved[name] = true
		return nil
	}

	refService, refFile, err := readExtendsRef(extendsNode, name)
	if err != nil {
		return err
	}
	m.opts.ProcessEvent("extends", extendsEventMetadata(refService, refFile))

	var baseService *yaml.Node

	if refFile != "" {
		baseService, err = m.loadExtendsBaseFromFile(ctx, layer, refFile, refService, chain)
		if err != nil {
			return err
		}
	} else {
		if err := m.resolveServiceExtends(ctx, layer, services, refService, resolved, chain); err != nil {
			return err
		}
		_, baseService = override.FindKey(services, refService)
		if baseService == nil {
			return fmt.Errorf("service %q not found in extends", refService)
		}
	}

	baseClone := m.deepCloneNode(baseService)
	merged, err := override.ExtendServiceNode(baseClone, svcNode)
	if err != nil {
		return fmt.Errorf("extending service %s: %w", name, err)
	}
	override.DeleteKey(merged, "extends")
	override.SetKey(services, name, merged)
	resolved[name] = true
	return nil
}

// readExtendsRef parses the extends node into (refService, refFile).
func readExtendsRef(extendsNode *yaml.Node, name string) (string, string, error) {
	switch extendsNode.Kind {
	case yaml.ScalarNode:
		return extendsNode.Value, "", nil
	case yaml.MappingNode:
		_, sn := override.FindKey(extendsNode, "service")
		if sn == nil {
			return "", "", fmt.Errorf("extends.%s.service is required", name)
		}
		refService := sn.Value
		refFile := ""
		if _, fn := override.FindKey(extendsNode, "file"); fn != nil {
			refFile = fn.Value
		}
		return refService, refFile, nil
	default:
		return "", "", fmt.Errorf("extends must be a string or mapping")
	}
}

func extendsEventMetadata(service, file string) map[string]any {
	md := map[string]any{"service": service}
	if file != "" {
		md["file"] = file
	}
	return md
}

// loadExtendsBaseFromFile loads refFile, registers its nodes with their own
// NodeContext (WorkingDir = dir(refFile)), recursively resolves the base
// service's own extends, and returns the resolved base service node.
func (m *ComposeModel) loadExtendsBaseFromFile(ctx context.Context, layer *Layer, refFile, refService string, chain []string) (*yaml.Node, error) {
	filePath := refFile
	for _, loader := range m.opts.RemoteResourceLoaders() {
		if loader.Accept(refFile) {
			resolved, err := loader.Load(ctx, refFile)
			if err != nil {
				return nil, err
			}
			filePath = resolved
			break
		}
	}
	if filePath == refFile && !filepath.IsAbs(filePath) {
		filePath = filepath.Join(layer.Context.WorkingDir, filePath)
	}

	extNode, err := loadYamlFileNode(types.ConfigFile{Filename: filePath})
	if err != nil {
		return nil, fmt.Errorf("loading extends file %s: %w", refFile, err)
	}
	if extNode == nil {
		return nil, fmt.Errorf("extends file %s is empty", refFile)
	}

	extCtx := &types.NodeContext{
		Source:     filePath,
		WorkingDir: filepath.Dir(filePath),
		Env:        layer.Context.Env,
		Parent:     layer.Context,
	}
	m.registerNodes(extNode, extCtx)

	extRoot := unwrapDocument(extNode)
	_, extServices := override.FindKey(extRoot, "services")
	if extServices == nil {
		return nil, fmt.Errorf("extends file %s has no services", refFile)
	}
	_, baseService := override.FindKey(extServices, refService)
	if baseService == nil {
		return nil, fmt.Errorf("service %q not found in %s", refService, refFile)
	}

	extLayer := &Layer{Root: extNode, Context: extCtx}
	if err := m.resolveServiceExtends(ctx, extLayer, extServices, refService, map[string]bool{}, chain); err != nil {
		return nil, err
	}
	_, baseService = override.FindKey(extServices, refService)
	return baseService, nil
}

// deepCloneNode creates a deep copy of a yaml.Node tree, propagating
// per-node contexts from each original to its clone. This is needed when
// merging a base service into an extender so we do not mutate the base.
func (m *ComposeModel) deepCloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
		Line:        node.Line,
		Column:      node.Column,
	}
	if ctx, ok := m.contexts[node]; ok {
		m.contexts[clone] = ctx
	}
	if node.Alias != nil {
		clone.Alias = m.deepCloneNode(node.Alias)
	}
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, c := range node.Content {
			clone.Content[i] = m.deepCloneNode(c)
		}
	}
	return clone
}

// unwrapDocument unwraps a DocumentNode and returns its mapping. It is a
// no-op for nodes that are already a mapping or any other kind.
func unwrapDocument(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}
