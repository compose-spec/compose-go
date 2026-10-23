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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      cache_to:
        - user/app:cache
        - type=local,dest=path/to/cache
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].Build.CacheTo, types.StringList{"user/app:cache", "type=local,dest=path/to/cache"})
		assert.DeepEqual(t, p.Jobs["foo"].Build.CacheTo, types.StringList{"user/app:cache", "type=local,dest=path/to/cache"})
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      no_cache: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.NoCache, true)
		assert.Equal(t, p.Jobs["foo"].Build.NoCache, true)
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      pull: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.Pull, true)
		assert.Equal(t, p.Jobs["foo"].Build.Pull, true)
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      shm_size: 128m
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.ShmSize, types.UnitBytes(128*1024*1024))
		assert.Equal(t, p.Jobs["foo"].Build.ShmSize, types.UnitBytes(128*1024*1024))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// build.shm_size is a byte value like the service attribute of the same name:
// a number with an integer value is a number of bytes, and a number that is
// not (or does not fit in 64 bits) is rejected rather than silently read as 0.
func TestBuildShmSizeNumbers(t *testing.T) {
	build := func(value string) string {
		return "name: test\nservices:\n  foo:\n    build:\n      context: .\n      shm_size: " + value + "\n"
	}

	t.Run("an integer value", func(t *testing.T) {
		loadsAs(t, build("1e3"), build(`"1000"`))
	})

	t.Run("a fraction", func(t *testing.T) {
		err := loadErr(t, build("1.5"))
		assert.ErrorContains(t, err, "build.shm_size")
		assert.ErrorContains(t, err, "must be a integer or string")
	})

	t.Run("a number beyond 64 bits", func(t *testing.T) {
		err := loadErr(t, build("9223372036854775808"))
		assert.ErrorContains(t, err, "build.shm_size")
		assert.ErrorContains(t, err, "out of range")
	})
}

func TestBuildIsolation(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    build:
      context: .
      isolation: process
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      isolation: process
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.Isolation, "process")
		assert.Equal(t, p.Jobs["foo"].Build.Isolation, "process")
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      privileged: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Build.Privileged, true)
		assert.Equal(t, p.Jobs["foo"].Build.Privileged, true)
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      entitlements:
        - network.host
        - security.insecure
`)
	assert.DeepEqual(t, p.Services["foo"].Build.Entitlements, []string{"network.host", "security.insecure"})
	assert.DeepEqual(t, p.Jobs["foo"].Build.Entitlements, []string{"network.host", "security.insecure"})
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
jobs:
  foo:
    triggers:
      manual: true
    build:
      context: .
      provenance: mode=max
      sbom: true
`)
	assert.Equal(t, p.Services["foo"].Build.Provenance, "mode=max")
	assert.Equal(t, p.Services["foo"].Build.SBOM, "true")
	assert.Equal(t, p.Jobs["foo"].Build.Provenance, "mode=max")
	assert.Equal(t, p.Jobs["foo"].Build.SBOM, "true")
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
jobs:
  string:
    triggers:
      manual: true
    build:
      context: .
      no_cache_filter: foo
  list:
    triggers:
      manual: true
    build:
      context: .
      no_cache_filter: [foo, bar]
`)
	assert.DeepEqual(t, p.Services["string"].Build.NoCacheFilter, types.StringList{"foo"})
	assert.DeepEqual(t, p.Services["list"].Build.NoCacheFilter, types.StringList{"foo", "bar"})
	assert.DeepEqual(t, p.Jobs["string"].Build.NoCacheFilter, types.StringList{"foo"})
	assert.DeepEqual(t, p.Jobs["list"].Build.NoCacheFilter, types.StringList{"foo", "bar"})
}

// A negative size is rejected, like the Docker CLI does for the equivalent flag
// (docker build --shm-size).
func TestBuildShmSizeNotNegative(t *testing.T) {
	for name, value := range map[string]string{
		"a negative number":         `-1`,
		"a negative numeric string": `"-1"`,
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, "name: test\nservices:\n  foo:\n    build:\n      context: .\n      shm_size: "+value+"\n")
			assert.ErrorContains(t, err, "build.shm_size")
			assert.ErrorContains(t, err, "must not be negative")
		})
	}

	t.Run("a job", func(t *testing.T) {
		err := loadErr(t, "name: test\njobs:\n  foo:\n    triggers:\n      manual: true\n    build:\n      context: .\n      shm_size: -1\n")
		assert.ErrorContains(t, err, "jobs.foo.build.shm_size")
		assert.ErrorContains(t, err, "must not be negative")
	})
}
