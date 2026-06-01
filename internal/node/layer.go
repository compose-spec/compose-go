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

// Package node holds the yaml.Node-centric building blocks used by the v3
// loader pipeline. A Layer pairs a parsed YAML tree with the SourceContext
// that produced it, so per-node parsing context (working directory, env
// variables, source file/line) can be preserved across cross-file merges
// and applied lazily during interpolation and path resolution.
package node

import (
	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
)

// SourceInline is used as SourceContext.File when a Layer is built from
// in-memory bytes with no associated filename.
const SourceInline = "(inline)"

// SourceContext carries the parsing context attached to a YAML subtree.
// It is the unit of information needed to interpolate a scalar lazily and to
// resolve a relative path against the appropriate working directory after
// cross-file merge.
type SourceContext struct {
	// File is the absolute path of the source file, or SourceInline when the
	// layer was constructed from in-memory content.
	File string

	// WorkingDir is the directory against which relative paths inside this
	// subtree are resolved. For an included file it is the include's
	// project_directory, not the project root.
	WorkingDir string

	// Environment is the variable lookup table effective for this subtree.
	// It is the result of merging the shell environment with any env_file
	// declared by the layer's loader (top-level, include, or extends).
	Environment types.Mapping

	// EnvFiles lists the env_file paths, in load order, that contributed to
	// Environment. Kept for diagnostics; not consulted at lookup time.
	EnvFiles []string

	// Parent points to the SourceContext that triggered loading this one
	// (via include or extends). Nil for the root context. The chain enables
	// "in file X included from file Y" style diagnostics.
	Parent *SourceContext
}

// Layer is a parsed YAML document paired with its SourceContext.
//
// Node is the document root as returned by yaml.Decoder (the DocumentNode
// wrapper is typically stripped before storing the inner MappingNode). The
// node retains all position information and the original Kind/Tag/Style of
// every scalar, which v3 uses both for diagnostics and to drive type
// conversion at decode time.
//
// origins is a sparse side-table mapping individual *yaml.Node values to a
// SourceContext different from the layer default. Until a cross-file merge
// rewires nodes from other layers into this tree, the map is empty and
// Origin returns the layer Context for any node.
type Layer struct {
	Node    *yaml.Node
	Context *SourceContext

	origins    map[*yaml.Node]*SourceContext
	resetPaths []tree.Path
}

// NewLayer returns a Layer that pairs node with ctx. The origins side-table
// is allocated on first SetOrigin; until then Origin returns ctx for any
// queried node.
func NewLayer(node *yaml.Node, ctx *SourceContext) *Layer {
	return &Layer{Node: node, Context: ctx}
}

// Origin returns the SourceContext governing the interpretation of n. When no
// explicit origin has been recorded for n, the layer default Context is
// returned.
func (l *Layer) Origin(n *yaml.Node) *SourceContext {
	if l == nil {
		return nil
	}
	if ctx, ok := l.origins[n]; ok {
		return ctx
	}
	return l.Context
}

// SetOrigin records an explicit origin for n. Used by the merge phase when a
// node from another layer is grafted into this layer's tree.
func (l *Layer) SetOrigin(n *yaml.Node, ctx *SourceContext) {
	if l.origins == nil {
		l.origins = make(map[*yaml.Node]*SourceContext)
	}
	l.origins[n] = ctx
}

// SetResetPaths records the tree.Paths where !reset / !override tags were
// found during ResolveResetOverride. The merge phase consults this list to
// drop or replace values from base layers at those paths.
func (l *Layer) SetResetPaths(paths []tree.Path) {
	l.resetPaths = paths
}

// ResetPaths returns the list of paths recorded by SetResetPaths, in the
// order they were collected.
func (l *Layer) ResetPaths() []tree.Path {
	return l.resetPaths
}
