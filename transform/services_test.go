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

package transform

import (
	"testing"

	"github.com/compose-spec/compose-go/tree"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

func TestMakeServiceSlide(t *testing.T) {
	var mapping interface{}
	yaml.Unmarshal([]byte(`
foo:
  image: foo
bar:
  image: bar
zot:
  image: zot
`), &mapping)

	slice, err := makeServicesSlice(mapping, tree.NewPath("services"))
	assert.NilError(t, err)

	services := slice.([]interface{})
	slices.SortFunc(services, func(a, b interface{}) bool {
		right := a.(map[string]interface{})
		left := b.(map[string]interface{})
		return right["name"].(string) < left["name"].(string)
	})
	assert.DeepEqual(t, services, []interface{}{
		map[string]interface{}{
			"name":  "bar",
			"image": "bar",
		},
		map[string]interface{}{
			"name":  "foo",
			"image": "foo",
		},
		map[string]interface{}{
			"name":  "zot",
			"image": "zot",
		},
	})
}
