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

// TestOmitEmptyNodes_DropsEmptyDnsList reproduces the legacy OmitEmpty
// behaviour on yaml.Node: an empty `services.X.dns` list must disappear
// from the merged tree.
func TestOmitEmptyNodes_DropsEmptyDnsList(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(`
services:
  app:
    image: alpine
    dns: []
`), 0o644))

	model, err := LoadModel(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, withProjectName("test-omit-empty-dns", true))
	assert.NilError(t, err)

	root := unwrapDocument(model.Merged())
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, dns := override.FindKey(app, "dns")
	assert.Assert(t, dns == nil, "empty dns list should have been dropped")
}

// TestOmitEmptyNodes_KeepsNonEmptyDns checks the inverse case: a
// non-empty dns list stays in place.
func TestOmitEmptyNodes_KeepsNonEmptyDns(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(path, []byte(`
services:
  app:
    image: alpine
    dns:
      - 1.1.1.1
`), 0o644))

	model, err := LoadModel(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, withProjectName("test-omit-empty-dns-keep", true))
	assert.NilError(t, err)

	root := unwrapDocument(model.Merged())
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, dns := override.FindKey(app, "dns")
	assert.Assert(t, dns != nil, "non-empty dns list must be preserved")
	assert.Equal(t, dns.Kind, yaml.SequenceNode)
	assert.Equal(t, len(dns.Content), 1)
}
