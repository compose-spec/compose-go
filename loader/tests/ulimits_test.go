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

// The tests in this file lock the `ulimits` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#ulimits
//
// Spec: "`ulimits` overrides the default ulimits for a container."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestUlimits(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    ulimits:
      nproc: 65535
      nofile:
        soft: 20000
        hard: 40000
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    ulimits:
      nproc: 65535
      nofile:
        soft: 20000
        hard: 40000
`)
	expect := func(p *types.Project) {
		expected := map[string]*types.UlimitsConfig{
			"nproc": {Single: 65535},
			"nofile": {
				Soft: 20000,
				Hard: 40000,
			},
		}
		assert.DeepEqual(t, p.Services["foo"].Ulimits, expected)
		assert.DeepEqual(t, p.Jobs["foo"].Ulimits, expected)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// A ulimit whose soft or hard limit is not a number is rejected with an error
// naming the attribute, even when neither schema validation nor interpolation
// (which casts it) runs, instead of crashing the loader.
func TestUlimitsWithNonNumericLimit(t *testing.T) {
	for name, limits := range map[string]string{
		"soft limit": "{soft: unlimited, hard: 2}",
		"hard limit": "{soft: 1, hard: unlimited}",
	} {
		t.Run(name, func(t *testing.T) {
			err := loadUncheckedErr(t, serviceWith("ulimits", "{nofile: "+limits+"}"))
			assert.ErrorContains(t, err, "services[foo].ulimits[nofile]")
			assert.ErrorContains(t, err, "invalid type string for ulimit")
		})
	}
}
