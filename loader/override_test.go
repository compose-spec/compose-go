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

func TestOverrideVolume(t *testing.T) {
	yaml := `
name: test-override-volume
services:
  test:
    image: test
    volumes:
      - ./src:/src
`

	override := `
services:
  test:
    volumes:
      - ./src:/src
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
	assert.Equal(t, len(p.Services["test"].Volumes), 1)
}

// see https://github.com/docker/compose/issues/13298
func TestOverrideMiddle(t *testing.T) {
	pwd := t.TempDir()
	base := filepath.Join(pwd, "base.yaml")
	err := os.WriteFile(base, []byte(`
services:
  base:
    volumes:
      - /foo:/foo
    networks:
      - foo
`), 0o700)
	assert.NilError(t, err)

	override := filepath.Join(pwd, "override.yaml")
	err = os.WriteFile(override, []byte(`
services:
  override:
    extends:
      file: ./base.yaml
      service: base
    volumes: !override
      -  /bar:/bar
    networks: !override
      - bar
`), 0o700)
	assert.NilError(t, err)

	compose := filepath.Join(pwd, "compose.yaml")
	err = os.WriteFile(compose, []byte(`
name: test
services:
  test:
    image: test
    extends:
      file: ./override.yaml
      service: override
    volumes:
      - /zot:/zot
    networks: !override
      - zot

networks:
  zot: {}
`), 0o700)
	assert.NilError(t, err)

	project, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: pwd,
		ConfigFiles: []types.ConfigFile{
			{Filename: compose},
		},
	})
	assert.NilError(t, err)
	test := project.Services["test"]
	assert.Equal(t, len(test.Volumes), 2)
	assert.DeepEqual(t, test.Volumes, []types.ServiceVolumeConfig{
		{
			Type:   "bind",
			Source: "/bar",
			Target: "/bar",
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
		},
		{
			Type:   "bind",
			Source: "/zot",
			Target: "/zot",
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
		},
	})
	assert.DeepEqual(t, test.Networks, map[string]*types.ServiceNetworkConfig{
		"zot": nil,
	})
}

// https://github.com/docker/compose/issues/13346
func TestOverrideSelfExtends(t *testing.T) {
	yaml := `
name: test-override-extends
services:
  depend_base:
    image: nginx
    ports:
      - "8092:80"
  depend_one:
    image: nginx
    ports:
      - "8091:80"
  depend_two:
    extends:
      service: depend_one
  main_one:
    image: nginx
    depends_on:
      - depend_one
    ports:
      - "8090:80"
  main_two:
    extends: main_one
    depends_on: !override
      - depend_two
  main:
    extends:
      service: main_two
    depends_on:
      - depend_base
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "-",
				Content:  []byte(yaml),
			},
		},
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["main"].DependsOn, types.DependsOnConfig{
		"depend_base": {Condition: "service_started", Required: true},
		"depend_two":  {Condition: "service_started", Required: true},
	})
}
