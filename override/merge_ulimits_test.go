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
          soft: 20000
          hard: 65535
`)
}

// Test_mergeYamlUlimitsSyntaxCombinations covers every combination of the
// short (single limit) and long (soft/hard mapping) ulimit syntaxes between
// base and override: mappings merge key by key, while a syntax mismatch makes
// the override value win as a whole.
func Test_mergeYamlUlimitsSyntaxCombinations(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    ulimits:
      short_over_short: 100
      long_over_short: 100
      short_over_long:
          soft: 100
          hard: 200
      long_over_long:
          soft: 100
          hard: 200
`, `
services:
  test:
    image: foo
    ulimits:
      short_over_short: 500
      long_over_short:
          soft: 500
      short_over_long: 500
      long_over_long:
          hard: 500
`, `
services:
  test:
    image: foo
    ulimits:
      short_over_short: 500
      long_over_short:
          soft: 500
      short_over_long: 500
      long_over_long:
          soft: 100
          hard: 500
`)
}
