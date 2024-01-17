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

func TestOverrideNetworks(t *testing.T) {
	yaml := `
name: test-override-networks
services:
  test:
    image: test
    networks:
      - test_network

networks:
  test_network: {}
`

	override := `
services:
  test:
    image: test
    networks:
      test_network: 
        aliases:
          - alias1
          - alias2
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "base",
				Content:  []byte(yaml),
			},
			{
				Filename: "override",
				Content:  []byte(override),
			},
		},
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test"].Networks["test_network"].Aliases, []string{"alias1", "alias2"})
}

func TestOverrideBuildContext(t *testing.T) {
	yaml := `
name: test-override-networks
services:
  test:
    build: .
`

	override := `
services:
  test:
    build:
      context: src
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "base",
				Content:  []byte(yaml),
			},
			{
				Filename: "override",
				Content:  []byte(override),
			},
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["test"].Build.Context, "src")
}

func TestOverrideDepends_on(t *testing.T) {
	yaml := `
name: test-override-networks
services:
  test:
    image: test
    depends_on:
      - foo
  foo:
    image: foo
`

	override := `
services:
  test:
    depends_on:
      foo:
        condition: service_healthy
        required: false
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "base",
				Content:  []byte(yaml),
			},
			{
				Filename: "override",
				Content:  []byte(override),
			},
		},
	})
	assert.NilError(t, err)
	assert.Check(t, p.Services["test"].DependsOn["foo"].Required == false)
}

func TestOverridePartial(t *testing.T) {
	yaml := `
name: test-override-networks
services:
  test:
    image: test
    depends_on:
      foo:
        condition: service_healthy

  foo: 
    image: foo
`

	override := `
services:
  test:
    depends_on:
      foo:
        # This is invalid according to json schema as condition is required
        required: false
`
	_, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "base",
				Content:  []byte(yaml),
			},
			{
				Filename: "override",
				Content:  []byte(override),
			},
		},
	})
	assert.NilError(t, err)
}
