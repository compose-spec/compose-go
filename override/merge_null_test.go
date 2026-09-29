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

package override

import (
	"testing"
)

// An override setting a list-or-mapping attribute to null is a valid document
// and must merge without error, leaving the base value in place: consumers
// such as buildx bake only read build attributes and must keep loading files
// whose runtime attributes are nulled by an override.
// Regression of v2.15.0, reported by docker/buildx#4117.
func Test_mergeYamlNullOverrideOfMappingLikeAttribute(t *testing.T) {
	tests := []struct {
		attribute string
		want      string
	}{
		{attribute: "networks", want: "runtime: null"},
		{attribute: "models", want: "runtime: null"},
		{attribute: "depends_on", want: "runtime: {condition: service_started, required: true}"},
	}
	for _, tt := range tests {
		t.Run(tt.attribute, func(t *testing.T) {
			assertMergeYaml(t, `
services:
  test:
    image: foo
    `+tt.attribute+`: [runtime]
`, `
services:
  test:
    `+tt.attribute+`: null
`, `
services:
  test:
    image: foo
    `+tt.attribute+`:
      `+tt.want+`
`)
		})
	}
}

// A base setting the attribute to null and an override providing entries is
// equally valid: the override's entries are the result.
func Test_mergeYamlNullBaseWithOverrideOfMappingLikeAttribute(t *testing.T) {
	tests := []struct {
		attribute string
		want      string
	}{
		{attribute: "networks", want: "runtime: null"},
		{attribute: "models", want: "runtime: null"},
		{attribute: "depends_on", want: "runtime: {condition: service_started, required: true}"},
	}
	for _, tt := range tests {
		t.Run(tt.attribute, func(t *testing.T) {
			assertMergeYaml(t, `
services:
  test:
    image: foo
    `+tt.attribute+`: null
`, `
services:
  test:
    `+tt.attribute+`: [runtime]
`, `
services:
  test:
    image: foo
    `+tt.attribute+`:
      `+tt.want+`
`)
		})
	}
}
