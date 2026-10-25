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

package tests

// The tests in this file lock the `ports` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#ports
//
// Spec: "Exposes container ports."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestPortsShortSyntax(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    ports:
      - "80:8080"
      - "90:8090/udp"
      - 8600
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    ports:
      - "80:8080"
      - "90:8090/udp"
      - 8600
`)
	expect := func(p *types.Project) {
		ports := p.Services["foo"].Ports
		assert.Equal(t, len(ports), 3)
		assert.Equal(t, ports[0].Target, uint32(8080))
		assert.Equal(t, ports[0].Published, "80")
		assert.Equal(t, ports[1].Target, uint32(8090))
		assert.Equal(t, ports[1].Protocol, "udp")
		assert.Equal(t, ports[2].Target, uint32(8600))

		jobPorts := p.Jobs["foo"].Ports
		assert.Equal(t, len(jobPorts), 3)
		assert.Equal(t, jobPorts[0].Target, uint32(8080))
		assert.Equal(t, jobPorts[0].Published, "80")
		assert.Equal(t, jobPorts[1].Target, uint32(8090))
		assert.Equal(t, jobPorts[1].Protocol, "udp")
		assert.Equal(t, jobPorts[2].Target, uint32(8600))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestPortsLongSyntax(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    ports:
      - target: 80
        published: 8080
        protocol: tcp
        mode: host
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    ports:
      - target: 80
        published: 8080
        protocol: tcp
        mode: host
`)
	expect := func(p *types.Project) {
		ports := p.Services["foo"].Ports
		assert.Equal(t, len(ports), 1)
		assert.Equal(t, ports[0].Target, uint32(80))
		assert.Equal(t, ports[0].Published, "8080")
		assert.Equal(t, ports[0].Protocol, "tcp")
		assert.Equal(t, ports[0].Mode, "host")

		jobPorts := p.Jobs["foo"].Ports
		assert.Equal(t, len(jobPorts), 1)
		assert.Equal(t, jobPorts[0].Target, uint32(80))
		assert.Equal(t, jobPorts[0].Published, "8080")
		assert.Equal(t, jobPorts[0].Protocol, "tcp")
		assert.Equal(t, jobPorts[0].Mode, "host")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestPortsRange(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    ports:
      - "80-82:8080-8082"
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    ports:
      - "80-82:8080-8082"
`)
	ports := p.Services["foo"].Ports
	assert.Equal(t, len(ports), 3)
	assert.Equal(t, ports[0].Target, uint32(8080))
	assert.Equal(t, ports[0].Published, "80")
	assert.Equal(t, ports[1].Target, uint32(8081))
	assert.Equal(t, ports[1].Published, "81")
	assert.Equal(t, ports[2].Target, uint32(8082))
	assert.Equal(t, ports[2].Published, "82")

	jobPorts := p.Jobs["foo"].Ports
	assert.Equal(t, len(jobPorts), 3)
	assert.Equal(t, jobPorts[0].Target, uint32(8080))
	assert.Equal(t, jobPorts[0].Published, "80")
	assert.Equal(t, jobPorts[1].Target, uint32(8081))
	assert.Equal(t, jobPorts[1].Published, "81")
	assert.Equal(t, jobPorts[2].Target, uint32(8082))
	assert.Equal(t, jobPorts[2].Published, "82")
}

func TestNamedPort(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    ports:
      - name: http
        published: 8080
        target: 80
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    ports:
      - name: http
        published: 8080
        target: 80
`)
	assert.Equal(t, p.Services["foo"].Ports[0].Name, "http")
	assert.Equal(t, p.Jobs["foo"].Ports[0].Name, "http")
}

func TestAppProtocol(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    ports:
      - published: 8080
        target: 80
        protocol: tcp
        app_protocol: http
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    ports:
      - published: 8080
        target: 80
        protocol: tcp
        app_protocol: http
`)
	assert.Equal(t, p.Services["foo"].Ports[0].AppProtocol, "http")
	assert.Equal(t, p.Jobs["foo"].Ports[0].AppProtocol, "http")
}

// A port declared again by an override file is one port, whether the file
// repeats the short syntax or spells the same port in the long syntax, and
// whether it belongs to a service or to a job: the unicity of the entries is
// enforced once the short syntax is canonicalized.
func TestPortsRedeclaredByAnOverrideFileAreMerged(t *testing.T) {
	base := `
name: test
services:
  foo:
    image: alpine
    ports: ["8080:80", "9090:90"]
jobs:
  foo:
    image: alpine
    triggers:
      manual: true
    ports: ["8080:80", "9090:90"]
`
	port := func(published, target string) string {
		return `
      - mode: ingress
        protocol: tcp
        published: "` + published + `"
        target: ` + target
	}
	canonical := `
name: test
services:
  foo:
    image: alpine
    ports:` + port("8080", "80") + port("9090", "90") + `
jobs:
  foo:
    name: foo
    image: alpine
    triggers:
      manual: true
    ports:` + port("8080", "80") + port("9090", "90") + "\n"

	tests := []struct {
		name     string
		override string
	}{
		{"same short syntax", `
services:
  foo:
    ports: ["8080:80"]
jobs:
  foo:
    ports: ["8080:80"]
`},
		{"long syntax of a short one", `
services:
  foo:
    ports: [{target: 80, published: "8080"}]
jobs:
  foo:
    ports: [{target: 80, published: "8080"}]
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadFilesAs(t, []string{base, tt.override}, canonical)
		})
	}
}

// A job extending another one keeps the unicity of its entries like a service
// does: the port of the extending job replaces the one it inherits.
func TestPortsOfAnExtendingEntryReplaceTheInheritedOnes(t *testing.T) {
	loadsAs(t, `
name: test
services:
  base:
    image: alpine
    ports: ["8080:80"]
  foo:
    extends: base
    ports: [{target: 80, published: "8080", mode: host}]
jobs:
  base:
    image: alpine
    triggers:
      manual: true
    ports: ["8080:80"]
  foo:
    extends: base
    ports: [{target: 80, published: "8080", mode: host}]
`, `
name: test
services:
  base:
    image: alpine
    ports:
      - mode: ingress
        protocol: tcp
        published: "8080"
        target: 80
  foo:
    image: alpine
    ports:
      - mode: host
        protocol: tcp
        published: "8080"
        target: 80
jobs:
  base:
    name: base
    image: alpine
    triggers:
      manual: true
    ports:
      - mode: ingress
        protocol: tcp
        published: "8080"
        target: 80
  foo:
    name: foo
    image: alpine
    triggers:
      manual: true
    ports:
      - mode: host
        protocol: tcp
        published: "8080"
        target: 80
`)
}
