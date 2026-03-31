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

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestProvider(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    provider:
      type: foo
      options:
        bar: zot
        strings:
          - foo
          - bar
        numbers:
          - 12
          - 34
        booleans:
          - true
          - false
`)
	assert.DeepEqual(t, p.Services["test"].Provider, &types.ServiceProviderConfig{
		Type: "foo",
		Options: types.MultiOptions{
			"bar":      []string{"zot"},
			"strings":  []string{"foo", "bar"},
			"numbers":  []string{"12", "34"},
			"booleans": []string{"true", "false"},
		},
	})
}
