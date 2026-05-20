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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
	"gotest.tools/v3/assert"
)

func TestDiagnostic_DuplicateKeysReportsSourceLine(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	// line 2 col 5 declares 'image' for the first time, line 3 col 5
	// declares it again — the error must point at line 3.
	yaml := "services:\n  app:\n    image: a\n    image: b\n"
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	err := m.parseLayers(m.configDetails)
	assert.Assert(t, err != nil)
	expected := fmt.Sprintf("%s:4", path)
	assert.Assert(t, strings.Contains(err.Error(), expected),
		"error %q should mention source position %q", err.Error(), expected)
}

func TestDiagnostic_NonStringKeyReportsSourceLine(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	yaml := "services:\n  app:\n    environment:\n      42: value\n"
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	node, err := loadYamlFileNode(types.ConfigFile{Filename: path})
	assert.NilError(t, err)
	err = checkNonStringKeys(path, node, "")
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), path+":"),
		"error %q should be anchored at %q", err.Error(), path)
}

func TestDiagnostic_ExtendsMissingServiceReportsLocation(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	yaml := `
services:
  app:
    extends:
      service: missing
`
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	err := m.applyExtendsNode(context.TODO(), m.layers[0])
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), path+":"),
		"error %q should be anchored at %q", err.Error(), path)
	assert.Assert(t, strings.Contains(err.Error(), "missing"),
		"error %q should mention the missing service", err.Error())
}

func TestDiagnostic_IncludeMissingFileReportsLocation(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	yaml := `
include:
  - path: nonexistent.yaml
`
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	err := m.applyIncludeNodes(context.TODO())
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), path+":"),
		"error %q should be anchored at %q", err.Error(), path)
}
