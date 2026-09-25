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

// The tests in this file lock the `build` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/build.md
//
// Spec: "A Compose implementation which focuses on running an application on a
// local machine needs to also support (re)building the application from
// source."

import (
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// A service additional context may refer to a build-only service in an inactive
// profile without enabling that service for execution.
// https://github.com/compose-spec/compose-spec/blob/main/build.md#additional_contexts
// Regression: https://github.com/docker/compose/issues/14223.
func TestBuildAdditionalContextDisabledService(t *testing.T) {
	for _, tc := range []struct {
		name      string
		base      string
		wantError string
	}{
		{"buildable", "build: .", ""},
		{"image only", "image: busybox", "non-buildable service"},
		{"missing", "", "unknown service"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := `
name: test
services:
  classroom:
    profiles: [classroom]
    build:
      context: .
      additional_contexts:
        base: service:base
`
			if tc.base != "" {
				content += "  base:\n    profiles: [disabled_services]\n    " + tc.base + "\n"
			}
			p, err := loader.LoadWithContext(context.Background(), types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
			}, func(options *loader.Options) {
				options.Profiles = []string{"classroom"}
			})
			if tc.wantError != "" {
				assert.ErrorContains(t, err, tc.wantError)
				assert.ErrorContains(t, err, `service "classroom"`)
				assert.ErrorContains(t, err, `"base" as additional contexts base`)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, len(p.Services), 1)
			assert.Equal(t, p.Services["classroom"].Build.AdditionalContexts["base"], "service:base")
			assert.Equal(t, len(p.DisabledServices), 1)
			assert.Assert(t, p.DisabledServices["base"].Build != nil)
		})
	}
}

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
jobs:
  foo:
    triggers:
      manual: true
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

		jb := p.Jobs["foo"].Build
		assert.Equal(t, jb.Dockerfile, "Dockerfile")
		assert.DeepEqual(t, jb.Args, types.MappingWithEquals{"foo": ptr("bar")})
		assert.Equal(t, jb.Target, "foo")
		assert.Equal(t, jb.Network, "foo")
		assert.DeepEqual(t, jb.CacheFrom, types.StringList{"foo", "bar"})
		assert.DeepEqual(t, jb.Labels, types.Labels{"FOO": "BAR"})
		assert.DeepEqual(t, jb.Tags, types.StringList{"foo:v1.0.0", "docker.io/username/foo:my-other-tag"})
		assert.DeepEqual(t, jb.Platforms, types.StringList{"linux/amd64", "linux/arm64"})
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
jobs:
  bar:
    triggers:
      manual: true
    build:
      dockerfile_inline: |
        FROM alpine
        RUN echo "hello" > /world.txt
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["bar"].Build.DockerfileInline, "FROM alpine\nRUN echo \"hello\" > /world.txt\n")
		assert.Equal(t, p.Jobs["bar"].Build.DockerfileInline, "FROM alpine\nRUN echo \"hello\" > /world.txt\n")
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      ssh:
        - default
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].Build.SSH, types.SSHConfig{{ID: "default", Path: ""}})
		assert.DeepEqual(t, p.Jobs["foo"].Build.SSH, types.SSHConfig{{ID: "default", Path: ""}})
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
jobs:
  foo:
    triggers:
      manual: true
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

	jobSecrets := p.Jobs["foo"].Build.Secrets
	assert.Equal(t, len(jobSecrets), 2)
	assert.Equal(t, jobSecrets[0].Source, "secret1")
	assert.Equal(t, jobSecrets[1].UID, "103")
	assert.Equal(t, *jobSecrets[1].Mode, types.FileMode(0o440))
}
