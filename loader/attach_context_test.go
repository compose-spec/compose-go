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

	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestAttachEnvFileContexts_FromIncludedService(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))

	// sub/local.env brings EXTRA_PATH used to compute another env_file path
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "local.env"), []byte("EXTRA_PATH=extra.env\n"), 0o644))

	subYaml := `
services:
  app:
    image: alpine
    env_file:
      - ./vars.env
`
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte(subYaml), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
    env_file: sub/local.env
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: types.Mapping{},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyIncludeNodes(context.TODO()))

	// Project is constructed manually here: full decoding happens in a later
	// phase. The test focuses on what attachEnvFileContexts does given a
	// merged tree and a project whose Services.EnvFiles is already
	// populated.
	merged := unwrapDocument(m.layers[0].Root)
	project := &types.Project{
		Services: types.Services{
			"app": types.ServiceConfig{
				Name:     "app",
				Image:    "alpine",
				EnvFiles: []types.EnvFile{{Path: "./vars.env"}},
			},
		},
	}

	m.attachEnvFileContexts(merged, project)

	app := project.Services["app"]
	assert.Equal(t, len(app.EnvFiles), 1)
	assert.Assert(t, app.EnvFiles[0].Context != nil, "env_file should have received its loading context")
	// The context's WorkingDir matches the included file's directory.
	assert.Equal(t, app.EnvFiles[0].Context.WorkingDir, subdir)
	// And the context's Env carries the variable introduced by include.env_file.
	assert.Equal(t, app.EnvFiles[0].Context.Env["EXTRA_PATH"], "extra.env")
}

func TestAttachEnvFileContexts_ScalarShorthand(t *testing.T) {
	tmpdir := t.TempDir()
	p := filepath.Join(tmpdir, "compose.yaml")
	src := `
services:
  app:
    env_file: ./vars.env
`
	assert.NilError(t, os.WriteFile(p, []byte(src), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: p}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))

	merged := unwrapDocument(m.layers[0].Root)
	project := &types.Project{
		Services: types.Services{
			"app": types.ServiceConfig{
				Name:     "app",
				EnvFiles: []types.EnvFile{{Path: "./vars.env"}},
			},
		},
	}
	m.attachEnvFileContexts(merged, project)

	app := project.Services["app"]
	assert.Assert(t, app.EnvFiles[0].Context != nil)
	assert.Equal(t, app.EnvFiles[0].Context.Source, p)
}

func TestEnvFileEntryNodes_AllForms(t *testing.T) {
	src := `
env_file: ./single.env
`
	root := mustParse(t, src)
	_, ef := override.FindKey(unwrapDocument(root), "env_file")
	entries := envFileEntryNodes(ef)
	assert.Equal(t, len(entries), 1)
	assert.Equal(t, entries[0].Value, "./single.env")

	src = `
env_file:
  - a.env
  - b.env
`
	root = mustParse(t, src)
	_, ef = override.FindKey(unwrapDocument(root), "env_file")
	entries = envFileEntryNodes(ef)
	assert.Equal(t, len(entries), 2)
	assert.Equal(t, entries[0].Value, "a.env")
	assert.Equal(t, entries[1].Value, "b.env")

	src = `
env_file:
  - path: c.env
    required: false
`
	root = mustParse(t, src)
	_, ef = override.FindKey(unwrapDocument(root), "env_file")
	entries = envFileEntryNodes(ef)
	assert.Equal(t, len(entries), 1)
	assert.Equal(t, entries[0].Value, "c.env")
}

func mustParse(t *testing.T, src string) *yaml.Node {
	t.Helper()
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "in.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(src), 0o644))
	node, err := loadYamlFileNode(types.ConfigFile{Filename: path})
	assert.NilError(t, err)
	return node
}
