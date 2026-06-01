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

	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/types"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func buildParent(t *testing.T, workingDir string, env types.Mapping, content string) (*node.Layer, *Options) {
	t.Helper()
	parentPath := writeFile(t, workingDir, "compose.yaml", content)
	sc := &node.SourceContext{
		File:        parentPath,
		WorkingDir:  workingDir,
		Environment: env,
	}
	opts := &Options{
		ResourceLoaders: []ResourceLoader{localResourceLoader{WorkingDir: workingDir}},
	}
	layers, err := LoadLayer(context.TODO(), types.ConfigFile{Filename: parentPath}, sc, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(layers), 1)
	return layers[0], opts
}

func TestCollectIncludeLayers_NoBlockYieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	parent, opts := buildParent(t, dir, types.Mapping{}, `
services:
  web:
    image: nginx
`)
	got, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 0)
}

func TestCollectIncludeLayers_ShortFormString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "included.yaml", `
services:
  api:
    image: caddy
`)
	parent, opts := buildParent(t, dir, types.Mapping{}, `
include:
  - included.yaml
services:
  web:
    image: nginx
`)
	got, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 1)

	// The included layer's WorkingDir defaults to the included file's
	// directory.
	assert.Equal(t, got[0].Context.WorkingDir, dir)
	assert.Equal(t, got[0].Context.File, filepath.Join(dir, "included.yaml"))
	// Parent chain is preserved for diagnostics.
	assert.Equal(t, got[0].Context.Parent, parent.Context)

	var m map[string]any
	assert.NilError(t, got[0].Node.Decode(&m))
	assert.Equal(t, m["services"].(map[string]any)["api"].(map[string]any)["image"], "caddy")
}

func TestCollectIncludeLayers_ProjectDirectoryRedefined(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")
	writeFile(t, subdir, "compose.yaml", `
services:
  api:
    image: caddy
`)
	parent, opts := buildParent(t, root, types.Mapping{}, `
include:
  - path: sub/compose.yaml
    project_directory: sub
`)
	got, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Context.WorkingDir, subdir,
		"project_directory under sub/ resolved against parent working dir")
}

func TestCollectIncludeLayers_EnvFileLoaded(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".env.parent", "TAG=2.0\n")
	writeFile(t, root, "included.yaml", `
services:
  api:
    image: caddy:${TAG}
`)
	parent, opts := buildParent(t, root, types.Mapping{}, `
include:
  - path: included.yaml
    env_file:
      - .env.parent
`)
	got, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Context.Environment["TAG"], "2.0",
		"env_file scoped to the include is merged into the child SourceContext")
	// The included layer's image scalar still carries ${TAG} — interpolation
	// will fire later on the merged tree, using this SourceContext.
	var m map[string]any
	assert.NilError(t, got[0].Node.Decode(&m))
	assert.Equal(t, m["services"].(map[string]any)["api"].(map[string]any)["image"], "caddy:${TAG}",
		"included layer is not eagerly interpolated; substitution defers to the merge phase")
}

func TestCollectIncludeLayers_InterpolatesIncludeBlockPaths(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "included.yaml", `
services:
  api:
    image: caddy
`)
	env := types.Mapping{"FILE": "included.yaml"}
	parent, opts := buildParent(t, root, env, `
include:
  - ${FILE}
`)
	got, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].Context.File, filepath.Join(root, "included.yaml"),
		"include path scalar interpolated in parent context before loading")
}

func TestCollectIncludeLayers_RejectsNonListIncludeBlock(t *testing.T) {
	dir := t.TempDir()
	parent, opts := buildParent(t, dir, types.Mapping{}, `
include: included.yaml
`)
	_, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.ErrorContains(t, err, "`include` must be a list")
}

func TestCollectIncludeLayers_DevNullDisablesEnvInheritance(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".env.parent", "TAG=2.0\n")
	writeFile(t, root, "included.yaml", `services: {api: {image: caddy}}`)
	parent, opts := buildParent(t, root, types.Mapping{"TAG": "1.0"}, `
include:
  - path: included.yaml
    env_file:
      - /dev/null
`)
	got, err := CollectIncludeLayers(context.TODO(), parent, opts)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 1)
	// No env_file actually loaded; child env is just the parent env clone.
	assert.Equal(t, got[0].Context.Environment["TAG"], "1.0")
}
