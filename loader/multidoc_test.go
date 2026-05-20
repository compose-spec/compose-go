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

// TestLoadMultiDocumentYaml reproduces the docker/compose TestPublish OCI
// scenario: when `compose publish` packs several compose files into one OCI
// layer, the resulting compose.yaml is a multi-document YAML stream (each
// original file becomes a `---`-separated document). The loader must treat
// every document as a separate layer so they merge in declaration order,
// not silently keep only the last one.
func TestLoadMultiDocumentYaml(t *testing.T) {
	tmpdir := t.TempDir()
	extendsHash := "abcdef.yaml"
	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, extendsHash), []byte(`services:
  base:
    image: alpine
`), 0o644))

	// First document: extends a remote-resolved file. Second document:
	// adds an env_file. This is exactly what `compose publish` packs into
	// the OCI manifest for a project loaded from two compose files.
	multidoc := `services:
  app:
    extends:
      file: abcdef.yaml
      service: base
---
services:
  app:
    environment:
      HELLO: WORLD
`
	composePath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(composePath, []byte(multidoc), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: composePath}},
	}, withProjectName("test-multidoc", true))
	assert.NilError(t, err)

	app := p.Services["app"]
	// First document brings extends -> inherits image:alpine from base
	assert.Equal(t, app.Image, "alpine")
	// Second document brings environment HELLO=WORLD
	v, ok := app.Environment["HELLO"]
	assert.Assert(t, ok, "HELLO must be set on app")
	assert.Assert(t, v != nil)
	assert.Equal(t, *v, "WORLD")
}

// TestLoadMultiDocumentYaml_TrailingSeparator ensures a trailing `---`
// (which yields an empty document) is silently skipped.
func TestLoadMultiDocumentYaml_TrailingSeparator(t *testing.T) {
	tmpdir := t.TempDir()
	composePath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(composePath, []byte(`services:
  app:
    image: nginx
---
`), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: composePath}},
	}, withProjectName("test-multidoc-trailing", true))
	assert.NilError(t, err)
	assert.Equal(t, p.Services["app"].Image, "nginx")
}
