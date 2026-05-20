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

func TestResolvePathsPass_BuildContextRelativeToService(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	src := `
services:
  app:
    build:
      context: ./image
`
	assert.NilError(t, os.WriteFile(path, []byte(src), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{ResolvePaths: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	m.resolvePathsPass(m.layers[0].Root)

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, build := override.FindKey(app, "build")
	_, ctx := override.FindKey(build, "context")
	assert.Equal(t, ctx.Value, filepath.Join(tmpdir, "image"))
}

func TestResolvePathsPass_PreservesAbsolutePath(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	src := `
services:
  app:
    build:
      context: /already/absolute
`
	assert.NilError(t, os.WriteFile(path, []byte(src), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	m.resolvePathsPass(m.layers[0].Root)

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, build := override.FindKey(app, "build")
	_, ctx := override.FindKey(build, "context")
	assert.Equal(t, ctx.Value, "/already/absolute")
}

func TestResolvePathsPass_PreservesRemoteURL(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	src := `
services:
  app:
    build:
      context: https://github.com/example/repo.git
`
	assert.NilError(t, os.WriteFile(path, []byte(src), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	m.resolvePathsPass(m.layers[0].Root)

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, build := override.FindKey(app, "build")
	_, ctx := override.FindKey(build, "context")
	assert.Equal(t, ctx.Value, "https://github.com/example/repo.git")
}

func TestResolvePathsPass_EnvFileNotResolvedAtThisPass(t *testing.T) {
	tmpdir := t.TempDir()
	p := filepath.Join(tmpdir, "compose.yaml")
	src := `
services:
  app:
    env_file:
      - ./vars.env
`
	assert.NilError(t, os.WriteFile(p, []byte(src), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: p}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	m.resolvePathsPass(m.layers[0].Root)

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, envFile := override.FindKey(app, "env_file")
	// env_file.path must remain relative — it is resolved on demand by
	// WithServicesEnvironmentResolved using EnvFile.Context.
	assert.Equal(t, envFile.Content[0].Value, "./vars.env")
}

func TestResolvePathsPass_BuildContextOfIncludedService(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))

	subYaml := `
services:
  included:
    build:
      context: ./local
`
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte(subYaml), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
	}, &Options{SkipExtends: true, ResolvePaths: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyIncludeNodes(context.TODO()))
	m.resolvePathsPass(m.layers[0].Root)

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, included := override.FindKey(services, "included")
	_, build := override.FindKey(included, "build")
	_, ctx := override.FindKey(build, "context")
	// The included build.context is resolved against the included file's
	// directory (sub), NOT against the top-level working directory.
	assert.Equal(t, ctx.Value, filepath.Join(subdir, "local"))
}

func TestResolveShortVolume_RewritesBindSource(t *testing.T) {
	tmpdir := t.TempDir()
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "./data:/var/data", Tag: "!!str"}
	rewrite := func(value string, ctx *types.NodeContext) string {
		return filepath.Join(ctx.WorkingDir, value)
	}
	resolveShortVolume(node, &types.NodeContext{WorkingDir: tmpdir}, rewrite)
	assert.Equal(t, node.Value, filepath.Join(tmpdir, "data")+":/var/data")
}

func TestResolveShortVolume_LeavesNamedVolumeAlone(t *testing.T) {
	node := &yaml.Node{Kind: yaml.ScalarNode, Value: "mydata:/var/data", Tag: "!!str"}
	rewrite := func(value string, ctx *types.NodeContext) string {
		return filepath.Join(ctx.WorkingDir, value)
	}
	resolveShortVolume(node, &types.NodeContext{WorkingDir: "/wd"}, rewrite)
	assert.Equal(t, node.Value, "mydata:/var/data")
}
