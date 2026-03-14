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
	"testing"

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
`)
	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Extensions["x-bar"], "baz")
		assert.Equal(t, p.Services["foo"].Extensions["x-foo"], "bar")
	}
	expect(p)

	yamlP, _ := roundTrip(t, p)
	expect(yamlP)
}
