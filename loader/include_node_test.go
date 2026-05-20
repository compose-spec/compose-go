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

func TestApplyIncludeNodes_GraftsLayerWithOwnContext(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))

	subYaml := `
services:
  included:
    image: alpine
    build:
      context: ./local
`
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte(subYaml), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
services:
  app:
    image: nginx
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyIncludeNodes(context.TODO()))

	assert.Equal(t, len(m.layers), 2, "expected included layer + parent layer")
	// Included layer comes first (so parent overrides it during merge).
	assert.Equal(t, m.layers[0].Context.Source, filepath.Join(subdir, "compose.yaml"))
	assert.Equal(t, m.layers[0].Context.WorkingDir, subdir)
	assert.Equal(t, m.layers[1].Context.Source, topPath)

	// The included tree must NOT be mutated: build.context still says "./local".
	root := unwrapDocument(m.layers[0].Root)
	_, services := override.FindKey(root, "services")
	_, included := override.FindKey(services, "included")
	_, build := override.FindKey(included, "build")
	_, buildCtx := override.FindKey(build, "context")
	assert.Equal(t, buildCtx.Value, "./local")

	// And its NodeContext points at the included file's directory.
	ctx, ok := m.contexts[buildCtx]
	assert.Assert(t, ok)
	assert.Equal(t, ctx.WorkingDir, subdir)

	// The parent layer no longer carries the include directive.
	parentRoot := unwrapDocument(m.layers[1].Root)
	_, includeNode := override.FindKey(parentRoot, "include")
	assert.Assert(t, includeNode == nil)
}

func TestApplyIncludeNodes_EnvFile_AddsVariablesToContext(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))

	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "local.env"), []byte("EXTRA=value\n"), 0o644))
	subYaml := "services:\n  included:\n    image: alpine\n"
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte(subYaml), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
    env_file: sub/local.env
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: types.Mapping{},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyIncludeNodes(context.TODO()))

	includedCtx := m.layers[0].Context
	assert.Equal(t, includedCtx.Env["EXTRA"], "value")
}

func TestApplyIncludeNodes_ProjectDirectoryOverridesWorkingDir(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	otherdir := filepath.Join(tmpdir, "other")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))
	assert.NilError(t, os.MkdirAll(otherdir, 0o755))

	subYaml := "services:\n  included:\n    image: alpine\n"
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte(subYaml), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
    project_directory: other
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyIncludeNodes(context.TODO()))

	assert.Equal(t, m.layers[0].Context.WorkingDir, otherdir)
}

func TestApplyIncludeNodes_CycleDetected(t *testing.T) {
	tmpdir := t.TempDir()
	aPath := filepath.Join(tmpdir, "a.yaml")
	bPath := filepath.Join(tmpdir, "b.yaml")
	assert.NilError(t, os.WriteFile(aPath, []byte("include:\n  - b.yaml\n"), 0o644))
	assert.NilError(t, os.WriteFile(bPath, []byte("include:\n  - a.yaml\n"), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: aPath}},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	err := m.applyIncludeNodes(context.TODO())
	assert.ErrorContains(t, err, "include cycle detected")
}

func TestApplyIncludeNodes_NestedIncludeInheritsParentContext(t *testing.T) {
	tmpdir := t.TempDir()
	dirA := filepath.Join(tmpdir, "a")
	dirB := filepath.Join(tmpdir, "b")
	assert.NilError(t, os.MkdirAll(dirA, 0o755))
	assert.NilError(t, os.MkdirAll(dirB, 0o755))

	// a.env brings VAR_A; b.env brings VAR_B. b is included from a, so b
	// should see both.
	assert.NilError(t, os.WriteFile(filepath.Join(dirA, "a.env"), []byte("VAR_A=1\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(dirB, "b.env"), []byte("VAR_B=2\n"), 0o644))

	aYaml := `
include:
  - path: ../b/compose.yaml
    env_file: ../b/b.env
services:
  serviceA:
    image: alpine
`
	bYaml := "services:\n  serviceB:\n    image: alpine\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dirA, "compose.yaml"), []byte(aYaml), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(dirB, "compose.yaml"), []byte(bYaml), 0o644))

	topYaml := `
include:
  - path: a/compose.yaml
    env_file: a/a.env
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	m := newComposeModel(types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: types.Mapping{},
	}, &Options{SkipExtends: true})
	assert.NilError(t, m.parseLayers(m.configDetails))
	assert.NilError(t, m.applyIncludeNodes(context.TODO()))

	// 3 layers expected: b (deepest), a, top
	assert.Equal(t, len(m.layers), 3)
	// The deepest included layer (b) must see VAR_A (from a's include) AND
	// VAR_B (from its own include directive in a).
	bCtx := m.layers[0].Context
	assert.Equal(t, bCtx.Env["VAR_A"], "1", "VAR_A should be inherited from a's include.env_file")
	assert.Equal(t, bCtx.Env["VAR_B"], "2", "VAR_B should come from b's include.env_file")
}
