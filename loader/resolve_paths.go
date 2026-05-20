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
	"path"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v3/format"
	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// resolvablePathPatterns is the central list of tree paths whose value is a
// filesystem path that the loader rewrites against the node's
// NodeContext.WorkingDir during the path resolution pass.
//
// services.*.env_file.* is intentionally absent: env files are resolved on
// demand by WithServicesEnvironmentResolved (Phase 7), so they can use the
// per-EnvFile loading context (including variables provided by an enclosing
// include.env_file) for both their path and their content interpolation.
var resolvablePathPatterns = []tree.Path{
	"services.*.build.context",
	"services.*.build.additional_contexts.*",
	"services.*.label_file",
	"services.*.label_file.*.path",
	"services.*.extends.file",
	"services.*.develop.watch.*.path",
	"configs.*.file",
	"secrets.*.file",
}

// resolvePathsPass walks the given tree and rewrites every relative path
// found at a resolvable location into an absolute one. Volume sources
// (short and long syntax) are handled separately because they are not plain
// scalar paths.
func (m *ComposeModel) resolvePathsPass(root *yaml.Node) {
	m.resolvePathsWalk(root, tree.NewPath(), m.contextFor(root))
}

func (m *ComposeModel) resolvePathsWalk(node *yaml.Node, p tree.Path, inherited *types.NodeContext) {
	if node == nil {
		return
	}
	ctx := inherited
	if c, ok := m.contexts[node]; ok {
		ctx = c
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			m.resolvePathsWalk(child, p, ctx)
		}
	case yaml.MappingNode:
		if p.Matches("services.*.volumes.*") {
			resolveLongVolume(node, ctx)
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			childCtx := ctx
			if c, ok := m.contexts[val]; ok {
				childCtx = c
			} else if c, ok := m.contexts[key]; ok {
				childCtx = c
			}
			m.resolvePathsWalk(val, p.Next(key.Value), childCtx)
		}
	case yaml.SequenceNode:
		if p.Matches("services.*.volumes") {
			for _, item := range node.Content {
				if item.Kind != yaml.ScalarNode {
					continue
				}
				itemCtx := ctx
				if c, ok := m.contexts[item]; ok {
					itemCtx = c
				}
				resolveShortVolume(item, itemCtx)
			}
		}
		for _, child := range node.Content {
			m.resolvePathsWalk(child, p.Next(tree.PathMatchList), ctx)
		}
	case yaml.ScalarNode:
		for _, pattern := range resolvablePathPatterns {
			if !p.Matches(pattern) {
				continue
			}
			rewriteScalarPath(node, ctx)
			return
		}
	}
}

// rewriteScalarPath turns a relative path scalar into an absolute one using
// ctx.WorkingDir. Empty values, remote URLs (containing "://") and values
// already absolute on Posix or Windows-style filesystems are left alone.
func rewriteScalarPath(node *yaml.Node, ctx *types.NodeContext) {
	if ctx == nil || node.Value == "" {
		return
	}
	if strings.Contains(node.Value, "://") {
		return
	}
	if filepath.IsAbs(node.Value) || path.IsAbs(node.Value) {
		return
	}
	node.Value = filepath.Join(ctx.WorkingDir, node.Value)
}

// resolveShortVolume rewrites the source part of a short-syntax volume
// scalar ("./local:/app:ro") if it is a relative bind path. Anonymous and
// named volumes are left alone.
func resolveShortVolume(node *yaml.Node, ctx *types.NodeContext) {
	if ctx == nil || node.Value == "" {
		return
	}
	vol, err := format.ParseVolume(node.Value)
	if err != nil {
		return
	}
	if vol.Type != types.VolumeTypeBind || vol.Source == "" {
		return
	}
	if filepath.IsAbs(vol.Source) || path.IsAbs(vol.Source) {
		return
	}
	abs := filepath.Join(ctx.WorkingDir, vol.Source)
	node.Value = strings.Replace(node.Value, vol.Source, abs, 1)
}

// resolveLongVolume rewrites the source key of a long-syntax bind volume
// mapping when it holds a relative path. Other volume types are left alone.
func resolveLongVolume(node *yaml.Node, ctx *types.NodeContext) {
	if ctx == nil || node.Kind != yaml.MappingNode {
		return
	}
	_, kind := override.FindKey(node, "type")
	if kind == nil || kind.Value != types.VolumeTypeBind {
		return
	}
	_, src := override.FindKey(node, "source")
	if src == nil || src.Kind != yaml.ScalarNode || src.Value == "" {
		return
	}
	if filepath.IsAbs(src.Value) || path.IsAbs(src.Value) {
		return
	}
	src.Value = filepath.Join(ctx.WorkingDir, src.Value)
}
