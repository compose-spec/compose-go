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

func TestMemLimitMixed(t *testing.T) {
	l := `
services:
  test:
    mem_limit: '256m'
`
	r := `
services:
  test:
    mem_limit: 1
`
	t.Run("StringThenInt", func(t *testing.T) {
		assertMergeYaml(t, r, l, `
services:
  test:
    mem_limit: 256m
`)
	})

	t.Run("IntThenString", func(t *testing.T) {
		assertMergeYaml(t, l, r, `
services:
  test:
    mem_limit: 1
`)
	})
}
