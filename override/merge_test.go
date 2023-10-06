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

	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

// override using the same logging driver will override driver options
func Test_mergeOverrides(t *testing.T) {
	right := `
services:
  test:
    image: foo
    scale: 1
`
	left := `
services:
  test:
    image: bar
    scale: 2
`
	expected := `
services:
  test:
    image: bar
    scale: 2
`

	got, err := Merge(unmarshall(t, right), unmarshall(t, left))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshall(t, expected))
}

func assertMergeYaml(t *testing.T, right string, left string, want string) {
	got, err := Merge(unmarshall(t, right), unmarshall(t, left))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshall(t, want))
}

func unmarshall(t *testing.T, s string) map[string]any {
	var val map[string]any
	err := yaml.Unmarshal([]byte(s), &val)
	assert.NilError(t, err)
	return val
}
