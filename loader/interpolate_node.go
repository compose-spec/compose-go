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
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v3/template"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// interpolateTree walks the yaml tree starting at node and substitutes
// variables in every scalar value using the NodeContext attached to that
// scalar. Nodes that are not registered in the contexts map inherit the
// context of their nearest registered ancestor through the inherited
// parameter.
//
// p is the tree.Path of node; it is used to look up type cast rules from
// interpolateTypeCastMapping.
func (m *ComposeModel) interpolateTree(node *yaml.Node, p tree.Path, inherited *types.NodeContext) error {
	if node == nil {
		return nil
	}
	ctx := inherited
	if c, ok := m.contexts[node]; ok {
		ctx = c
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := m.interpolateTree(child, p, ctx); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			// Prefer the value's own context (more specific) when present.
			pairCtx := ctx
			if c, ok := m.contexts[val]; ok {
				pairCtx = c
			} else if c, ok := m.contexts[key]; ok {
				pairCtx = c
			}
			if err := m.interpolateTree(val, p.Next(key.Value), pairCtx); err != nil {
				return err
			}
		}

	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := m.interpolateTree(child, p.Next(tree.PathMatchList), ctx); err != nil {
				return err
			}
		}

	case yaml.ScalarNode:
		return m.interpolateScalar(node, p, ctx)
	}
	return nil
}

// interpolateScalar substitutes variables in a single scalar node and applies
// type casting based on the tree path. The original node is mutated in place
// so that downstream passes see the resolved value.
func (m *ComposeModel) interpolateScalar(node *yaml.Node, p tree.Path, ctx *types.NodeContext) error {
	if !strings.Contains(node.Value, "$") {
		return nil
	}

	lookup := func(key string) (string, bool) {
		if ctx == nil {
			return "", false
		}
		v, ok := ctx.Env[key]
		return v, ok
	}

	substituted, err := template.Substitute(node.Value, template.Mapping(lookup))
	if err != nil {
		return wrapNodeErr(ctx, node, err)
	}

	for pattern, caster := range interpolateTypeCastMapping {
		if !p.Matches(pattern) {
			continue
		}
		casted, castErr := caster(substituted)
		if castErr != nil {
			return nodeErrf(ctx, node, "%s: failed to cast %q: %v", p, substituted, castErr)
		}
		switch casted.(type) {
		case bool:
			node.Tag = "!!bool"
		case int, int64:
			node.Tag = "!!int"
		case float32, float64:
			node.Tag = "!!float"
		case nil:
			node.Tag = "!!null"
			node.Value = "null"
			return nil
		}
		node.Value = fmt.Sprint(casted)
		return nil
	}

	node.Value = substituted
	return nil
}
