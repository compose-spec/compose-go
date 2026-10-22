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

// The tests in this file lock the `read_only` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/container_spec.md#read_only
//
// Spec: "`read_only` configures the service container to be created with a
// read-only filesystem."
//
// The `read_only` flag of a volume mount is another attribute, locked by the
// volumes tests.

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// read_only is part of the container specification, shared by services and
// jobs, and survives a YAML and JSON round trip.
func TestReadOnly(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    read_only: true
  bar:
    image: alpine
    read_only: false
  baz:
    image: alpine
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    read_only: true
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].ReadOnly, true)
		assert.Equal(t, p.Services["bar"].ReadOnly, false)
		assert.Equal(t, p.Services["baz"].ReadOnly, false)
		assert.Equal(t, p.Jobs["foo"].ReadOnly, true)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// A boolean written as a string is accepted, as the value may come from
// interpolation.
func TestReadOnlyBooleanString(t *testing.T) {
	for name, tc := range map[string]struct {
		value string
		want  bool
	}{
		"true as a string":  {`"true"`, true},
		"false as a string": {`"false"`, false},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, serviceWith("read_only", tc.value))
			assert.Equal(t, p.Services["foo"].ReadOnly, tc.want)
		})
	}
}

func TestReadOnlyInterpolation(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		t.Run(value, func(t *testing.T) {
			p := loadWithEnv(t, serviceWith("read_only", "${READ_ONLY}"), map[string]string{"READ_ONLY": value})
			assert.Equal(t, p.Services["foo"].ReadOnly, value == "true")
		})
	}

	t.Run("a default value", func(t *testing.T) {
		loadsAs(t, serviceWith("read_only", "${READ_ONLY:-true}"), serviceWith("read_only", "true"))
	})
}

// A later file overrides the flag of an earlier one, in both directions: the
// canonical form of false is the absence of the attribute.
func TestReadOnlyOverride(t *testing.T) {
	t.Run("false then true", func(t *testing.T) {
		loadFilesAs(t,
			[]string{
				serviceWith("read_only", "false"),
				"services:\n  foo:\n    read_only: true\n",
			},
			serviceWith("read_only", "true"))
	})

	t.Run("true then false", func(t *testing.T) {
		loadFilesAs(t,
			[]string{
				serviceWith("read_only", "true"),
				"services:\n  foo:\n    read_only: false\n",
			},
			"name: test\nservices:\n  foo:\n    image: alpine\n")
	})
}

// A value that is not a boolean is rejected, with the attribute in the error.
func TestReadOnlyInvalid(t *testing.T) {
	for name, tc := range map[string]struct{ value, message string }{
		"text":            {`abc`, "invalid boolean"},
		"an empty string": {`""`, "invalid boolean"},
		"a list":          {`[true]`, "must be a boolean or string"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, serviceWith("read_only", tc.value))
			assert.ErrorContains(t, err, "services.foo.read_only")
			assert.ErrorContains(t, err, tc.message)
		})
	}
}
