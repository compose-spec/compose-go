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

func Test_mergeYamlUlimits(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    ulimits:
      nofile: 
          soft: 20000
          hard: 40000
      nproc: 65535
      locks: 
          soft: 20000
          hard: 40000
`, `
services:
  test:
    image: foo
    ulimits:
      nofile: 
          soft: 10000
          hard: 40000
      nproc: 
          soft: 65535
      locks:
          hard: 65535
`, `
services:
  test:
    image: foo
    ulimits:
      nofile: 
          soft: 10000
          hard: 40000
      nproc: 
          soft: 65535
      locks:
          hard: 65535
`)
}
