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

func TestBuildConfig(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: ./dir
      dockerfile: Dockerfile
      args:
        foo: bar
      target: foo
      network: foo
      cache_from:
        - foo
        - bar
      labels: [FOO=BAR]
      additional_contexts:
        foo: ./bar
      tags:
        - foo:v1.0.0
        - docker.io/username/foo:my-other-tag
      platforms:
        - linux/amd64
        - linux/arm64
`)

	expect := func(p *types.Project) {
		b := p.Services["foo"].Build
		assert.Equal(t, b.Dockerfile, "Dockerfile")
		assert.DeepEqual(t, b.Args, types.MappingWithEquals{"foo": ptr("bar")})
		assert.Equal(t, b.Target, "foo")
		assert.Equal(t, b.Network, "foo")
		assert.DeepEqual(t, b.CacheFrom, types.StringList{"foo", "bar"})
		assert.DeepEqual(t, b.Labels, types.Labels{"FOO": "BAR"})
		assert.DeepEqual(t, b.Tags, types.StringList{"foo:v1.0.0", "docker.io/username/foo:my-other-tag"})
		assert.DeepEqual(t, b.Platforms, types.StringList{"linux/amd64", "linux/arm64"})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDockerfileInline(t *testing.T) {
	p := load(t, `
name: test
services:
  bar:
    build:
      dockerfile_inline: |
        FROM alpine
        RUN echo "hello" > /world.txt
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["bar"].Build.DockerfileInline, "FROM alpine\nRUN echo \"hello\" > /world.txt\n")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildSSH(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      ssh:
        - default
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].Build.SSH, types.SSHConfig{{ID: "default", Path: ""}})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildSecrets(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      secrets:
        - source: secret1
          target: /run/secrets/secret1
        - source: secret2
          target: my_secret
          uid: '103'
          gid: '103'
          mode: 0440
secrets:
  secret1:
    file: ./secret_data
  secret2:
    external: true
`)
	secrets := p.Services["foo"].Build.Secrets
	assert.Equal(t, len(secrets), 2)
	assert.Equal(t, secrets[0].Source, "secret1")
	assert.Equal(t, secrets[1].UID, "103")
	assert.Equal(t, *secrets[1].Mode, types.FileMode(0o440))
}
