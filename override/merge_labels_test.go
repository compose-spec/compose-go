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

func Test_mergeYamlLabelsSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    labels:
      - FOO=BAR
`, `
services:
  test:
    labels:
      - QIX=ZOT
      - EMPTY=
      - NIL
`, `
services:
  test:
    image: foo
    labels:
      - FOO=BAR
      - QIX=ZOT
      - EMPTY=
      - NIL
`)
}

func Test_mergeYamlLabelsMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    labels:
      FOO: BAR
`, `
services:
  test:
    labels:
      EMPTY: ""
      NIL: null
      QIX: ZOT
`, `
services:
  test:
    image: foo
    labels:
      - FOO=BAR
      - EMPTY=
      - NIL
      - QIX=ZOT
`)
}

func Test_mergeYamlLabelsMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    labels:
      FOO: BAR
`, `
services:
  test:
    labels:
      - QIX=ZOT
`, `
services:
  test:
    image: foo
    labels:
      - FOO=BAR
      - QIX=ZOT
`)
}

func Test_mergeYamlLabelsNumber(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    labels:
      FOO: 1
`, `
services:
  test:
    labels:
      FOO: 3
`, `
services:
  test:
    labels:
      - FOO=3
`)
}
