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
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// Layer is a single Compose file parsed as a yaml.Node tree together with
// the loading context it was parsed under.
type Layer struct {
	// Root is the parsed yaml tree (a DocumentNode wrapping a MappingNode).
	Root *yaml.Node
	// Context is the loading context for nodes in Root that do not have a
	// more specific context registered.
	Context *types.NodeContext
}

// ComposeModel holds raw yaml.Node layers together with a per-node loading
// context, and resolves them lazily into a *types.Project. Yaml nodes are
// kept in their raw (uninterpolated, unmerged) form as long as possible.
// Interpolation, path resolution and decoding all happen during Resolve()
// and consume the per-node NodeContext when they need to.
type ComposeModel struct {
	// layers is the ordered set of parsed Compose files. The order matters:
	// later layers override earlier ones during merge.
	layers []*Layer
	// contexts associates each yaml.Node parsed by the loader with the
	// NodeContext it was parsed under. Node pointers survive merging (leaf
	// scalars are never cloned), so a single map covers the whole model.
	contexts map[*yaml.Node]*types.NodeContext
	// configDetails carries the project-level settings (working dir, env)
	// supplied by the caller.
	configDetails types.ConfigDetails
	// opts holds the resolved loader Options.
	opts *Options
	// loadedFiles tracks absolute file paths processed so far, for cycle
	// detection on include.
	loadedFiles []string
}

// newComposeModel creates an empty model.
func newComposeModel(configDetails types.ConfigDetails, opts *Options) *ComposeModel {
	return &ComposeModel{
		contexts:      map[*yaml.Node]*types.NodeContext{},
		configDetails: configDetails,
		opts:          opts,
	}
}

// registerNodes associates every node in a yaml tree with the given context.
// Calling registerNodes on a tree that already has entries is a no-op for
// nodes that already carry a context (more specific contexts win).
func (m *ComposeModel) registerNodes(node *yaml.Node, ctx *types.NodeContext) {
	m.registerNodesVisited(node, ctx, false, map[*yaml.Node]bool{})
}

func (m *ComposeModel) registerNodesVisited(node *yaml.Node, ctx *types.NodeContext, force bool, visited map[*yaml.Node]bool) {
	if node == nil || visited[node] {
		return
	}
	visited[node] = true
	if _, set := m.contexts[node]; force || !set {
		m.contexts[node] = ctx
	}
	for _, child := range node.Content {
		m.registerNodesVisited(child, ctx, force, visited)
	}
	if node.Alias != nil {
		m.registerNodesVisited(node.Alias, ctx, force, visited)
	}
}

// contextFor returns the NodeContext attached to node, falling back to the
// enclosing layer's context when node was created during merge and never
// registered. Returns nil only if the model itself has no layers.
func (m *ComposeModel) contextFor(node *yaml.Node) *types.NodeContext {
	if ctx, ok := m.contexts[node]; ok {
		return ctx
	}
	if len(m.layers) > 0 {
		return m.layers[0].Context
	}
	return nil
}
