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
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/dotenv"
	"github.com/compose-spec/compose-go/v3/internal/node"
	interp "github.com/compose-spec/compose-go/v3/interpolation"
	"github.com/compose-spec/compose-go/v3/paths"
	"github.com/compose-spec/compose-go/v3/template"
	"github.com/compose-spec/compose-go/v3/types"
)

// CollectIncludeLayers reads the top-level `include` block from a parent
// layer and returns the list of direct child layers it materializes. Each
// child carries its own SourceContext, capturing the include block's
// `project_directory` and the environment resolved from its `env_file`
// entries — which is what allows the merge / interpolate phases downstream
// to honor per-include context lazily.
//
// The include block is interpolated in the parent's SourceContext before
// any path is resolved, because the path / project_directory / env_file
// scalars themselves may contain ${VAR} references that must be substituted
// in the *parent* environment. This is the one point in the v3 pipeline
// where interpolation is performed eagerly; everywhere else, scalars are
// interpolated after merge in their own SourceContext.
//
// The function only produces *direct* children. The orchestrator is
// responsible for recursing into each child to process its own include
// block. CollectIncludeLayers leaves the parent's `include` mapping entry
// in place; the orchestrator removes it once all children have been
// collected.
//
// CollectIncludeLayers does not perform cross-file merging; it only loads
// included files into stand-alone Layers. Cycle detection is delegated to
// the orchestrator, which keeps a global set of resolved filenames.
func CollectIncludeLayers(ctx context.Context, parent *node.Layer, opts *Options) ([]*node.Layer, error) {
	includeNode := layerMappingField(parent.Node, "include")
	if includeNode == nil {
		return nil, nil
	}
	if includeNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("`include` must be a list, got %s", kindName(includeNode.Kind))
	}

	if err := interpolateIncludeBlock(includeNode, parent.Context, opts); err != nil {
		return nil, err
	}

	var layers []*node.Layer
	for _, entry := range includeNode.Content {
		entryLayers, err := collectOneInclude(ctx, parent, entry, opts)
		if err != nil {
			return nil, err
		}
		layers = append(layers, entryLayers...)
	}
	return layers, nil
}

// collectOneInclude turns a single include entry (string short form or
// mapping long form) into its corresponding child Layers. Multi-path entries
// produce one Layer per resolved path, in declaration order, the same way
// the v2 ApplyInclude does.
func collectOneInclude(ctx context.Context, parent *node.Layer, entry *yaml.Node, opts *Options) ([]*node.Layer, error) {
	cfg, err := readIncludeEntry(entry)
	if err != nil {
		return nil, err
	}

	parentWD := parent.Context.WorkingDir
	resolvedPaths, projectDir, err := resolveIncludePaths(ctx, cfg, parentWD, opts)
	if err != nil {
		return nil, err
	}

	envFiles, env, err := resolveIncludeEnvironment(cfg, projectDir, parentWD, parent.Context.Environment)
	if err != nil {
		return nil, err
	}

	childCtx := &node.SourceContext{
		WorkingDir:  projectDir,
		Environment: env,
		EnvFiles:    envFiles,
		Parent:      parent.Context,
	}

	var layers []*node.Layer
	for _, p := range resolvedPaths {
		layerCtx := *childCtx
		layerCtx.File = p
		fileLayers, err := LoadLayer(ctx, types.ConfigFile{Filename: p}, &layerCtx, opts)
		if err != nil {
			return nil, err
		}
		// v2 ApplyInclude always forces ResolvePaths=true for the include
		// sub-load. v3 does the same here; subsequent passes in the
		// orchestrator short-circuit on already-absolute paths via
		// filepath.IsAbs in absScalar, so this single resolution does not
		// double up with the outer pass when ResolvePaths is also true.
		//
		// extends.file is deliberately left untouched: the orchestrator
		// extends pass needs the original relative reference so it can
		// re-resolve through the loaded layer's ResourceLoader (re-rooted
		// at the include working directory in LoadV3), exactly as v2
		// ApplyExtends does inside the recursive loadYamlModel of an
		// include. Resolving it here would lead to double-joining when
		// the orchestrator runs loader.Load on the already-absolutized
		// path.
		// v2 ApplyInclude force-runs ResolvePaths=true on the include
		// sub-load even when the outer load opted out, so include paths
		// become absolute and the outer pass never has to touch them
		// again. v3 only runs the sub-resolve when the outer load opted
		// in: otherwise leave the include's relative paths untouched so
		// `build: .` declared next to the include stays "." after the
		// merge (TestIncludeRelative). When skipping, run a lightweight
		// cleaning pass so cosmetic forms (`./`, `./foo`) collapse to
		// their canonical relative spelling (`.`, `foo`) the same way
		// filepath.Join in the v2 sub-resolve would have.
		if opts.ResolvePaths {
			var remotes []paths.RemoteResource
			for _, loader := range opts.RemoteResourceLoaders() {
				remotes = append(remotes, loader.Accept)
			}
			for _, layer := range fileLayers {
				if err := paths.ResolveRelativePathsNode(layer.Node, paths.NodeResolverOptions{
					WorkingDirFor: func(_ *yaml.Node) string {
						return projectDir
					},
					Remotes: remotes,
					ExcludePaths: []string{
						"services.*.extends.file",
					},
				}); err != nil {
					return nil, err
				}
				if layer.Context != nil {
					layer.Context.PathsPreResolved = true
				}
			}
		} else {
			for _, layer := range fileLayers {
				if err := paths.ResolveRelativePathsNode(layer.Node, paths.NodeResolverOptions{
					WorkingDirFor: func(_ *yaml.Node) string {
						return "."
					},
					ExcludePaths: []string{
						"services.*.extends.file",
					},
				}); err != nil {
					return nil, err
				}
				if layer.Context != nil {
					layer.Context.PathsPreResolved = true
				}
			}
		}
		layers = append(layers, fileLayers...)
	}
	return layers, nil
}

// readIncludeEntry normalizes a single include sequence entry. A bare
// scalar is promoted to a single-path long form; a mapping is decoded
// natively into IncludeConfig via yaml.v4 (StringList now implements
// UnmarshalYAML so short-form path / env_file values are accepted).
func readIncludeEntry(entry *yaml.Node) (types.IncludeConfig, error) {
	if entry == nil {
		return types.IncludeConfig{}, fmt.Errorf("empty include entry")
	}
	switch entry.Kind {
	case yaml.ScalarNode:
		return types.IncludeConfig{Path: types.StringList{entry.Value}}, nil
	case yaml.MappingNode:
		var cfg types.IncludeConfig
		if err := entry.Decode(&cfg); err != nil {
			return types.IncludeConfig{}, fmt.Errorf("invalid include entry: %w", err)
		}
		return cfg, nil
	}
	return types.IncludeConfig{}, fmt.Errorf("include entry must be a string or a mapping, got %s", kindName(entry.Kind))
}

// resolveIncludePaths walks each entry in cfg.Path through the configured
// ResourceLoaders and returns the absolute local paths plus the
// project_directory that applies to the included files. The first path
// defines the project_directory when none is declared; later paths in the
// same entry are treated as overrides loaded from the same directory.
//
// The returned project_directory is the absolute path to the include's
// project root. The per-include path resolution pass uses it as the
// WorkingDir to absolutize relative paths inside the included tree, and
// the PathsPreResolved flag set on the layer's SourceContext prevents the
// orchestrator outer pass from re-resolving them.
func resolveIncludePaths(ctx context.Context, cfg types.IncludeConfig, parentWD string, opts *Options) ([]string, string, error) {
	var resolved []string
	projectDir := cfg.ProjectDirectory
	for i, p := range cfg.Path {
		_, fullPath, err := resolveResourceWithLoader(ctx, opts, p)
		if err != nil {
			return nil, "", err
		}
		if i == 0 {
			switch {
			case projectDir == "":
				projectDir = filepath.Dir(fullPath)
			case !filepath.IsAbs(projectDir):
				projectDir = filepath.Join(parentWD, projectDir)
			}
		}
		resolved = append(resolved, fullPath)
	}
	return resolved, projectDir, nil
}

// resolveIncludeEnvironment loads the env_file(s) declared on the include
// block and merges them on top of the parent environment. Relative env_file
// paths are resolved against parentWD (matching v2 behavior); a single
// `/dev/null` entry disables environment inheritance for that file.
//
// When cfg.EnvFile is empty, an implicit `<project_directory>/.env` is used
// if it exists — same convention as v2.
func resolveIncludeEnvironment(cfg types.IncludeConfig, projectDir, parentWD string, parentEnv types.Mapping) ([]string, types.Mapping, error) {
	envFiles := []string{}
	if len(cfg.EnvFile) == 0 {
		f := filepath.Join(projectDir, ".env")
		if s, err := os.Stat(f); err == nil && !s.IsDir() {
			envFiles = []string{f}
		}
	} else {
		for _, f := range cfg.EnvFile {
			if f == "/dev/null" {
				continue
			}
			if !filepath.IsAbs(f) {
				f = filepath.Join(parentWD, f)
			}
			s, err := os.Stat(f)
			if err != nil {
				return nil, nil, err
			}
			if s.IsDir() {
				return nil, nil, fmt.Errorf("%s is not a file", f)
			}
			envFiles = append(envFiles, f)
		}
	}

	envFromFile, err := dotenv.GetEnvFromFile(parentEnv, envFiles)
	if err != nil {
		return nil, nil, err
	}
	merged := parentEnv.Clone().Merge(envFromFile)
	return envFiles, merged, nil
}

// resolveResourceWithLoader finds the ResourceLoader in opts that accepts
// p and returns it together with the resolved absolute path produced by
// its Load method. Mirrors the v2 dispatch logic inside ApplyInclude and
// is the only resource-lookup helper kept in v3 because every caller needs
// the loader handle for follow-up loader.Dir computations.
func resolveResourceWithLoader(ctx context.Context, opts *Options, p string) (ResourceLoader, string, error) {
	for _, loader := range opts.ResourceLoaders {
		if !loader.Accept(p) {
			continue
		}
		full, err := loader.Load(ctx, p)
		if err != nil {
			return nil, "", err
		}
		return loader, full, nil
	}
	return nil, "", fmt.Errorf("no ResourceLoader accepted %q", p)
}

// interpolateIncludeBlock runs InterpolateNode on the include sub-tree with
// the parent SourceContext. This is the one place in the v3 pipeline where
// interpolation is eager: the include path / project_directory / env_file
// scalars must be substituted before paths are resolved, otherwise the
// loader has no way to find the referenced files.
func interpolateIncludeBlock(includeNode *yaml.Node, sc *node.SourceContext, opts *Options) error {
	if opts != nil && opts.SkipInterpolation {
		return nil
	}
	lookup := func(key string) (string, bool) {
		if sc == nil {
			return "", false
		}
		v, ok := sc.Environment[key]
		return v, ok
	}
	substitute := template.Substitute
	if opts != nil && opts.Interpolate != nil && opts.Interpolate.Substitute != nil {
		substitute = opts.Interpolate.Substitute
	}
	return interp.InterpolateNode(includeNode, interp.NodeOptions{
		LookupValue: lookup,
		Substitute:  substitute,
	})
}

// layerMappingField returns the value Node for key inside a Layer's root
// mapping, or nil when absent / not a mapping.
func layerMappingField(root *yaml.Node, key string) *yaml.Node {
	if root == nil {
		return nil
	}
	r := root
	if r.Kind == yaml.DocumentNode && len(r.Content) == 1 {
		r = r.Content[0]
	}
	if r.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(r.Content); i += 2 {
		if r.Content[i].Value == key {
			return r.Content[i+1]
		}
	}
	return nil
}

// kindName returns a human-readable label for a yaml.Kind, used in error
// messages. yaml.v4 exposes the constants but no String() helper.
func kindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	}
	return "unknown"
}
