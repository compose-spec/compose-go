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

// The tests in this file lock the `memswap_limit` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/container_spec.md#memswap_limit
//   https://github.com/compose-spec/compose-spec/blob/main/11-extension.md#specifying-byte-values
//
// Spec: "`memswap_limit` defines the amount of memory the container is allowed
// to swap to disk. This is a modifier attribute that only has meaning if
// `memory` is also set. [...] If `memswap_limit` is set to a positive integer,
// then both `memory` and `memswap_limit` must be set. `memswap_limit`
// represents the total amount of memory and swap that can be used."
//
// Whether `memory` is set alongside is a matter for the container runtime:
// nothing here depends on it.

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// memswap_limit is part of the container specification, shared by services and
// jobs, and survives a YAML and JSON round trip as a number of bytes.
func TestMemSwapLimit(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    memswap_limit: 512m
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    memswap_limit: 512m
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].MemSwapLimit, types.UnitBytes(512*1024*1024))
		assert.Equal(t, p.Jobs["foo"].MemSwapLimit, types.UnitBytes(512*1024*1024))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// A byte value is accepted in every notation of the specification, and
// canonicalizes into a number of bytes.
func TestMemSwapLimitByteValues(t *testing.T) {
	loadsByteValueNotations(t, "memswap_limit")
}

// The schema types memswap_limit as a number or a string: a number with an integer
// value is a number of bytes whatever its notation, any other number is
// rejected rather than silently read as 0, which would leave the amount unset.
func TestMemSwapLimitNumbers(t *testing.T) {
	t.Run("an integer value", func(t *testing.T) {
		loadsIntegerValuedNumbers(t, "memswap_limit")
	})
	t.Run("not a number of bytes", func(t *testing.T) {
		rejectsUnrepresentableNumbers(t, "memswap_limit")
	})
}

// The value is subject to interpolation like any other, whatever its notation.
func TestMemSwapLimitInterpolation(t *testing.T) {
	for name, tc := range map[string]struct {
		env  string
		want types.UnitBytes
	}{
		"a value with a unit": {"512m", 512 * 1024 * 1024},
		"a number of bytes":   {"1048576", 1048576},
	} {
		t.Run(name, func(t *testing.T) {
			p := loadWithEnv(t, serviceWith("memswap_limit", "${MEMSWAP_LIMIT}"), map[string]string{"MEMSWAP_LIMIT": tc.env})
			assert.Equal(t, p.Services["foo"].MemSwapLimit, tc.want)
		})
	}

	t.Run("a default value", func(t *testing.T) {
		loadsAs(t, serviceWith("memswap_limit", "${MEMSWAP_LIMIT:-1g}"), serviceWith("memswap_limit", `"1073741824"`))
	})
}

// A later file overrides the limit of an earlier one, whatever the notation
// each of them uses.
func TestMemSwapLimitOverride(t *testing.T) {
	loadFilesAs(t,
		[]string{
			serviceWith("memswap_limit", "256m"),
			"services:\n  foo:\n    memswap_limit: 1g\n",
		},
		serviceWith("memswap_limit", `"1073741824"`))

	loadFilesAs(t,
		[]string{
			serviceWith("memswap_limit", "1048576"),
			"services:\n  foo:\n    memswap_limit: 2m\n",
		},
		serviceWith("memswap_limit", `"2097152"`))
}

// A value that is not a byte value is rejected, with the attribute in the
// error.
func TestMemSwapLimitInvalid(t *testing.T) {
	for name, tc := range map[string]struct{ value, message string }{
		"an unknown notation": {`abc`, "invalid size"},
		"an empty string":     {`""`, "invalid size"},
		"a boolean":           {`true`, "must be a number or string"},
		"a list":              {`[512m]`, "must be a number or string"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, serviceWith("memswap_limit", tc.value))
			assert.ErrorContains(t, err, "memswap_limit")
			assert.ErrorContains(t, err, tc.message)
		})
	}
}

// Spec: "If `memswap_limit` is explicitly set to -1, the container is allowed
// to use unlimited swap, up to the amount available on the host system."
func TestMemSwapLimitUnlimitedSwap(t *testing.T) {
	for name, value := range map[string]string{
		"a number":         `-1`,
		"a numeric string": `"-1"`,
	} {
		t.Run(name, func(t *testing.T) {
			loadsAs(t, serviceWith("memswap_limit", value), serviceWith("memswap_limit", `"-1"`))
		})
	}
}

// Spec: "If `memswap_limit` is set to 0, the setting is ignored, and the value
// is treated as unset."
func TestMemSwapLimitZeroIsUnset(t *testing.T) {
	loadsAs(t, serviceWith("memswap_limit", "0"), "name: test\nservices:\n  foo:\n    image: alpine\n")
}
