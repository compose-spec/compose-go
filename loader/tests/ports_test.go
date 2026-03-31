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
`)
	expect := func(p *types.Project) {
		ports := p.Services["foo"].Ports
		assert.Equal(t, len(ports), 3)
		assert.Equal(t, ports[0].Target, uint32(8080))
		assert.Equal(t, ports[0].Published, "80")
		assert.Equal(t, ports[1].Target, uint32(8090))
		assert.Equal(t, ports[1].Protocol, "udp")
		assert.Equal(t, ports[2].Target, uint32(8600))
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
`)
	expect := func(p *types.Project) {
		ports := p.Services["foo"].Ports
		assert.Equal(t, len(ports), 1)
		assert.Equal(t, ports[0].Target, uint32(80))
		assert.Equal(t, ports[0].Published, "8080")
		assert.Equal(t, ports[0].Protocol, "tcp")
		assert.Equal(t, ports[0].Mode, "host")
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
`)
	ports := p.Services["foo"].Ports
	assert.Equal(t, len(ports), 3)
	assert.Equal(t, ports[0].Target, uint32(8080))
	assert.Equal(t, ports[0].Published, "80")
	assert.Equal(t, ports[1].Target, uint32(8081))
	assert.Equal(t, ports[1].Published, "81")
	assert.Equal(t, ports[2].Target, uint32(8082))
	assert.Equal(t, ports[2].Published, "82")
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
`)
	assert.Equal(t, p.Services["foo"].Ports[0].Name, "http")
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
`)
	assert.Equal(t, p.Services["foo"].Ports[0].AppProtocol, "http")
}
