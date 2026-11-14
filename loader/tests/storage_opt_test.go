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

// The tests in this file lock the `storage_opt` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#storage_opt
//
// Spec: "`storage_opt` defines storage driver options for a service."
//
// The attribute is a mapping of option names to values: like `driver_opts`
// and `logging.options`, its keys are free but its shape is declared, so that
// a runtime can tell the keys the specification allows from the ones it does
// not (docker/compose#14505).

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestStorageOpt(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    storage_opt:
      size: "20G"
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    storage_opt:
      size: "20G"
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].StorageOpt, map[string]string{"size": "20G"})
		assert.DeepEqual(t, p.Jobs["foo"].StorageOpt, map[string]string{"size": "20G"})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// Each option of `storage_opt` takes a string or a number, which is kept as the
// text the storage driver receives; a mapping or a list under an option is not
// a storage driver option and is rejected with the path of the option.
func TestStorageOptValues(t *testing.T) {
	for _, tt := range []struct {
		name, value, want string
	}{
		{"a size with a unit", `"1G"`, `"1G"`},
		{"a number of bytes", `1073741824`, `"1073741824"`},
		{"an option without a value", `null`, `""`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			loadsAs(t, serviceWith("storage_opt", "{size: "+tt.value+"}"), `
name: test
services:
  foo:
    image: alpine
    storage_opt:
      size: `+tt.want+`
`)
		})
	}

	for _, tt := range []struct {
		name, value string
	}{
		{"a boolean", `true`},
		{"a list", `[1G]`},
		{"a mapping", `{max: 1G}`},
	} {
		t.Run("rejects "+tt.name, func(t *testing.T) {
			err := loadErr(t, serviceWith("storage_opt", "{size: "+tt.value+"}"))
			assert.ErrorContains(t, err, "services.foo.storage_opt.size")
		})
	}
}
