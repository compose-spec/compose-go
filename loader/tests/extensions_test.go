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

package tests

// The tests in this file lock the `extensions (x-* attributes)` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/11-extension.md
//
// Spec: "As with Fragments, Extensions can be used to make your Compose file
// more efficient and easier to maintain."

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestTopLevelExtensions(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
x-foo: bar
x-bar: baz
x-nested:
  foo: bar
  bar: baz
`)
	expect := func(p *types.Project) {
		assert.Equal(t, p.Extensions["x-foo"], "bar")
		assert.Equal(t, p.Extensions["x-bar"], "baz")
		nested := p.Extensions["x-nested"].(map[string]any)
		assert.Equal(t, nested["foo"], "bar")
		assert.Equal(t, nested["bar"], "baz")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestServiceExtensions(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    x-bar: baz
    x-foo: bar
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    x-bar: baz
    x-foo: bar
`)
	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Extensions["x-bar"], "baz")
		assert.Equal(t, p.Services["foo"].Extensions["x-foo"], "bar")
		assert.Equal(t, p.Jobs["foo"].Extensions["x-bar"], "baz")
		assert.Equal(t, p.Jobs["foo"].Extensions["x-foo"], "bar")
	}
	expect(p)

	yamlP, _ := roundTrip(t, p)
	expect(yamlP)
}

// An x-* extension that the specification later adopted as an attribute is
// promoted to that attribute when the latter is absent, so that files written
// before the adoption keep working.

func TestExtensionAliasIsPromoted(t *testing.T) {
	t.Run("service attribute and nested attribute", func(t *testing.T) {
		loadsAs(t, `
name: test
services:
  web:
    image: app
    x-develop:
      watch:
        - path: src
          action: sync
          target: /app
          x-initialSync: true
`, `
name: test
services:
  web:
    image: app
    develop:
      watch:
        - path: src
          action: sync
          target: /app
          initial_sync: true
`)
	})

	t.Run("official attribute wins and alias stays an extension", func(t *testing.T) {
		loadsAs(t, `
name: test
services:
  web:
    image: app
    develop:
      watch:
        - path: src
          action: sync
          target: /app
          initial_sync: true
          x-initialSync: false
`, `
name: test
services:
  web:
    image: app
    develop:
      watch:
        - path: src
          action: sync
          target: /app
          initial_sync: true
          x-initialSync: false
`)
	})

	t.Run("alias is only promoted where the attribute exists", func(t *testing.T) {
		loadsAs(t, `
name: test
services:
  web:
    image: app
    x-initialSync: true
x-develop:
  watch: []
`, `
name: test
services:
  web:
    image: app
    x-initialSync: true
x-develop:
  watch: []
`)
	})

	t.Run("promoted value is validated as the attribute", func(t *testing.T) {
		err := loadErr(t, `
name: test
services:
  web:
    image: app
    x-develop: not-an-object
`)
		assert.ErrorContains(t, err, "services.web.develop")
	})
}

// Extensions are promoted before include and extends read the file, and
// sharing them through YAML anchors across services does not link the services.
func TestExtensionAliasWithReuse(t *testing.T) {
	watch := `
      watch:
        - path: src
          action: sync
          target: /app
`
	t.Run("extends", func(t *testing.T) {
		loadsAs(t, `
name: test
services:
  base:
    image: app
    x-develop:`+watch+`
  web:
    extends:
      service: base
`, `
name: test
services:
  base:
    image: app
    develop:`+watch+`
  web:
    image: app
    develop:`+watch)
	})

	t.Run("include", func(t *testing.T) {
		dir := t.TempDir()
		assert.NilError(t, os.WriteFile(filepath.Join(dir, "included.yml"), []byte(`
services:
  base:
    image: app
    x-develop:`+watch), 0o600))
		p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
			WorkingDir: dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yml"), Content: []byte(`
name: test
include:
  - included.yml
`)}},
			Environment: map[string]string{},
		}, func(options *loader.Options) {
			options.SkipConsistencyCheck = true
			options.SkipNormalization = true
		})
		assert.NilError(t, err)
		assert.Assert(t, p.Services["base"].Develop != nil)
		assert.Equal(t, len(p.Services["base"].Develop.Watch), 1)
	})

	t.Run("YAML anchor shared by services", func(t *testing.T) {
		loadsAs(t, `
name: test
x-shared: &shared
  x-develop:`+watch+`
services:
  a:
    <<: *shared
    image: app
  b:
    <<: *shared
    image: app
`, `
name: test
x-shared:
  x-develop:`+watch+`
services:
  a:
    image: app
    develop:`+watch+`
  b:
    image: app
    develop:`+watch)
	})

	t.Run("attribute set to null does not hide the extension", func(t *testing.T) {
		loadsAs(t, `
name: test
services:
  web:
    image: app
    develop:
    x-develop:`+watch, `
name: test
services:
  web:
    image: app
    develop:`+watch)
	})
}

// Each file of a multi-file project is promoted on its own, so that an
// override written with the extension still overrides the base file.
func TestExtensionAliasIsPromotedPerFile(t *testing.T) {
	loadFilesAs(t, []string{`
name: test
services:
  web:
    image: app
    develop:
      watch:
        - path: src
          action: sync
          target: /app
`, `
services:
  web:
    x-develop:
      watch:
        - path: other
          action: rebuild
`}, `
name: test
services:
  web:
    image: app
    develop:
      watch:
        - path: src
          action: sync
          target: /app
        - path: other
          action: rebuild
`)
}
