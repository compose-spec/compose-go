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

	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestParseLayers_SingleFile(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: alpine\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
		Environment: types.Mapping{"FOO": "bar"},
	}, &Options{})

	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.Equal(t, len(m.layers), 1)
	assert.Equal(t, m.layers[0].Context.Source, path)
	assert.Equal(t, m.layers[0].Context.WorkingDir, tmpdir)
	assert.Equal(t, m.layers[0].Context.Env["FOO"], "bar")
	assert.Assert(t, m.layers[0].Root != nil)
}

func TestParseLayers_RegistersEveryNode(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: alpine\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))

	layerCtx := m.layers[0].Context
	visited := 0
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n == nil {
			return
		}
		visited++
		ctx, ok := m.contexts[n]
		assert.Check(t, ok, "node not registered: line %d col %d", n.Line, n.Column)
		assert.Equal(t, ctx, layerCtx)
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(m.layers[0].Root)
	assert.Assert(t, visited > 3, "expected at least a few nodes, got %d", visited)
}

func TestParseLayers_MultipleFiles(t *testing.T) {
	tmpdir := t.TempDir()
	a := filepath.Join(tmpdir, "a.yaml")
	b := filepath.Join(tmpdir, "b.yaml")
	assert.NilError(t, os.WriteFile(a, []byte("services:\n  app:\n    image: alpine\n"), 0o644))
	assert.NilError(t, os.WriteFile(b, []byte("services:\n  app:\n    command: echo\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{
			{Filename: a},
			{Filename: b},
		},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.Equal(t, len(m.layers), 2)
	assert.Equal(t, m.layers[0].Context.Source, a)
	assert.Equal(t, m.layers[1].Context.Source, b)
	assert.Assert(t, m.layers[0].Context != m.layers[1].Context)
}

func TestParseLayers_DuplicateKeysRejected(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: a\n    image: b\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	err := m.parseLayers(m.configDetails)
	assert.ErrorContains(t, err, "already defined")
}

func TestParseLayers_EmptyFileIgnored(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(""), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.Equal(t, len(m.layers), 0)
}

func TestContextFor_FallsBackToFirstLayer(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte("services:\n  app:\n    image: alpine\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))

	unknown := &yaml.Node{Kind: yaml.ScalarNode, Value: "unknown"}
	got := m.contextFor(unknown)
	assert.Equal(t, got, m.layers[0].Context)
}

func TestCheckNonStringKeys_Rejects(t *testing.T) {
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "42", Tag: "!!int"},
			{Kind: yaml.ScalarNode, Value: "x", Tag: "!!str"},
		}},
	}}
	err := checkNonStringKeys("test.yaml", doc, "")
	assert.ErrorContains(t, err, "non-string key")
}

func TestCheckNonStringKeys_AcceptsTopLevelStringKeys(t *testing.T) {
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "services", Tag: "!!str"},
			{Kind: yaml.MappingNode, Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "app", Tag: "!!str"},
				{Kind: yaml.ScalarNode, Value: "alpine", Tag: "!!str"},
			}},
		}},
	}}
	assert.NilError(t, checkNonStringKeys("test.yaml", doc, ""))
}
