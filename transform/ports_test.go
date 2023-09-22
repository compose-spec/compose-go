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

package transform

import "testing"

func TestExpandServicePorts(t *testing.T) {
	t.Parallel()
	t.Run("simple int", func(t *testing.T) {
		assertExpand(t, `
services:
  test:
    ports:
      - 9090
`, `
services:
  test:
    ports:
      - mode: ingress
        host_ip: "" 
        published: ""
        target: 9090
        protocol: tcp
`)
	})
	t.Run("simple port binding", func(t *testing.T) {
		assertExpand(t, `
services:
  test:
    ports:
      - 127.0.0.1:8080:80/tcp
`, `
services:
  test:
    ports:
      - mode: ingress
        host_ip: 127.0.0.1
        published: "8080"
        target: 80
        protocol: tcp
`)
	})
	t.Run("port range", func(t *testing.T) {
		assertExpand(t, `
services:
  test:
    ports:
      - 127.0.0.1:8080-8082:80-82/tcp
`, `
services:
  test:
    ports:
      - mode: ingress
        host_ip: 127.0.0.1
        published: "8080"
        target: 80
        protocol: tcp
      - mode: ingress
        host_ip: 127.0.0.1
        published: "8081"
        target: 81
        protocol: tcp
      - mode: ingress
        host_ip: 127.0.0.1
        published: "8082"
        target: 82
        protocol: tcp
`)
	})

}
