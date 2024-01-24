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

func TestMergeSysctlsSequence(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    sysctls:
      - net.ipv6.conf.all.disable_ipv6=1
`, `
services:
  test:
    sysctls:
      - net.ipv4.tcp_keepalive_time=300
`, `
services:
  test:
    image: foo
    sysctls:
      - net.ipv6.conf.all.disable_ipv6=1
      - net.ipv4.tcp_keepalive_time=300
`)
}

func TestMergeSysctlsMapping(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    sysctls:
      "net.ipv6.conf.all.disable_ipv6": 1
`, `
services:
  test:
    sysctls:
      "net.ipv4.tcp_keepalive_time": "300"
`, `
services:
  test:
    image: foo
    sysctls:
      - net.ipv6.conf.all.disable_ipv6=1
      - net.ipv4.tcp_keepalive_time=300
`)
}

func TestMergeSysctlsMixed(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    sysctls:
      "net.ipv6.conf.all.disable_ipv6": 1
`, `
services:
  test:
    sysctls:
      - net.ipv4.tcp_keepalive_time=300
`, `
services:
  test:
    image: foo
    sysctls:
      - net.ipv6.conf.all.disable_ipv6=1
      - net.ipv4.tcp_keepalive_time=300
`)
}

func TestMergeSysctlsNumbers(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    sysctls:
      "net.ipv4.tcp_keepalive_time": 1
`, `
services:
  test:
    sysctls:
      "net.ipv4.tcp_keepalive_time": 3
`, `
services:
  test:
    sysctls:
      - net.ipv4.tcp_keepalive_time=3
`)
}
