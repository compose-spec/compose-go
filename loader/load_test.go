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
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/types"
)

// loadMap runs load and decodes the returned tree into a
// map[string]any so the existing test assertions can keep navigating it
// in dict form.
func loadMap(t *testing.T, cd types.ConfigDetails, opts *Options) (map[string]any, error) {
	t.Helper()
	root, err := load(context.TODO(), &cd, opts)
	if err != nil {
		return nil, err
	}
	var dict map[string]any
	if err := root.Decode(&dict); err != nil {
		return nil, err
	}
	return dict, nil
}

func loadConfig(t *testing.T, dir string, files ...string) types.ConfigDetails {
	t.Helper()
	cfgFiles := make([]types.ConfigFile, len(files))
	for i, name := range files {
		cfgFiles[i] = types.ConfigFile{Filename: filepath.Join(dir, name)}
	}
	return types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: cfgFiles,
		Environment: types.Mapping{},
	}
}

func TestLoad_SingleFileBasic(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
services:
  web:
    image: nginx
`)
	dict, err := loadMap(t, loadConfig(t, dir, "compose.yaml"), &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
	})
	assert.NilError(t, err)
	web := dict["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
}

func TestLoad_MultiFileMergeLeftToRight(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `
services:
  web:
    image: nginx
    restart: always
`)
	writeFile(t, dir, "override.yaml", `
services:
  web:
    image: caddy
`)
	dict, err := loadMap(t, loadConfig(t, dir, "base.yaml", "override.yaml"), &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
	})
	assert.NilError(t, err)
	web := dict["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "caddy", "later file overrides base")
	assert.Equal(t, web["restart"], "always", "base value preserved")
}

func TestLoad_LazyInterpolationAcrossInclude(t *testing.T) {
	// The headline demonstration: an env_file declared on the include
	// block introduces variables that are only visible to scalars from the
	// included file. The parent file keeps the variables of its own shell
	// environment. Same merged tree, two scopes.
	//
	// The semantics match v2's Mapping.Merge: existing keys (from the
	// shell environment) win over env_file entries, so this test relies on
	// API_TAG being defined ONLY in the env_file and WEB_TAG being defined
	// ONLY in the shell environment.
	root := t.TempDir()
	writeFile(t, root, ".env.parent", "API_TAG=2.0\n")
	writeFile(t, root, "included.yaml", `
services:
  api:
    image: caddy:${API_TAG}
`)
	writeFile(t, root, "compose.yaml", `
include:
  - path: included.yaml
    env_file:
      - .env.parent
services:
  web:
    image: nginx:${WEB_TAG}
`)
	cd := loadConfig(t, root, "compose.yaml")
	cd.Environment = types.Mapping{"WEB_TAG": "root-1.0"}
	dict, err := loadMap(t, cd, &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
	})
	assert.NilError(t, err)
	// api inherits API_TAG from the include block env_file.
	api := dict["services"].(map[string]any)["api"].(map[string]any)
	assert.Equal(t, api["image"], "caddy:2.0",
		"included scalar interpolated in include SourceContext")
	// web uses WEB_TAG from the shell environment (parent context).
	web := dict["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx:root-1.0",
		"parent scalar interpolated in parent SourceContext")
}

func TestLoad_ExtendsSameFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
services:
  base:
    image: nginx
    restart: always
  web:
    extends: base
`)
	dict, err := loadMap(t, loadConfig(t, dir, "compose.yaml"), &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
	})
	assert.NilError(t, err)
	web := dict["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
	assert.Equal(t, web["restart"], "always")
	_, hasExtends := web["extends"]
	assert.Assert(t, !hasExtends, "extends key stripped after merge")
}

func TestLoad_ResetTagApplied(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `
services:
  web:
    image: nginx
    command: ["nginx"]
`)
	writeFile(t, dir, "override.yaml", `
services:
  web:
    command: !reset null
`)
	dict, err := loadMap(t, loadConfig(t, dir, "base.yaml", "override.yaml"), &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
	})
	assert.NilError(t, err)
	web := dict["services"].(map[string]any)["web"].(map[string]any)
	_, hasCommand := web["command"]
	assert.Assert(t, !hasCommand, "command stripped by !reset")
	assert.Equal(t, web["image"], "nginx")
}

func TestLoad_PathResolutionPerInclude(t *testing.T) {
	// Different relative paths in parent vs included file must resolve
	// against their own working dirs.
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")
	writeFile(t, subdir, "compose.yaml", `
services:
  api:
    build:
      context: ./local-app
`)
	writeFile(t, root, "compose.yaml", `
include:
  - path: sub/compose.yaml
    project_directory: sub
services:
  web:
    build:
      context: ./root-app
`)
	dict, err := loadMap(t, loadConfig(t, root, "compose.yaml"), &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
		ResolvePaths:         true,
	})
	assert.NilError(t, err)
	web := dict["services"].(map[string]any)["web"].(map[string]any)
	api := dict["services"].(map[string]any)["api"].(map[string]any)
	assert.Equal(t,
		web["build"].(map[string]any)["context"],
		filepath.Join(root, "root-app"),
		"parent scalar resolved against project root")
	assert.Equal(t,
		api["build"].(map[string]any)["context"],
		filepath.Join(subdir, "local-app"),
		"included scalar resolved against include project_directory")
}

func TestLoad_EmptyConfigRejected(t *testing.T) {
	// load rejects an empty input rather than silently producing an
	// empty tree.
	cd := types.ConfigDetails{
		WorkingDir:  "/work",
		Environment: types.Mapping{},
	}
	_, err := load(context.TODO(), &cd, &Options{
		SkipNormalization:    true,
		SkipValidation:       true,
		SkipConsistencyCheck: true,
	})
	assert.ErrorContains(t, err, "empty compose file")
}
