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

func Test_mergeYamlDNSSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    dns:
      - 8.8.8.8
    dns_opt:
      - use-vc
    dns_search:
      - dc1.example.com
`, `
services:
  test:
    dns:
      - 9.9.9.9
    dns_opt:
      - no-tld-query
    dns_search:
      - dc2.example.com
`, `
services:
  test:
    image: foo
    dns:
      - 8.8.8.8
      - 9.9.9.9
    dns_opt:
      - use-vc
      - no-tld-query
    dns_search:
      - dc1.example.com
      - dc2.example.com
`)
}

func Test_mergeYamlDNSMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    dns:
      - 8.8.8.8
      - 10.10.10.10
      - 9.9.9.9
    dns_opt:
      - use-vc
      - no-tld-query
    dns_search:
      - dc1.example.com
      - dc2.example.com
`, `
services:
  test:
    dns: 9.9.9.9
    dns_opt:
      - no-tld-query
    dns_search: dc2.example.com
`, `
services:
  test:
    image: foo
    dns:
      - 8.8.8.8
      - 10.10.10.10
      - 9.9.9.9
    dns_opt:
      - use-vc
      - no-tld-query
    dns_search:
      - dc1.example.com
      - dc2.example.com
`)
}
