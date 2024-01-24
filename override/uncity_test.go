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

	"gotest.tools/v3/assert"
)

func Test_EnvironmentUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    environment:
      - FOO=BAR
      - BAR=QIX
      - QIX=
      - ZOT
      - FOO=ZOT
      - QIX
      - ZOT=
`, `
services:
  test:
    image: foo
    environment:
      - FOO=ZOT
      - BAR=QIX
      - QIX
      - ZOT=
`)
}

func Test_VolumeUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    volumes:
      - .:/foo
      - foo:/bar
      - src:/foo
`, `
services:
  test:
    image: foo
    volumes:
      - src:/foo
      - foo:/bar
`)
}

func Test_PortsShortUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    ports:
      - "9080:80"
      - "9081:81"
      - "9080:80"
      - "5000"
      - "6060:6060/udp"
      - "9080:6060/udp"
`, `
services:
  test:
    image: foo
    ports:
      - "9080:80"
      - "9081:81"
      - "5000"
      - "6060:6060/udp"
      - "9080:6060/udp"
`)
}

func Test_PortsLongtUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    ports:
      - target: 80
        host_ip: 127.0.0.1
        published: "8080"
        protocol: tcp
        mode: host
      - target: 81
        published: "8080"
        protocol: tcp
      - target: 80
        host_ip: 127.0.0.2
        published: "8080"
        protocol: tcp
      - target: 81
        published: "8080"
        protocol: tcp
`, `
services:
  test:
    image: foo
    ports:
      - target: 80
        host_ip: 127.0.0.1
        published: "8080"
        protocol: tcp
        mode: host
      - target: 81
        published: "8080"
        protocol: tcp
      - target: 80
        host_ip: 127.0.0.2
        published: "8080"
        protocol: tcp
`)
}

func assertUnicity(t *testing.T, before string, expected string) {
	got, err := EnforceUnicity(unmarshal(t, before))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, expected))
}
