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
	"services.*.build",         // short syntax: `build: ./local`
	"services.*.build.context", // long syntax
	"services.*.build.additional_contexts.*",
	"services.*.build.ssh.*",
	"services.*.label_file",   // scalar shorthand: `label_file: ./foo.label`
	"services.*.label_file.*", // sequence of scalars
	"services.*.label_file.*.path",
	"services.*.env_file",   // scalar shorthand: `env_file: ./foo.env`
	"services.*.env_file.*", // sequence of scalars
	"services.*.env_file.*.path",
	"services.*.extends.file",
	"services.*.develop.watch.*.path",
	"configs.*.file",
	"secrets.*.file",
}

// resolvePathsPass walks the given tree and rewrites every relative path
// found at a resolvable location. Volume sources (short and long syntax)
// are handled separately because they are not plain scalar paths.
//
// Behaviour matches the legacy loader:
//   - when the loader option ResolvePaths is true (the default), every
//     resolvable path is rewritten to an absolute path against the per-node
//     WorkingDir;
//   - when ResolvePaths is false, paths that come from an included or
//     extends file (their NodeContext has a Parent) are still rewritten,
//     but expressed relative to the main working directory so the result
//     matches what the legacy loader produced.
func (m *ComposeModel) resolvePathsPass(root *yaml.Node) {
	rewrite := m.pathRewriter()
	m.resolvePathsWalk(root, tree.NewPath(), m.contextFor(root), rewrite)
}

// pathRewriter returns the function used to rewrite a scalar path value
// given the NodeContext attached to its node.
//
// The output is always expressed relative to the main project working
// directory: that way, when the post-merge legacy pass calls
// paths.ResolveRelativePaths with the same main working directory, the
// rewrite is either a no-op (path already absolute) or a Join+Clean of a
// relative path against the project root, exactly as the legacy loader
// produced it.
//
// When ResolvePaths is false we don't touch paths that originate in the
// main layer (NodeContext.Parent == nil) so callers asking for "no path
// resolution" keep their main paths verbatim, matching legacy behaviour.
func (m *ComposeModel) pathRewriter() func(value string, ctx *types.NodeContext) string {
	mainWD := m.configDetails.WorkingDir
	return func(value string, ctx *types.NodeContext) string {
		if ctx == nil || value == "" {
			return value
		}
		if strings.Contains(value, "://") {
			return value
		}
		if filepath.IsAbs(value) || path.IsAbs(value) {
			return value
		}
		// Build contexts and additional_contexts accept references to
		// remote sources (git URLs, github.com/, docker-image://, ...).
		// Leave those unchanged.
		if isRemoteContextRef(value) {
			return value
		}
		if !m.opts.ResolvePaths && ctx.Parent == nil {
			return value
		}
		abs := filepath.Join(ctx.WorkingDir, value)
		rel, err := filepath.Rel(mainWD, abs)
		if err != nil {
			return abs
		}
		return rel
	}
}

// isRemoteContextRef mirrors paths.isRemoteContext: any of the buildkit-
// recognised remote prefixes should be passed through untouched.
func isRemoteContextRef(v string) bool {
	for _, prefix := range []string{"https://", "http://", "git://", "ssh://", "github.com/", "git@"} {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

func (m *ComposeModel) resolvePathsWalk(node *yaml.Node, p tree.Path, inherited *types.NodeContext, rewrite func(string, *types.NodeContext) string) {
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
			m.resolvePathsWalk(child, p, ctx, rewrite)
		}
	case yaml.MappingNode:
		if p.Matches("services.*.volumes.*") {
			resolveLongVolume(node, ctx, rewrite)
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
			m.resolvePathsWalk(val, p.Next(key.Value), childCtx, rewrite)
		}
	case yaml.SequenceNode:
		if p.Matches("services.*.volumes") {
			for i, item := range node.Content {
				if item.Kind != yaml.ScalarNode {
					continue
				}
				itemCtx := ctx
				if c, ok := m.contexts[item]; ok {
					itemCtx = c
				}
				if replacement := convertShortVolume(item, itemCtx, rewrite); replacement != nil {
					node.Content[i] = replacement
				}
			}
		}
		for _, child := range node.Content {
			m.resolvePathsWalk(child, p.Next(tree.PathMatchList), ctx, rewrite)
		}
	case yaml.ScalarNode:
		for _, pattern := range resolvablePathPatterns {
			if !p.Matches(pattern) {
				continue
			}
			node.Value = rewrite(node.Value, ctx)
			return
		}
	}
}

// convertShortVolume turns a short-syntax bind volume scalar
// ("./local:/app:ro") into its long-syntax mapping equivalent while
// preserving the relative source path. The conversion is needed because
// the downstream transform.Canonical pass would otherwise invoke
// format.ParseVolume after the source has been rewritten to a project-
// relative path that isFilePath might no longer recognise as bind, and
// the volume would end up classified as a named volume.
//
// The source path is left as-is. The caller's resolveLongVolume pass
// rewrites it exactly once, using the NodeContext attached to the new
// source scalar (which we register here to mirror the original).
//
// Returns nil when the scalar is not a bind path (named volume, anonymous
// volume, parse error, source already absolute), so the caller leaves the
// node in place.
func convertShortVolume(node *yaml.Node, ctx *types.NodeContext, _ func(string, *types.NodeContext) string) *yaml.Node {
	if ctx == nil || node.Value == "" {
		return nil
	}
	vol, err := format.ParseVolume(node.Value)
	if err != nil {
		return nil
	}
	if vol.Type != types.VolumeTypeBind || vol.Source == "" {
		return nil
	}
	pairs := []override.KeyValue{
		{Key: "type", Value: override.NewScalar(types.VolumeTypeBind)},
		{Key: "source", Value: override.NewScalar(vol.Source)},
		{Key: "target", Value: override.NewScalar(vol.Target)},
		{Key: "bind", Value: override.NewMapping(override.KeyValue{
			Key: "create_host_path", Value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
		})},
	}
	if vol.ReadOnly {
		pairs = append(pairs, override.KeyValue{Key: "read_only", Value: &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}})
	}
	return override.NewMapping(pairs...)
}

// resolveLongVolume rewrites the source key of a long-syntax bind volume
// mapping. Other volume types are left alone.
func resolveLongVolume(node *yaml.Node, ctx *types.NodeContext, rewrite func(string, *types.NodeContext) string) {
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
	src.Value = rewrite(src.Value, ctx)
}
