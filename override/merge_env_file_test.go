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

func TestMergeEnvFilesSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    env_file:
      - foo.env
`, `
services:
  test:
    env_file:
      - bar.env
      - baz.env
`, `
services:
  test:
    image: foo
    env_file:
      - foo.env
      - bar.env
      - baz.env
`)
}

func TestMergeEnvFilesString(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    env_file: foo.env
`, `
services:
  test:
    env_file: bar.env
`, `
services:
  test:
    image: foo
    env_file:
      - foo.env
      - bar.env
`)
}

func TestMergeEnvFilesMixed(t *testing.T) {
	l := `
services:
  test:
    env_file:
      - bar.env
`

	r := `
services:
  test:
    env_file: foo.env
`

	t.Run("SequenceThenString", func(t *testing.T) {
		assertMergeYaml(t, r, l, `
services:
  test:
    image: foo
    env_file:
      - bar.env
      - foo.env
`)
	})

	t.Run("StringThenSequence", func(t *testing.T) {
		assertMergeYaml(t, l, r, `
services:
  test:
    image: foo
    env_file:
      - foo.env
      - bar.env
`)
	})
}
