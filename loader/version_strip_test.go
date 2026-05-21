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
	"gotest.tools/v3/assert"
)

// TestVersionStrip_OnYamlNode verifies that the obsolete `version` key is
// removed by the yaml.Node pipeline (Phase B.5) so the legacy
// postMergeLegacy version handling is no longer required.
func TestVersionStrip_OnYamlNode(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(`
version: "3.8"
name: test-version-strip
services:
  app:
    image: alpine
`), 0o644))

	model, err := LoadModel(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, withProjectName("test-version-strip", true))
	assert.NilError(t, err)

	_, version := override.FindKey(unwrapDocument(model.Merged()), "version")
	assert.Assert(t, version == nil, "version key must be stripped from the merged tree")
}
