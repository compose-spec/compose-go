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

// The tests in this file lock the `shm_size` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/container_spec.md#shm_size
//   https://github.com/compose-spec/compose-spec/blob/main/11-extension.md#specifying-byte-values
//
// Spec: "`shm_size` configures the size of the shared memory (`/dev/shm`
// partition on Linux) allowed by the service container. It's specified as a
// byte value."
//
// The `build.shm_size` attribute, which sizes the shared memory of image
// builds, is locked by the build tests.

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// shm_size is part of the container specification, shared by services and
// jobs, and survives a YAML and JSON round trip as a number of bytes.
func TestShmSize(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    shm_size: 512m
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    shm_size: 512m
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].ShmSize, types.UnitBytes(512*1024*1024))
		assert.Equal(t, p.Jobs["foo"].ShmSize, types.UnitBytes(512*1024*1024))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// A byte value is accepted in every notation of the specification, and
// canonicalizes into a number of bytes.
func TestShmSizeByteValues(t *testing.T) {
	loadsByteValueNotations(t, "shm_size")
}

// The schema types shm_size as a number or a string: a number with an integer
// value is a number of bytes whatever its notation, any other number is
// rejected rather than silently read as 0, which would leave the amount unset.
func TestShmSizeNumbers(t *testing.T) {
	t.Run("an integer value", func(t *testing.T) {
		loadsIntegerValuedNumbers(t, "shm_size")
	})
	t.Run("not a number of bytes", func(t *testing.T) {
		rejectsUnrepresentableNumbers(t, "shm_size")
	})
}

// The value is subject to interpolation like any other, whatever its notation.
func TestShmSizeInterpolation(t *testing.T) {
	for name, tc := range map[string]struct {
		env  string
		want types.UnitBytes
	}{
		"a value with a unit": {"512m", 512 * 1024 * 1024},
		"a number of bytes":   {"1048576", 1048576},
	} {
		t.Run(name, func(t *testing.T) {
			p := loadWithEnv(t, serviceWith("shm_size", "${SHM_SIZE}"), map[string]string{"SHM_SIZE": tc.env})
			assert.Equal(t, p.Services["foo"].ShmSize, tc.want)
		})
	}

	t.Run("a default value", func(t *testing.T) {
		loadsAs(t, serviceWith("shm_size", "${SHM_SIZE:-1g}"), serviceWith("shm_size", `"1073741824"`))
	})
}

// A later file overrides the size of an earlier one, whatever the notation
// each of them uses.
func TestShmSizeOverride(t *testing.T) {
	loadFilesAs(t,
		[]string{
			serviceWith("shm_size", "256m"),
			"services:\n  foo:\n    shm_size: 1g\n",
		},
		serviceWith("shm_size", `"1073741824"`))

	loadFilesAs(t,
		[]string{
			serviceWith("shm_size", "1048576"),
			"services:\n  foo:\n    shm_size: 2m\n",
		},
		serviceWith("shm_size", `"2097152"`))
}

// A value that is not a byte value is rejected, with the attribute in the
// error.
func TestShmSizeInvalid(t *testing.T) {
	for name, tc := range map[string]struct{ value, message string }{
		"an unknown notation": {`abc`, "invalid size"},
		"an empty string":     {`""`, "invalid size"},
		"a boolean":           {`true`, "must be a number or string"},
		"a list":              {`[512m]`, "must be a number or string"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, serviceWith("shm_size", tc.value))
			assert.ErrorContains(t, err, "shm_size")
			assert.ErrorContains(t, err, tc.message)
		})
	}
}
