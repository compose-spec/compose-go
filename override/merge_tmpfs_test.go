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

func TestMergeTmpfsSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    tmpfs:
      - /foo
`, `
services:
  test:
    tmpfs:
      - /bar
      - /baz
`, `
services:
  test:
    image: foo
    tmpfs:
      - /foo
      - /bar
      - /baz
`)
}

func TestMergeTmpfsString(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    tmpfs: /foo
`, `
services:
  test:
    tmpfs: /bar
`, `
services:
  test:
    image: foo
    tmpfs:
      - /foo
      - /bar
`)
}

func TestMergeTmpfsMixed(t *testing.T) {
	l := `
services:
  test:
    tmpfs:
      - /bar
`

	r := `
services:
  test:
    image: foo
    tmpfs: /foo
`

	t.Run("SequenceThenString", func(t *testing.T) {
		assertMergeYaml(t, r, l, `
services:
  test:
    image: foo
    annotations:
      - /foo
      - /bar
`)
	})

	t.Run("StringThenSequence", func(t *testing.T) {
		assertMergeYaml(t, l, r, `
services:
  test:
    image: foo
    annotations:
      - /bar
      - /foo
`)
	})
}
