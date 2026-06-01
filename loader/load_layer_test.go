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
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/types"
)

func sourceCtx(workingDir string) *node.SourceContext {
	return &node.SourceContext{
		File:        "test.yaml",
		WorkingDir:  workingDir,
		Environment: types.Mapping{},
	}
}

func TestLoadLayer_FromContent(t *testing.T) {
	file := types.ConfigFile{
		Filename: "(inline)",
		Content: []byte(`
services:
  web:
    image: nginx
`),
	}
	layers, err := LoadLayer(context.TODO(), file, sourceCtx("/work"), &Options{})
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 1)
	assert.Equal(t, layers[0].Context.WorkingDir, "/work")
	assert.Equal(t, layers[0].Node.Kind, yaml.MappingNode)

	var m map[string]any
	assert.NilError(t, layers[0].Node.Decode(&m))
	assert.Equal(t, m["services"].(map[string]any)["web"].(map[string]any)["image"], "nginx")
}

func TestLoadLayer_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(`name: from-file
services:
  api:
    image: alpine
`), 0o644))

	layers, err := LoadLayer(context.TODO(), types.ConfigFile{Filename: path}, sourceCtx(dir), &Options{})
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 1)
	var m map[string]any
	assert.NilError(t, layers[0].Node.Decode(&m))
	assert.Equal(t, m["name"], "from-file")
}

func TestLoadLayer_UnfoldsAliasesAndMergeKeys(t *testing.T) {
	file := types.ConfigFile{
		Filename: "(inline)",
		Content: []byte(`
defaults: &defaults
  image: nginx
  restart: always
services:
  web:
    <<: *defaults
    image: caddy
`),
	}
	layers, err := LoadLayer(context.TODO(), file, sourceCtx("/work"), &Options{})
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 1)

	// After NormalizeAliases: no AliasNode, no `<<` key. Surrounding wins.
	var sawAlias, sawMergeKey bool
	var visit func(*yaml.Node)
	visit = func(n *yaml.Node) {
		if n == nil {
			return
		}
		if n.Kind == yaml.AliasNode {
			sawAlias = true
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == "<<" {
					sawMergeKey = true
				}
				visit(n.Content[i+1])
			}
			return
		}
		for _, c := range n.Content {
			visit(c)
		}
	}
	visit(layers[0].Node)
	assert.Assert(t, !sawAlias, "no alias should remain")
	assert.Assert(t, !sawMergeKey, "no merge key should remain")

	var m map[string]any
	assert.NilError(t, layers[0].Node.Decode(&m))
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "caddy")
	assert.Equal(t, web["restart"], "always")
}

func TestLoadLayer_CollectsResetPaths(t *testing.T) {
	file := types.ConfigFile{
		Filename: "(inline)",
		Content: []byte(`
services:
  web:
    image: nginx
    command: !reset null
`),
	}
	layers, err := LoadLayer(context.TODO(), file, sourceCtx("/work"), &Options{})
	assert.NilError(t, err)
	paths := layers[0].ResetPaths()
	assert.Equal(t, len(paths), 1)
	assert.Equal(t, paths[0].String(), "services.web.command")
}

func TestLoadLayer_MultiDocument(t *testing.T) {
	file := types.ConfigFile{
		Filename: "(inline)",
		Content: []byte(`
name: first
---
name: second
`),
	}
	layers, err := LoadLayer(context.TODO(), file, sourceCtx("/work"), &Options{})
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 2)

	var m1, m2 map[string]any
	assert.NilError(t, layers[0].Node.Decode(&m1))
	assert.NilError(t, layers[1].Node.Decode(&m2))
	assert.Equal(t, m1["name"], "first")
	assert.Equal(t, m2["name"], "second")
}

func TestLoadLayer_FromPrebuiltNode(t *testing.T) {
	// Build a yaml.Node by parsing then passing it as ConfigFile.Node.
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte("name: pre-parsed\n"), &doc))

	layers, err := LoadLayer(context.TODO(), types.ConfigFile{Node: &doc}, sourceCtx("/work"), &Options{})
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 1)
	var m map[string]any
	assert.NilError(t, layers[0].Node.Decode(&m))
	assert.Equal(t, m["name"], "pre-parsed")
}

func TestLoadLayer_RejectsAliasBomb(t *testing.T) {
	// Document under MaxNodeVisits cap to verify the resolver propagates
	// its error through LoadLayer.
	file := types.ConfigFile{
		Filename: "(inline)",
		Content: []byte(`
services:
  web:
    extends: &self {service: web}
    <<: *self
`),
	}
	// Lower the cap so the resolver fires.
	_, err := LoadLayer(context.TODO(), file, sourceCtx("/work"), &Options{MaxNodeVisits: 3})
	assert.ErrorContains(t, err, "exceeds maximum node visit limit")
}
