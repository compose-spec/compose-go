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

func TestApplyExtendsNode_SameFile(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	yaml := `
services:
  base:
    image: alpine
    command: echo base
  child:
    extends: base
    command: echo child
`
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))

	assert.NilError(t, m.applyExtendsNode(context.TODO(), m.layers[0]))

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, child := override.FindKey(services, "child")
	assert.Assert(t, child != nil)
	_, image := override.FindKey(child, "image")
	assert.Assert(t, image != nil)
	assert.Equal(t, image.Value, "alpine")
	_, command := override.FindKey(child, "command")
	assert.Assert(t, command != nil)
	assert.Equal(t, command.Value, "echo child")
	_, extends := override.FindKey(child, "extends")
	assert.Assert(t, extends == nil, "extends key must be removed after resolution")
}

func TestApplyExtendsNode_ExternalFile_PreservesContextOfInheritedFields(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))

	baseYaml := `
services:
  base:
    build:
      context: ./local
`
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "base.yaml"), []byte(baseYaml), 0o644))

	mainYaml := `
services:
  app:
    extends:
      file: ./sub/base.yaml
      service: base
    command: echo
`
	mainPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(mainPath, []byte(mainYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: mainPath}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyExtendsNode(context.TODO(), m.layers[0]))

	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, build := override.FindKey(app, "build")
	_, buildCtx := override.FindKey(build, "context")
	assert.Assert(t, buildCtx != nil)
	// The path is NOT rewritten here — it remains as declared in base.yaml.
	assert.Equal(t, buildCtx.Value, "./local")
	// However, its NodeContext points to the extends file's directory, so
	// the path resolution pass will be able to resolve it correctly.
	ctx, ok := m.contexts[buildCtx]
	assert.Assert(t, ok)
	assert.Equal(t, ctx.WorkingDir, subdir)
}

func TestApplyExtendsNode_CycleDetected(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "compose.yaml")
	yaml := `
services:
  a:
    extends: b
  b:
    extends: a
`
	assert.NilError(t, os.WriteFile(path, []byte(yaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, &Options{})
	assert.NilError(t, m.parseLayers(m.configDetails))
	err := m.applyExtendsNode(context.TODO(), m.layers[0])
	assert.ErrorContains(t, err, "circular reference")
}
