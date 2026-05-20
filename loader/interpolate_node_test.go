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
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestInterpolateTree_UsesPerNodeEnv(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: alpine:${TAG}\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
		Environment: types.Mapping{"TAG": "3.20"},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))

	assert.NilError(t, m.interpolateTree(m.layers[0].Root, tree.NewPath(), m.layers[0].Context))

	imageNode := findScalarUnderKey(t, m.layers[0].Root, "image")
	assert.Equal(t, imageNode.Value, "alpine:3.20")
}

func TestInterpolateTree_AppliesTypeCast(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: alpine\n    scale: ${REPLICAS}\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
		Environment: types.Mapping{"REPLICAS": "5"},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))

	assert.NilError(t, m.interpolateTree(m.layers[0].Root, tree.NewPath(), m.layers[0].Context))

	scaleNode := findScalarUnderKey(t, m.layers[0].Root, "scale")
	assert.Equal(t, scaleNode.Value, "5")
	assert.Equal(t, scaleNode.Tag, "!!int")
}

func TestInterpolateTree_DistinctEnvPerLayer(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: alpine:${TAG}\n"), 0o644))

	// Two layers with different Env mappings, both parsed from the same file
	// content but registered under distinct contexts.
	m := newComposeModel(types.ConfigDetails{
		WorkingDir: tmpdir,
	}, &Options{})

	root1, err := loadYamlFileNode(types.ConfigFile{Filename: path})
	assert.NilError(t, err)
	ctx1 := &types.NodeContext{Source: path, WorkingDir: tmpdir, Env: types.Mapping{"TAG": "3.19"}}
	m.layers = append(m.layers, &Layer{Root: root1, Context: ctx1})
	m.registerNodes(root1, ctx1)

	root2, err := loadYamlFileNode(types.ConfigFile{Filename: path})
	assert.NilError(t, err)
	ctx2 := &types.NodeContext{Source: path, WorkingDir: tmpdir, Env: types.Mapping{"TAG": "3.20"}}
	m.layers = append(m.layers, &Layer{Root: root2, Context: ctx2})
	m.registerNodes(root2, ctx2)

	assert.NilError(t, m.interpolateTree(root1, tree.NewPath(), ctx1))
	assert.NilError(t, m.interpolateTree(root2, tree.NewPath(), ctx2))

	assert.Equal(t, findScalarUnderKey(t, root1, "image").Value, "alpine:3.19")
	assert.Equal(t, findScalarUnderKey(t, root2, "image").Value, "alpine:3.20")
}

func TestInterpolateTree_LeavesScalarsWithoutDollarUntouched(t *testing.T) {
	scalar := &yaml.Node{Kind: yaml.ScalarNode, Value: "nginx:latest", Tag: "!!str"}
	m := &ComposeModel{contexts: map[*yaml.Node]*types.NodeContext{}}
	assert.NilError(t, m.interpolateScalar(scalar, tree.NewPath("services", "app", "image"), &types.NodeContext{Env: types.Mapping{}}))
	assert.Equal(t, scalar.Value, "nginx:latest")
}

// findScalarUnderKey returns the first ScalarNode reached by walking down
// from node and following the (depth-first) edge labeled key in any mapping
// encountered. Used to keep tests concise.
func findScalarUnderKey(t *testing.T, node *yaml.Node, key string) *yaml.Node {
	t.Helper()
	if node == nil {
		t.Fatalf("nil node while looking up %q", key)
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range node.Content {
			if res := tryFind(c, key); res != nil {
				return res
			}
		}
	case yaml.MappingNode:
		if res := tryFind(node, key); res != nil {
			return res
		}
	}
	t.Fatalf("key %q not found", key)
	return nil
}

func tryFind(node *yaml.Node, key string) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key && node.Content[i+1].Kind == yaml.ScalarNode {
				return node.Content[i+1]
			}
		}
	}
	for _, c := range node.Content {
		if res := tryFind(c, key); res != nil {
			return res
		}
	}
	return nil
}
