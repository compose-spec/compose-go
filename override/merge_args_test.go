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

func Test_mergeYamlArgsSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        - FOO=BAR
`, `
services:
  test:
    build:
      context: .
      args:
        - GIT_COMMIT=cdc3b19
        - EMPTY=
        - NIL
`, `
services:
  test:
    build:
      context: .
      args:
        - FOO=BAR
        - GIT_COMMIT=cdc3b19
        - EMPTY=
        - NIL
`)
}

func Test_mergeYamlArgsMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        FOO: BAR
`, `
services:
  test:
    build:
      context: .
      args:
        EMPTY: ""
        NIL: null
        QIX: ZOT
`, `
services:
  test:
    build:
      context: .
      args:
       - FOO=BAR
       - EMPTY=
       - NIL
       - QIX=ZOT
`)
}

func Test_mergeYamlArgsMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        FOO: BAR
`, `
services:
  test:
    build:
      args:
        - QIX=ZOT
`, `
services:
  test:
    build:
      context: .
      args:
        - FOO=BAR
        - QIX=ZOT
`)
}

func Test_mergeYamlArgsNumber(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    build:
      context: .
      args:
        FOO: 1
`, `
services:
  test:
    build:
      context: .
      args:
        FOO: 3
`, `
services:
  test:
    build:
      context: .
      args:
       - FOO=3
`)
}
