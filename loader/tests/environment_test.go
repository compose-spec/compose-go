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

import (
	"context"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestEnvironmentMap(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
    environment:
      FOO: "1"
      BAR: 2
      GA: 2.5
      BU: ""
      ZO:
      MEU:
`, map[string]string{"MEU": "Shadoks"})

	expect := func(p *types.Project) {
		env := p.Services["foo"].Environment
		assert.Equal(t, *env["FOO"], "1")
		assert.Equal(t, *env["BAR"], "2")
		assert.Equal(t, *env["GA"], "2.5")
		assert.Equal(t, *env["BU"], "")
		assert.Equal(t, *env["MEU"], "Shadoks")
		assert.Assert(t, env["ZO"] == nil)
	}
	expect(p)
}

func TestEnvironmentList(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
    environment:
      - FOO=1
      - BAR=2
      - BU=
      - ZO
      - MEU
`, map[string]string{"MEU": "Shadoks"})

	expect := func(p *types.Project) {
		env := p.Services["foo"].Environment
		assert.Equal(t, *env["FOO"], "1")
		assert.Equal(t, *env["BAR"], "2")
		assert.Equal(t, *env["BU"], "")
		assert.Equal(t, *env["MEU"], "Shadoks")
		assert.Assert(t, env["ZO"] == nil)
	}
	expect(p)
}

func TestEnvironmentBoolean(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    environment:
      FOO: true
      BAR: false
`)
	assert.Equal(t, *p.Services["foo"].Environment["FOO"], "true")
	assert.Equal(t, *p.Services["foo"].Environment["BAR"], "false")
}

func TestEnvironmentInvalidValue(t *testing.T) {
	_, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    environment:
      FOO: ["1"]
`)}},
		Environment: map[string]string{},
	})
	assert.ErrorContains(t, err, "services.foo.environment.FOO must be a boolean, null, number or string")
}

func TestEnvironmentInvalidObject(t *testing.T) {
	_, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    environment: "FOO=1"
`)}},
		Environment: map[string]string{},
	})
	assert.ErrorContains(t, err, "services.foo.environment must be a mapping")
}

func TestEnvironmentWhitespace(t *testing.T) {
	_, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  test:
    environment:
      - DEBUG = true
`)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
	})
	assert.Check(t, strings.Contains(err.Error(), "environment variable DEBUG  is declared with a trailing space"), err.Error())
}
