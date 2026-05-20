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
	"strings"

	"github.com/compose-spec/compose-go/v3/dotenv"
	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// applyIncludeNodes expands include: directives across all layers. Each
// include entry becomes additional Layers grafted into the model with their
// own NodeContext (WorkingDir, Env, Parent). Included trees are NOT mutated
// or pre-resolved; the path resolution and environment resolution passes
// will consult their per-node contexts when they need them.
//
// Included layers are inserted before their parent so that parent values win
// during the merge pass. Includes nested inside an included file are
// expanded recursively.
func (m *ComposeModel) applyIncludeNodes(ctx context.Context) error {
	var expanded []*Layer
	for _, layer := range m.layers {
		chain := []string{layer.Context.Source}
		out, err := m.expandIncludesOf(ctx, layer, chain)
		if err != nil {
			return err
		}
		expanded = append(expanded, out...)
	}
	m.layers = expanded
	return nil
}

// expandIncludesOf processes the include directive of a single layer,
// recursively expanding any include declared inside an included file. The
// returned slice is ordered from deepest include to layer itself.
//
// chain is the running list of file paths currently being expanded along
// the include nesting branch. It is used to detect a true cycle (a file
// that includes itself, possibly through intermediates) without flagging a
// file that legitimately appears twice in two unrelated include branches.
func (m *ComposeModel) expandIncludesOf(ctx context.Context, layer *Layer, chain []string) ([]*Layer, error) {
	root := unwrapDocument(layer.Root)
	_, includeNode := override.FindKey(root, "include")
	if includeNode == nil {
		return []*Layer{layer}, nil
	}
	if includeNode.Kind != yaml.SequenceNode {
		return nil, nodeErrf(layer.Context, includeNode, "include must be a sequence")
	}
	var out []*Layer
	for _, entry := range includeNode.Content {
		incLayers, err := m.loadIncludeEntry(ctx, layer, entry, chain)
		if err != nil {
			return nil, err
		}
		for _, incLayer := range incLayers {
			nested, err := m.expandIncludesOf(ctx, incLayer, append(chain, incLayer.Context.Source))
			if err != nil {
				return nil, err
			}
			out = append(out, nested...)
		}
	}
	override.DeleteKey(root, "include")
	out = append(out, layer)
	return out, nil
}

// loadIncludeEntry parses a single include entry and returns the new Layers.
// An entry may be a scalar (a single path) or a mapping (path / paths,
// project_directory, env_file).
func (m *ComposeModel) loadIncludeEntry(ctx context.Context, parent *Layer, entry *yaml.Node, chain []string) ([]*Layer, error) {
	paths, projectDir, envFiles, err := readIncludeEntry(entry)
	if err != nil {
		return nil, wrapNodeErr(parent.Context, entry, err)
	}

	for i, p := range paths {
		resolved, err := m.resolveIncludePath(ctx, parent, p)
		if err != nil {
			return nil, wrapNodeErr(parent.Context, entry, err)
		}
		paths[i] = resolved
	}

	if err := detectIncludeCycle(chain, paths); err != nil {
		return nil, wrapNodeErr(parent.Context, entry, err)
	}

	workDir := projectDir
	switch {
	case workDir == "":
		workDir = filepath.Dir(paths[0])
	case !filepath.IsAbs(workDir):
		workDir = filepath.Join(parent.Context.WorkingDir, workDir)
	}

	// Notify user-supplied listeners that an include is being processed.
	// The legacy loader emits this event for each include entry; some
	// consumers (e.g. docker compose publish) rely on it to refuse
	// publishing when local includes are present.
	m.opts.ProcessEvent("include", map[string]any{
		"path":       paths,
		"workingdir": workDir,
	})

	env, err := m.computeIncludeEnv(parent, envFiles)
	if err != nil {
		return nil, wrapNodeErr(parent.Context, entry, err)
	}

	var layers []*Layer
	for _, p := range paths {
		m.loadedFiles = append(m.loadedFiles, p)
		node, err := loadYamlFileNode(types.ConfigFile{Filename: p})
		if err != nil {
			return nil, nodeErrf(parent.Context, entry, "loading include %s: %v", p, err)
		}
		if node == nil {
			continue
		}
		nodeCtx := &types.NodeContext{
			Source:     p,
			WorkingDir: workDir,
			Env:        env,
			Parent:     parent.Context,
		}
		m.registerNodes(node, nodeCtx)
		layer := &Layer{Root: node, Context: nodeCtx}

		if !m.opts.SkipExtends {
			if err := m.applyExtendsNode(ctx, layer); err != nil {
				return nil, err
			}
		}
		// Pre-inject build.context defaults on the included tree so they are
		// resolved against the included file's working directory rather than
		// the main project directory. The legacy loader does this implicitly
		// because it runs transform.SetDefaultValues + ResolveRelativePaths on
		// the included dict before merging it into the main one.
		injectBuildContextDefault(node)
		layers = append(layers, layer)
	}
	return layers, nil
}

// readIncludeEntry parses one include entry into (paths, projectDir, envFiles).
func readIncludeEntry(entry *yaml.Node) ([]string, string, []string, error) {
	switch entry.Kind {
	case yaml.ScalarNode:
		if entry.Value == "" {
			return nil, "", nil, fmt.Errorf("include entry has no path")
		}
		return []string{entry.Value}, "", nil, nil
	case yaml.MappingNode:
		var paths []string
		var projectDir string
		var envFiles []string
		_, pathNode := override.FindKey(entry, "path")
		switch {
		case pathNode == nil:
			return nil, "", nil, fmt.Errorf("include entry has no path")
		case pathNode.Kind == yaml.ScalarNode:
			paths = []string{pathNode.Value}
		case pathNode.Kind == yaml.SequenceNode:
			for _, p := range pathNode.Content {
				paths = append(paths, p.Value)
			}
		}
		if _, pdNode := override.FindKey(entry, "project_directory"); pdNode != nil {
			projectDir = pdNode.Value
		}
		if _, efNode := override.FindKey(entry, "env_file"); efNode != nil {
			switch efNode.Kind {
			case yaml.ScalarNode:
				envFiles = []string{efNode.Value}
			case yaml.SequenceNode:
				for _, e := range efNode.Content {
					envFiles = append(envFiles, e.Value)
				}
			}
		}
		if len(paths) == 0 {
			return nil, "", nil, fmt.Errorf("include entry has no path")
		}
		return paths, projectDir, envFiles, nil
	default:
		return nil, "", nil, fmt.Errorf("include entry must be a string or mapping")
	}
}

func (m *ComposeModel) resolveIncludePath(ctx context.Context, parent *Layer, p string) (string, error) {
	for _, loader := range m.opts.RemoteResourceLoaders() {
		if !loader.Accept(p) {
			continue
		}
		absPath, err := loader.Load(ctx, p)
		if err != nil {
			return "", fmt.Errorf("loading include %s: %w", p, err)
		}
		return absPath, nil
	}
	if !filepath.IsAbs(p) {
		return filepath.Join(parent.Context.WorkingDir, p), nil
	}
	return p, nil
}

// detectIncludeCycle returns a non-nil error when one of paths already
// appears in the current expansion chain — i.e. an include is about to load
// a file that is an ancestor of the current include site. A file legitimately
// reused across two unrelated include branches is not a cycle.
func detectIncludeCycle(chain, paths []string) error {
	for _, p := range paths {
		for _, in := range chain {
			if in != p {
				continue
			}
			full := append([]string{}, chain...)
			full = append(full, p)
			return fmt.Errorf("include cycle detected:\n%s\n include %s",
				full[0], strings.Join(full[1:], "\n include "))
		}
	}
	return nil
}

// computeIncludeEnv builds the environment available inside the included
// files. The parent's Env is cloned, then env_file entries declared in the
// include directive are loaded and merged on top.
func (m *ComposeModel) computeIncludeEnv(parent *Layer, envFiles []string) (types.Mapping, error) {
	env := parent.Context.Env.Clone()
	if len(envFiles) == 0 {
		return env, nil
	}
	resolved := make([]string, len(envFiles))
	for i, f := range envFiles {
		if filepath.IsAbs(f) {
			resolved[i] = f
		} else {
			resolved[i] = filepath.Join(parent.Context.WorkingDir, f)
		}
	}
	fromFile, err := dotenv.GetEnvFromFile(env, resolved)
	if err != nil {
		return nil, err
	}
	return env.Merge(fromFile), nil
}
