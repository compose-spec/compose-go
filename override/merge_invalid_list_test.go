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

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// convertIntoMapping must return an error (not panic) when a list element
// cannot be used as a string key.
func Test_mergeInvalidListElement(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		override string
	}{
		{
			name: "depends_on map element",
			base: `
services:
  x: {image: alpine, depends_on: [y]}
  y: {image: alpine}
`,
			override: `
services:
  x: {depends_on: [{}]}
`,
		},
		{
			name: "depends_on int element",
			base: `
services:
  x: {image: alpine, depends_on: [y]}
  y: {image: alpine}
`,
			override: `
services:
  x: {depends_on: [123]}
`,
		},
		{
			name: "networks map element",
			base: `
services:
  x: {image: alpine, networks: [front]}
`,
			override: `
services:
  x: {networks: [{}]}
`,
		},
		{
			name: "depends_on invalid element in base",
			base: `
services:
  x: {image: alpine, depends_on: [{}]}
  y: {image: alpine}
`,
			override: `
services:
  x: {depends_on: [y]}
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Merge(unmarshal(t, tc.base), unmarshal(t, tc.override))
			assert.Check(t, is.ErrorContains(err, "cannot use"))
		})
	}
}
