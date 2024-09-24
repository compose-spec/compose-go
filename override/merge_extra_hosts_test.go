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

func TestMergeExtraHostsSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    extra_hosts:
      - example.com=1.2.3.4
`, `
services:
  test:
    extra_hosts:
      - localhost=127.0.0.1
      - example.com=1.2.3.4
      - example.com=4.3.2.1
`, `
services:
  test:
    image: foo
    extra_hosts:
      - example.com=1.2.3.4
      - localhost=127.0.0.1
      - example.com=4.3.2.1
`)
}

func TestMergeExtraHostsMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    extra_hosts:
      "example.com": "1.2.3.4"
`, `
services:
  test:
    extra_hosts:
      "localhost": "127.0.0.1"
      "example.com": ["1.2.3.4", "4.3.2.1"]
`, `
services:
  test:
    image: foo
    extra_hosts:
      - example.com=1.2.3.4
      - example.com=4.3.2.1
      - localhost=127.0.0.1
`)
}

func TestMergeExtraHostsMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    extra_hosts:
      "example.com": "1.2.3.4"
`, `
services:
  test:
    extra_hosts:
      - localhost=127.0.0.1
      - example.com=1.2.3.4
      - example.com=4.3.2.1
`, `
services:
  test:
    image: foo
    extra_hosts:
      - example.com=1.2.3.4
      - localhost=127.0.0.1
      - example.com=4.3.2.1
`)
}
