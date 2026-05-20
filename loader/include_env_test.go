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

// TestInclude_EnvFile_InterpolatesIncludedSecret reproduces the
// docker/compose TestSecretFromInclude regression: a secret declared
// inside an included compose file references a variable that is only
// available through include.env_file. The variable must be substituted
// using the included layer's environment before merge, not the main
// project's.
func TestInclude_EnvFile_InterpolatesIncludedSecret(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))

	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "local.env"), []byte("SECRET_FILE=mysecret.txt\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "mysecret.txt"), []byte("s3cr3t"), 0o644))

	subYaml := `
services:
  app:
    image: alpine
    secrets:
      - sample
secrets:
  sample:
    file: ${SECRET_FILE}
`
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte(subYaml), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
    env_file: sub/local.env
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: map[string]string{},
	}, withProjectName("test-include-secret-interpolation", true))
	assert.NilError(t, err)

	secret, ok := p.Secrets["sample"]
	assert.Assert(t, ok, "secret 'sample' missing from project")
	// File path should have been interpolated using SECRET_FILE from the
	// include's env_file and resolved against the included file's dir.
	assert.Equal(t, secret.File, filepath.Join(subdir, "mysecret.txt"))
}

// TestInclude_EnvFile_ResolvesEnvironmentBackedSecret reproduces the exact
// docker/compose TestSecretFromInclude scenario: an included file declares
// a secret using `environment: VAR` where VAR is only provided by
// include.env_file. The loader must materialise the secret's value in the
// project so the runtime can mount it without re-reading the parent env.
func TestInclude_EnvFile_ResolvesEnvironmentBackedSecret(t *testing.T) {
	tmpdir := t.TempDir()
	childdir := filepath.Join(tmpdir, "child")
	assert.NilError(t, os.MkdirAll(childdir, 0o755))

	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "secret.env"), []byte("MY_SECRET=this-is-secret\n"), 0o644))

	childYaml := `
services:
  included:
    image: alpine
    secrets:
      - my-secret
    command: cat /run/secrets/my-secret
secrets:
  my-secret:
    environment: MY_SECRET
`
	assert.NilError(t, os.WriteFile(filepath.Join(childdir, "compose.yaml"), []byte(childYaml), 0o644))

	topYaml := `
include:
  - path: child/compose.yaml
    env_file:
      - secret.env
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: map[string]string{},
	}, withProjectName("test-include-env-secret", true))
	assert.NilError(t, err)

	secret, ok := p.Secrets["my-secret"]
	assert.Assert(t, ok, "secret 'my-secret' missing from project")
	assert.Equal(t, secret.Environment, "MY_SECRET")
	// The loader must have captured MY_SECRET=this-is-secret from the
	// include.env_file so the runtime can mount the secret.
	assert.Equal(t, secret.Content, "this-is-secret")
}
