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
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
)

func Test_mergeYamlServiceNetworkSequence(t *testing.T) {
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

func Test_mergeYamlServiceNetworksMapping(t *testing.T) {
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

func Test_mergeYamlServiceNetworksMixed(t *testing.T) {
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

func Test_mergeYamlNetworks(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
networks:
  network1:
    ipam:
      config:
        - subnet: 172.28.0.0/16
          ip_range: 172.28.5.0/24
          gateway: 172.28.5.254
          aux_addresses:
            host1: 172.28.1.5
            host2: 172.28.1.6
            host3: 172.28.1.7
      options:
        foo: bar
        baz: "0"
    labels:
      com.example.description: "Financial transaction network"
      com.example.department: "Finance"
      com.example.label-with-empty-value: ""
`, `
services:
  test:
    image: foo
networks:
  network1:
    ipam:
      config:
        - subnet: 172.28.0.0/16
          ip_range: 172.28.5.1/24
          gateway: 172.28.5.254
          aux_addresses:
            host1: 172.28.1.5
            host2: 172.28.1.4
            host4: 172.28.1.10
        - subnet: 172.28.10.0/16
          ip_range: 172.28.10.1/24
          gateway: 172.28.10.254
          aux_addresses:
            host1: 172.28.10.5
            host2: 172.28.10.4
            host3: 172.28.10.10
      options:
        bar: foo
        baz: "0"
    labels:
      - "com.example.department-new=New"
      - "com.example.description=Financial transaction network"
      - "com.example.label-with-empty-value="
  network2:
`, `
services:
  test:
    image: foo
networks:
  network1:
    ipam:
      config:
        - subnet: 172.28.0.0/16
          ip_range: 172.28.5.1/24
          gateway: 172.28.5.254
          aux_addresses:
            host1: 172.28.1.5
            host2: 172.28.1.4
            host3: 172.28.1.7
            host4: 172.28.1.10
        - subnet: 172.28.10.0/16
          ip_range: 172.28.10.1/24
          gateway: 172.28.10.254
          aux_addresses:
            host1: 172.28.10.5
            host2: 172.28.10.4
            host3: 172.28.10.10
      options:
        foo: bar
        bar: foo
        baz: "0"
    labels:
      - "com.example.department=Finance"
      - "com.example.description=Financial transaction network"
      - "com.example.label-with-empty-value="
      - "com.example.department-new=New"
  network2:
`)
}

func Test_mergeIPAMConfig_invalidTypes(t *testing.T) {
	tests := []struct {
		name    string
		c       any
		o       any
		wantErr string
	}{
		{
			name:    "c is not a list",
			c:       map[string]any{},
			o:       []any{},
			wantErr: "base value is not a list",
		},
		{
			name:    "o is not a list",
			c:       []any{},
			o:       map[string]any{},
			wantErr: "override value is not a list",
		},
		{
			name:    "both are not lists",
			c:       "invalid",
			o:       123,
			wantErr: "base value is not a list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := mergeIPAMConfig(tt.c, tt.o, tree.NewPath("networks", "ipam", "config"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
