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

	"github.com/compose-spec/compose-go/tree"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

func Test_mergeYamlBase(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    container_name: foo
    init:  true 
`, `
services:
  test:
    image: bar
    init:  false 
`, `
services:
  test:
    container_name: foo
    image: bar
    init: false
`)
}

func assertMergeYaml(t *testing.T, right string, left string, want string) {
	got, err := MergeYaml(unmarshall(t, right), unmarshall(t, left), tree.NewPath())
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshall(t, want))
}

func unmarshall(t *testing.T, s string) map[string]interface{} {
	var val map[string]interface{}
	err := yaml.Unmarshal([]byte(s), &val)
	assert.NilError(t, err)
	return val
}
