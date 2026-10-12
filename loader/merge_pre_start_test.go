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
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func loadFiles(t *testing.T, files ...string) (*types.Project, error) {
	t.Helper()
	var configs []types.ConfigFile
	for i, f := range files {
		configs = append(configs, types.ConfigFile{Filename: []string{"compose.yaml", "override.yaml", "override2.yaml"}[i], Content: []byte(f)})
	}
	return LoadWithContext(context.Background(), types.ConfigDetails{ConfigFiles: configs},
		func(o *Options) { o.SetProjectName("test", true) })
}

const preStartBase = `
services:
  app:
    image: app:1
    pre_start:
      - image: init:1
        command: [migrate, --up]
        environment:
          A: a
      - command: [seed]
`

// publish use case: pin the image of an existing hook without repeating (nor
// freezing) the rest of its definition.
func TestPreStartMerge_PinHookImage(t *testing.T) {
	p, err := loadFiles(t, preStartBase, `
services:
  app:
    image: app@sha256:aaaa
    pre_start: !merge
      - image: init@sha256:bbbb
      - {}
`)
	assert.NilError(t, err)
	hooks := p.Services["app"].PreStart
	assert.Equal(t, len(hooks), 2)
	assert.Equal(t, hooks[0].Image, "init@sha256:bbbb")
	assert.DeepEqual(t, []string(hooks[0].Command), []string{"migrate", "--up"})
	// second hook has no image of its own: inherits the (pinned) service image
	assert.Equal(t, hooks[1].Image, "app@sha256:aaaa")
	assert.DeepEqual(t, []string(hooks[1].Command), []string{"seed"})
}

// command replaces, environment merges, extra override entries are appended
func TestPreStartMerge_FieldRules(t *testing.T) {
	p, err := loadFiles(t, preStartBase, `
services:
  app:
    pre_start: !merge
      - command: [migrate, --down]
        environment:
          B: b
      - null
      - command: [extra]
`)
	assert.NilError(t, err)
	hooks := p.Services["app"].PreStart
	assert.Equal(t, len(hooks), 3)
	assert.DeepEqual(t, []string(hooks[0].Command), []string{"migrate", "--down"})
	assert.Equal(t, *hooks[0].Environment["A"], "a")
	assert.Equal(t, *hooks[0].Environment["B"], "b")
	assert.DeepEqual(t, []string(hooks[1].Command), []string{"seed"})
	assert.DeepEqual(t, []string(hooks[2].Command), []string{"extra"})
}

// without the tag, hooks keep being appended
func TestPreStartMerge_DefaultStillAppends(t *testing.T) {
	p, err := loadFiles(t, preStartBase, `
services:
  app:
    pre_start:
      - command: [extra]
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["app"].PreStart), 3)
}

// the tag is rejected anywhere it is not supported, never silently ignored
func TestPreStartMerge_RejectedElsewhere(t *testing.T) {
	_, err := loadFiles(t, `
services:
  app:
    image: app:1
    dns: [1.1.1.1]
`, `
services:
  app:
    dns: !merge
      - 9.9.9.9
`)
	assert.ErrorContains(t, err, "!merge is only supported")
}

// the tag is reported on anchored content too (alias reuse)
func TestPreStartMerge_Anchor(t *testing.T) {
	p, err := loadFiles(t, `
services:
  a:
    image: a:1
    pre_start: [{image: init:1}]
  b:
    image: b:1
    pre_start: [{image: init:1}]
`, `
services:
  a:
    pre_start: &hooks !merge
      - image: init@sha256:cccc
  b:
    pre_start: *hooks
`)
	assert.NilError(t, err)
	for _, name := range []string{"a", "b"} {
		hooks := p.Services[name].PreStart
		assert.Equal(t, len(hooks), 1, name)
		assert.Equal(t, hooks[0].Image, "init@sha256:cccc", name)
	}
}

// `!merge` combined with `extends` on the same service is not supported: the
// positional merge would apply to the extended base rather than to previous files.
func TestPreStartMerge_Extends(t *testing.T) {
	_, err := loadFiles(t, `
services:
  base:
    image: app:1
  app:
    image: app:1
    pre_start: [{command: [z]}]
`, `
services:
  base:
    image: app:1
  app:
    extends: base
    pre_start: !merge
      - command: [b]
`)
	assert.ErrorContains(t, err, "!merge cannot be combined with extends")
}

// `!merge` on a service also declared by an included file: fail loudly rather
// than silently appending the hooks.
func TestPreStartMerge_Include(t *testing.T) {
	tmpdir := t.TempDir()
	createFileSubDir(t, tmpdir, "module", `
services:
  app:
    image: app:1
    pre_start: [{image: init:1}]
`, "compose.yml")
	path := createFile(t, tmpdir, `
include:
  - ./module/compose.yml
services:
  app:
    pre_start: !merge
      - image: init@sha256:dddd
`, "compose.yml")
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, func(o *Options) { o.SetProjectName("test", true) })
	assert.ErrorContains(t, err, "!merge cannot be applied to services.app")
}

// the service name may contain dots, and a tagged anchor may be shared by
// several services: each one is merged with its own base
func TestPreStartMerge_DottedNameAndSharedAnchor(t *testing.T) {
	p, err := loadFiles(t, `
services:
  a.b:
    image: app:1
    pre_start: [{image: init:1}]
  c:
    image: app:1
    pre_start: [{image: init:1}, {command: [x]}]
`, `
services:
  a.b:
    pre_start: &p !merge
      - image: init@sha256:1
  c:
    pre_start: *p
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["a.b"].PreStart), 1)
	assert.Equal(t, p.Services["a.b"].PreStart[0].Image, "init@sha256:1")
	c := p.Services["c"].PreStart
	assert.Equal(t, len(c), 2)
	assert.Equal(t, c[0].Image, "init@sha256:1")
	assert.DeepEqual(t, []string(c[1].Command), []string{"x"})
}

// an empty tagged sequence leaves the hooks untouched
func TestPreStartMerge_EmptySequence(t *testing.T) {
	p, err := loadFiles(t, preStartBase, `
services:
  app:
    pre_start: !merge []
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["app"].PreStart), 2)
}

// nothing to merge with: the sequence is simply taken as declared
func TestPreStartMerge_NoBase(t *testing.T) {
	p, err := loadFiles(t, preStartBase, `
services:
  other:
    image: other:1
    pre_start: !merge
      - image: init@sha256:eeee
`)
	assert.NilError(t, err)
	assert.Equal(t, p.Services["other"].PreStart[0].Image, "init@sha256:eeee")
}

// a tag that is not on a sequence is rejected, not ignored
func TestPreStartMerge_NotASequence(t *testing.T) {
	_, err := loadFiles(t, preStartBase, `
services:
  app:
    pre_start: !merge
`)
	assert.ErrorContains(t, err, "!merge is only supported")
}
