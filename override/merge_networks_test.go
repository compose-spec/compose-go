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

func Test_mergeYamlNetworkSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    networks:
      - front-network
      - back-network
`, `
services:
  test:
    networks:
      - front-network
`, `
services:
  test:
    image: foo
    networks:
      front-network:
      back-network:
`)
}

func Test_mergeYamlNetworksMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    networks:
      network1:
        aliases:
          - alias1
          - alias2
        link_local_ips:
          - 57.123.22.11
          - 57.123.22.13
      network2:
        aliases:
          - alias1
          - alias3
`, `
services:
  test:
    networks:
      network1:
        aliases:
          - alias3
          - alias1
        link_local_ips:
          - 57.123.22.12
          - 57.123.22.13
`, `
services:
  test:
    image: foo
    networks:
      network1:
        aliases:
          - alias1
          - alias2
          - alias3
        link_local_ips:
          - 57.123.22.11
          - 57.123.22.13
          - 57.123.22.12
      network2:
        aliases:
          - alias1
          - alias3
`)
}

func Test_mergeYamlNetworkstMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    networks:
      - front-network
      - back-network
      - network1
`, `
services:
  test:
    image: foo
    networks:
      network1:
        aliases:
          - alias1
          - alias2
        link_local_ips:
          - 57.123.22.11
          - 57.123.22.13
      network2:
        aliases:
          - alias1
          - alias3
`, `
services:
  test:
    image: foo
    networks:
      front-network:
      back-network:
      network1:
        aliases:
          - alias1
          - alias2
        link_local_ips:
          - 57.123.22.11
          - 57.123.22.13
      network2:
        aliases:
          - alias1
          - alias3
`)
}
