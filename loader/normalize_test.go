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
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/goccy/go-yaml"
	"gotest.tools/v3/assert"
)

func TestNormalizeNetworkNames(t *testing.T) {
	project := `
name: myProject
services:
  foo:
    build:
      context: ./testdata
      args:
        FOO: null
        ZOT: null
networks:
  myExternalnet:
    external: true
  myNamedNet:
    name: CustomName
  mynet: {}
`
	expected := `name: myProject
services:
  foo:
    build:
      context: ./testdata
      dockerfile: Dockerfile
      args:
        FOO: BAR
    networks:
      default: null
networks:
  default:
    name: myProject_default
  myExternalnet:
    name: myExternalnet
    external: true
  myNamedNet:
    name: CustomName
  mynet:
    name: myProject_mynet
`

	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, types.Mapping{"FOO": "BAR"})
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestNormalizeVolumes(t *testing.T) {
	project := `
name: myProject
volumes:  
  myExternalVol: 
    external: true
  myvol: {}
  myNamedVol: 
    name: CustomName
`

	expected := `
name: myProject
volumes:  
  myExternalVol: 
    name: myExternalVol
    external: true
  myvol: 
    name: myProject_myvol
  myNamedVol: 
    name: CustomName
`
	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, nil)
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestNormalizeDependsOn(t *testing.T) {
	project := `
name: myProject
services:
  foo:
    depends_on: 
      bar:
        condition: service_healthy
        required: true
        restart: true
    network_mode: service:zot

  bar: 
    volumes_from:
      - zot
      - container:xxx

  zot: {}
`
	expected := `
name: myProject
services:
  bar:
    depends_on:
      zot:
        condition: service_started
        required: true
        restart: false
    networks:
      default: null
    volumes_from:
      - zot
      - container:xxx
  foo:
    depends_on:
      bar:
        condition: service_healthy
        required: true
        restart: true
      zot:
        condition: service_started
        required: true
        restart: true
    network_mode: service:zot
  zot:
    networks:
      default: null
networks:
  default:
    name: myProject_default
`
	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, nil)
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestNormalizeImplicitDependencies(t *testing.T) {
	project := `
name: myProject
services:
  test:
    ipc: service:foo
    cgroup: service:bar
    uts: service:baz
    pid: service:qux
    volumes_from: [quux]
    links: [corge]
    depends_on: # explicit dependency MUST not be overridden
      foo: 
        condition: service_healthy
`

	expected := `
name: myProject
services:
  test:
    ipc: service:foo
    cgroup: service:bar
    uts: service:baz
    pid: service:qux
    volumes_from: [quux]
    links: [corge]
    depends_on: # explicit dependency MUST not be overridden
      foo: 
        condition: service_healthy
      bar: 
        condition: service_started
        restart: true
        required: true
      baz: 
        condition: service_started
        restart: true
        required: true
      qux: 
        condition: service_started
        restart: true
        required: true
      quux: 
        condition: service_started
        required: true
        restart: false
      corge: 
        condition: service_started
        restart: true
        required: true
    networks:
      default: null
networks:
  default:
    name: myProject_default
`

	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, nil)
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestImplicitContextPath(t *testing.T) {
	project := `
name: myProject
services: 
  test:
    build: {}
`
	expected := `
name: myProject
services: 
  test:
    build:
      context: .
      dockerfile: "Dockerfile"
    networks:
      default: null
networks:
  default:
    name: myProject_default
`

	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, nil)
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestNormalizeDefaultNetwork(t *testing.T) {
	project := `
name: myProject
services:  
  test:
    image: test
`

	expected := `
name: myProject
networks:
  default:
    name: myProject_default
services:  
  test: 
    image: test
    networks:
      default: null
`
	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, nil)
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestNormalizeCustomNetwork(t *testing.T) {
	project := `
name: myProject
services:  
  test: 
    networks:
      my_network: null
networks:
  my_network: null
`

	expected := `
name: myProject
networks:
  my_network:
    name: myProject_my_network
services:  
  test: 
    networks:
      my_network: null
`
	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, nil)
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}

func TestNormalizeEnvironment(t *testing.T) {
	project := `
name: myProject
services:  
  test: 
    environment:
      - FOO
      - BAR
      - ZOT=QIX
`

	expected := `
name: myProject
networks:
  default:
    name: myProject_default
services:  
  test: 
    environment:
      - FOO
      - BAR=bar
      - ZOT=QIX
    networks:
      default: null
`
	var model map[string]any
	err := yaml.Unmarshal([]byte(project), &model)
	assert.NilError(t, err)
	model, err = Normalize(model, map[string]string{
		"BAR": "bar",
		"ZOT": "zot",
	})
	assert.NilError(t, err)

	var expect map[string]any
	err = yaml.Unmarshal([]byte(expected), &expect)
	assert.NilError(t, err)
	assert.DeepEqual(t, expect, model)
}
