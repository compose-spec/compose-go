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

func TestBuildCacheTo(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      cache_to:
        - user/app:cache
        - type=local,dest=path/to/cache
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].Build.CacheTo, types.StringList{"user/app:cache", "type=local,dest=path/to/cache"})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildNoCache(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      no_cache: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.NoCache, true)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildPull(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      pull: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.Pull, true)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildShmSize(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      shm_size: 128m
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.ShmSize, types.UnitBytes(128*1024*1024))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildIsolation(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      isolation: process
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.Isolation, "process")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildPrivileged(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      privileged: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.Privileged, true)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestBuildEntitlements(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      entitlements:
        - network.host
        - security.insecure
`)
	assert.DeepEqual(t, p.Services["foo"].Build.Entitlements, []string{"network.host", "security.insecure"})
}

func TestBuildAttestations(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      provenance: mode=max
      sbom: true
`)
	assert.Equal(t, p.Services["foo"].Build.Provenance, "mode=max")
	assert.Equal(t, p.Services["foo"].Build.SBOM, "true")
}

func TestBuildNoCacheFilter(t *testing.T) {
	p := load(t, `
name: test
services:
  string:
    build:
      context: .
      no_cache_filter: foo
  list:
    build:
      context: .
      no_cache_filter: [foo, bar]
`)
	assert.DeepEqual(t, p.Services["string"].Build.NoCacheFilter, types.StringList{"foo"})
	assert.DeepEqual(t, p.Services["list"].Build.NoCacheFilter, types.StringList{"foo", "bar"})
}
