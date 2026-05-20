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

	"github.com/compose-spec/compose-go/v3/types"
	"gotest.tools/v3/assert"
)

// Reference tests for the v3 refactoring (see plan.md).
//
// These tests are written first and skipped until the corresponding phase of
// the refactoring closes the underlying gap. They are the discriminant gates
// of the refactoring.

// TestInclude_EnvFile_ProvidesContextToServiceEnvFile asserts that variables
// supplied by include.env_file are available to interpolate the path of an
// env_file declared inside the included service.
//
// Today this fails: WithServicesEnvironmentResolved cannot reach the include's
// env (limitation 3 in plan.md). Will turn green at the end of Phase 7.
func TestInclude_EnvFile_ProvidesContextToServiceEnvFile(t *testing.T) {
	t.Skip("reference test for refactoring plan.md — turns green at Phase 7")

	tmpdir := t.TempDir()
	subDir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subDir, 0o755))

	// sub/local.env supplies EXTRA_PATH used to compute the service env_file path
	assert.NilError(t, os.WriteFile(filepath.Join(subDir, "local.env"), []byte("EXTRA_PATH=extra.env\n"), 0o644))

	// sub/extra.env contains the actual variable to be picked up by the service
	assert.NilError(t, os.WriteFile(filepath.Join(subDir, "extra.env"), []byte("FOO=bar\n"), 0o644))

	// sub/compose.yaml declares the service whose env_file uses ${EXTRA_PATH}
	subCompose := `
services:
  app:
    image: alpine
    env_file: ${EXTRA_PATH}
`
	assert.NilError(t, os.WriteFile(filepath.Join(subDir, "compose.yaml"), []byte(subCompose), 0o644))

	// top-level compose.yaml includes sub/compose.yaml and supplies sub/local.env
	topCompose := `
include:
  - path: sub/compose.yaml
    env_file: sub/local.env
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topCompose), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: map[string]string{},
	}, withProjectName("test-include-envfile-context", true))
	assert.NilError(t, err)

	resolved, err := p.WithServicesEnvironmentResolved(false)
	assert.NilError(t, err)

	app := resolved.Services["app"]
	foo, ok := app.Environment["FOO"]
	assert.Check(t, ok, "FOO should be present in resolved environment")
	if ok {
		assert.Check(t, foo != nil)
		assert.Equal(t, *foo, "bar")
	}
}

// TestLoad_DoesNotReadEnvFiles asserts that LoadWithContext does not read any
// env_file from disk. A service may reference a non-existent env_file
// (Required: false) without LoadWithContext failing.
//
// Today this fails: env_file resolution is part of the standard load pipeline.
// Will turn green at the end of Phase 7 once env_file resolution becomes lazy.
func TestLoad_DoesNotReadEnvFiles(t *testing.T) {
	t.Skip("reference test for refactoring plan.md — turns green at Phase 7")

	tmpdir := t.TempDir()
	yaml := `
services:
  app:
    image: alpine
    env_file:
      - path: ./does-not-exist.env
        required: false
`
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
		Environment: map[string]string{},
	}, withProjectName("test-load-no-envfile-read", true))
	assert.NilError(t, err)

	app := p.Services["app"]
	assert.Equal(t, len(app.EnvFiles), 1)
	assert.Equal(t, app.EnvFiles[0].Required.IsTrue(), false)
}
