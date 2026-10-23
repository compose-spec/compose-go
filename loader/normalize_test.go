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
	"go.yaml.in/yaml/v4"
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

func TestNormalizePreStartInheritsServiceImage(t *testing.T) {
	project := `
name: myProject
services:
  app:
    image: alpine
    pre_start:
      - command: ["migrate"]
      - image: busybox
        command: ["chown", "-R", "1000:1000", "/data"]
  builder:
    build:
      context: .
    pre_start:
      - command: ["init"]
  hybrid:
    image: ubuntu
    build:
      context: .
    pre_start:
      - command: ["bootstrap"]
`
	expected := `
name: myProject
services:
  app:
    image: alpine
    networks:
      default: null
    pre_start:
      - command: ["migrate"]
        image: alpine
        networks:
          default: null
      - image: busybox
        command: ["chown", "-R", "1000:1000", "/data"]
        networks:
          default: null
  builder:
    build:
      context: .
      dockerfile: Dockerfile
    networks:
      default: null
    pre_start:
      - command: ["init"]
        networks:
          default: null
  hybrid:
    image: ubuntu
    build:
      context: .
      dockerfile: Dockerfile
    networks:
      default: null
    pre_start:
      - command: ["bootstrap"]
        image: ubuntu
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

// normalizesTo asserts that Normalize turns the input compose model into the
// expected one, both expressed as YAML.
func normalizesTo(t *testing.T, env types.Mapping, input, expected string) {
	t.Helper()
	var model map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(input), &model))
	model, err := Normalize(model, env)
	assert.NilError(t, err)

	var expect map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(expected), &expect))
	assert.DeepEqual(t, expect, model)
}

// Normalize must rewrite the deprecated `if_not_present` pull policy to its
// canonical `missing` spelling and leave the other policies untouched.
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#pull_policy
func TestNormalizePullPolicy(t *testing.T) {
	normalizesTo(t, nil, `
name: myProject
services:
  legacy:
    image: foo
    pull_policy: if_not_present
  always:
    image: foo
    pull_policy: always
`, `
name: myProject
services:
  legacy:
    image: foo
    pull_policy: missing
    networks:
      default: null
  always:
    image: foo
    pull_policy: always
    networks:
      default: null
networks:
  default:
    name: myProject_default
`)
}

// Normalize applies the same container-level rules to `jobs` as to
// `services`: build defaults, environment resolution, implicit dependencies
// (links, shared namespaces, volumes_from) and volume target cleaning, with
// every implicit dependency sharing a single depends_on map.
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#depends_on
func TestNormalizeContainerRulesApplyToJobs(t *testing.T) {
	normalizesTo(t, types.Mapping{"FROM_ENV": "resolved", "ARG_FROM_ENV": "built"}, `
name: myProject
services:
  db:
    image: postgres
  data:
    image: data
  base:
    image: base
jobs:
  migrate:
    build:
      args:
        ARG_FROM_ENV: null
        ARG_UNSET: null
    environment:
      FROM_ENV: null
      UNSET: null
    links:
      - "db:database"
      - data
    network_mode: "service:base"
    ipc: "service:base"
    volumes:
      - type: bind
        source: /src
        target: /app//data/
    volumes_from:
      - data:ro
      - "container:external"
    depends_on:
      db:
        condition: service_healthy
        restart: false
        required: false
`, `
name: myProject
services:
  db:
    image: postgres
    networks:
      default: null
  data:
    image: data
    networks:
      default: null
  base:
    image: base
    networks:
      default: null
jobs:
  migrate:
    build:
      context: .
      dockerfile: Dockerfile
      args:
        ARG_FROM_ENV: built
    environment:
      FROM_ENV: resolved
      UNSET: null
    links:
      - "db:database"
      - data
    network_mode: "service:base"
    ipc: "service:base"
    volumes:
      - type: bind
        source: /src
        target: /app/data
    volumes_from:
      - data:ro
      - "container:external"
    depends_on:
      db:
        condition: service_healthy
        restart: false
        required: false
      data:
        condition: service_started
        restart: true
        required: true
      base:
        condition: service_started
        restart: true
        required: true
networks:
  default:
    name: myProject_default
`)
}

// A pre_start hook whose declarations cannot be merged with the service's own
// container specification must fail the normalization, reporting the hook
// resolution.
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#pre_start
func TestNormalizePreStartMergeError(t *testing.T) {
	var model map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(`
name: myProject
services:
  app:
    image: alpine
    logging:
      driver: json-file
    pre_start:
      - command: ["migrate"]
        logging: not-a-mapping
`), &model))
	_, err := Normalize(model, nil)
	assert.ErrorContains(t, err, "resolving pre_start hook specification: services.hook.logging: cannot merge logging")
}

// Normalize completes a model in canonical form. A model whose attribute has
// another shape, which schema validation would have rejected, is reported as an
// error naming the attribute instead of crashing the caller: Normalize is
// exported, and is also reached with SkipValidation.
func TestNormalizeRejectsMisshapenModel(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr string
	}{
		{"services is a list", "services: [a]", "services must be a mapping"},
		{"jobs is a list", "jobs: [a]", "jobs must be a mapping"},
		{"service is a string", "services:\n  a: image", "services.a must be a mapping"},
		{"service is empty", "services:\n  a: null", "services.a must be a mapping"},
		{"networks is a string", "networks: front", "networks must be a mapping"},
		{"service networks is a list", "services:\n  a:\n    networks: [front]", "services.a.networks must be a mapping"},
		{"build is a string", "services:\n  a:\n    build: .", "services.a.build must be a mapping"},
		{"depends_on is a list", "services:\n  a:\n    depends_on: [b]", "services.a.depends_on must be a mapping"},
		{"links is a string", "services:\n  a:\n    links: b", "services.a.links must be a list"},
		{"link is a number", "services:\n  a:\n    links: [1]", "services.a.links[0] must be a string"},
		{"network_mode is a list", "services:\n  a:\n    network_mode: [host]", "services.a.network_mode must be a string"},
		{"volumes is a string", "services:\n  a:\n    volumes: /data", "services.a.volumes must be a list"},
		{"volume is a string", "services:\n  a:\n    volumes: [\"/data\"]", "services.a.volumes[0] must be a mapping"},
		{"volume has no target", "services:\n  a:\n    volumes:\n      - type: volume", "services.a.volumes[0].target must be a string"},
		{"volumes_from is a string", "services:\n  a:\n    volumes_from: b", "services.a.volumes_from must be a list"},
		{"volumes_from entry is a number", "services:\n  a:\n    volumes_from: [1]", "services.a.volumes_from[0] must be a string"},
		{"volume resource is a string", "volumes:\n  data: local", "volumes.data must be a mapping"},
		{"secrets is a list", "secrets: [pw]", "secrets must be a mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var model map[string]any
			assert.NilError(t, yaml.Unmarshal([]byte(tt.model), &model))
			_, err := Normalize(model, types.Mapping{})
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
