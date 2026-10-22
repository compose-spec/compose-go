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

// The tests in this file lock the `mem_swappiness` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/container_spec.md#mem_swappiness
//
// Spec: "`mem_swappiness` defines as a percentage, a value between 0 and 100,
// for the host kernel to swap out anonymous memory pages used by a container.
// `0`: Turns off anonymous page swapping. `100`: Sets all anonymous pages as
// swappable. The default value is platform specific."
//
// Not locked here, because it is not settled that the current behavior is the
// intended one: an explicit 0 does not survive marshaling since it equals the
// unset value.

import (
	"fmt"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// mem_swappiness is part of the container specification, shared by services
// and jobs, and survives a YAML and JSON round trip.
func TestMemSwappiness(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mem_swappiness: 60
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    mem_swappiness: 60
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].MemSwappiness, types.UnitBytes(60))
		assert.Equal(t, p.Jobs["foo"].MemSwappiness, types.UnitBytes(60))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// The percentage is accepted as a number or as a numeric string, the upper
// bound of the specification included.
func TestMemSwappinessNotation(t *testing.T) {
	for name, tc := range map[string]struct {
		value string
		want  types.UnitBytes
	}{
		"a number":                  {`60`, 60},
		"a numeric string":          {`"60"`, 60},
		"all pages swappable":       {`100`, 100},
		"all pages swappable, text": {`"100"`, 100},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, serviceWith("mem_swappiness", tc.value))
			assert.Equal(t, p.Services["foo"].MemSwappiness, tc.want)
		})
	}
}

// The value is subject to interpolation like any other.
func TestMemSwappinessInterpolation(t *testing.T) {
	p := loadWithEnv(t, serviceWith("mem_swappiness", "${SWAPPINESS}"), map[string]string{"SWAPPINESS": "30"})
	assert.Equal(t, p.Services["foo"].MemSwappiness, types.UnitBytes(30))

	p = load(t, serviceWith("mem_swappiness", "${SWAPPINESS:-10}"))
	assert.Equal(t, p.Services["foo"].MemSwappiness, types.UnitBytes(10))
}

// A later file overrides the percentage of an earlier one.
func TestMemSwappinessOverride(t *testing.T) {
	m := loadFilesMap(t, []string{
		serviceWith("mem_swappiness", "10"),
		"services:\n  foo:\n    mem_swappiness: 90\n",
	})
	foo := m["services"].(map[string]any)["foo"].(map[string]any)
	assert.Equal(t, fmt.Sprint(foo["mem_swappiness"]), "90")
}

// A value that is not a percentage is rejected, with the attribute in the
// error: the specification restricts it to 0 to 100, which excludes amounts
// beyond (a byte value such as 1g included) and text.
func TestMemSwappinessInvalid(t *testing.T) {
	for name, tc := range map[string]struct{ value, message string }{
		"text":                  {`abc`, "mem_swappiness must be a percentage"},
		"an empty string":       {`""`, "mem_swappiness must be a percentage"},
		"a boolean":             {`true`, "must be a integer or string"},
		"a list":                {`[60]`, "must be a integer or string"},
		"a fractional value":    {`0.5`, "must be a integer or string"},
		"above 100":             {`101`, "invalid value 101: mem_swappiness must be a percentage"},
		"above 100, text":       {`"101"`, "invalid value 101: mem_swappiness must be a percentage"},
		"a float above 100":     {`1e3`, "invalid value 1000: mem_swappiness must be a percentage"},
		"below the default":     {`-2`, "invalid value -2: mem_swappiness must be a percentage"},
		"a byte value":          {`1g`, "invalid value 1g: mem_swappiness must be a percentage"},
		"a number of kilobytes": {`"1k"`, "invalid value 1k: mem_swappiness must be a percentage"},
		"beyond 64 bits":        {`9223372036854775808`, "invalid value 9223372036854775808: mem_swappiness must be a percentage"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, serviceWith("mem_swappiness", tc.value))
			assert.ErrorContains(t, err, "services.foo.mem_swappiness")
			assert.ErrorContains(t, err, tc.message)
		})
	}

	t.Run("in a job", func(t *testing.T) {
		err := loadErr(t, `
name: test
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    mem_swappiness: 101
`)
		assert.ErrorContains(t, err, "jobs.foo.mem_swappiness")
		assert.ErrorContains(t, err, "mem_swappiness must be a percentage")
	})

	t.Run("after interpolation", func(t *testing.T) {
		_, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(serviceWith("mem_swappiness", "${SWAPPINESS}"))}},
			Environment: map[string]string{"SWAPPINESS": "150"},
		}, func(options *loader.Options) {
			options.SkipConsistencyCheck = true
			options.SkipNormalization = true
		})
		assert.ErrorContains(t, err, "services.foo.mem_swappiness")
		assert.ErrorContains(t, err, "invalid value 150")
	})
}

// The bounds of the percentage are accepted, and so is -1, the default of the
// Docker CLI (--memory-swappiness) which the engine reads as "not set": it is
// outside the range of the specification, yet files spelling it have always been
// accepted.
func TestMemSwappinessBounds(t *testing.T) {
	for name, tc := range map[string]struct {
		value string
		want  types.UnitBytes
	}{
		"turned off":               {`0`, 0},
		"all pages swappable":      {`100`, 100},
		"the default of the CLI":   {`-1`, -1},
		"the default, as a string": {`"-1"`, -1},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, serviceWith("mem_swappiness", tc.value))
			assert.Equal(t, p.Services["foo"].MemSwappiness, tc.want)
		})
	}
}
