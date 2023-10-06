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

// override using the same logging driver will override driver options
func Test_mergeYamlLoggingSameDriver(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    logging:
      driver: syslog
      options:
        syslog-address: "tcp://192.168.0.42:123"
`, `
services:
  test:
    logging:
      driver: syslog
      options:
        syslog-address: "tcp://127.0.0.1:123"
`, `
services:
  test:
    image: foo
    logging:
      driver: syslog
      options:
        syslog-address: "tcp://127.0.0.1:123"
`)
}

// check override with a distinct logging driver fully overrides driver options
func Test_mergeYamlLoggingDistinctDriver(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    logging:
      driver: local
      options:
        max-size: "10m"
`, `
services:
  test:
    logging:
      driver: syslog
      options:
        syslog-address: "tcp://127.0.0.1:123"
`, `
services:
  test:
    image: foo
    logging:
      driver: syslog
      options:
        syslog-address: "tcp://127.0.0.1:123"
`)
}

// check override without an explicit driver set (defaults to local driver)
func Test_mergeYamlLoggingImplicitDriver(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    logging:
      options:
        max-size: "10m"
`, `
services:
  test:
    logging:
      options:
        max-file: 3
`, `
services:
  test:
    image: foo
    logging:
      options:
        max-size: "10m"
        max-file: 3
`)
}
