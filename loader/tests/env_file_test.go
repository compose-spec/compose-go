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

func TestEnvFileString(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    env_file:
      - path: .env
        required: false
`)
	expect := func(p *types.Project) {
		assert.Equal(t, len(p.Services["foo"].EnvFiles), 1)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestEnvFileList(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    env_file:
      - path: .env
        required: false
      - path: .env.local
        required: false
`)
	expect := func(p *types.Project) {
		assert.Equal(t, len(p.Services["foo"].EnvFiles), 2)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestEnvFileLongSyntax(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    env_file:
      - path: .env
        required: false
        format: raw
`)
	expect := func(p *types.Project) {
		assert.Equal(t, len(p.Services["foo"].EnvFiles), 1)
		assert.Equal(t, p.Services["foo"].EnvFiles[0].Format, "raw")
		assert.Equal(t, bool(p.Services["foo"].EnvFiles[0].Required), false)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
