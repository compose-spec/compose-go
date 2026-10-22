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
// intended one: the value is not range-checked, and an explicit 0 does not
// survive marshaling since it equals the unset value.

import (
	"fmt"
	"testing"

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
// error.
func TestMemSwappinessInvalid(t *testing.T) {
	for name, tc := range map[string]struct{ value, message string }{
		"text":               {`abc`, "invalid size"},
		"an empty string":    {`""`, "invalid size"},
		"a boolean":          {`true`, "must be a integer or string"},
		"a list":             {`[60]`, "must be a integer or string"},
		"a fractional value": {`0.5`, "must be a integer or string"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, serviceWith("mem_swappiness", tc.value))
			assert.ErrorContains(t, err, "mem_swappiness")
			assert.ErrorContains(t, err, tc.message)
		})
	}
}
