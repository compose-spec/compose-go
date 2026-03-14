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

func TestServiceNetworks(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    networks:
      some-network:
        aliases:
          - alias1
          - alias3
      other-network:
        ipv4_address: 172.16.238.10
        ipv6_address: "2001:3984:3989::10"
        mac_address: "02:42:72:98:65:08"
      other-other-network:
networks:
  some-network:
  other-network:
  other-other-network:
`)
	nets := p.Services["foo"].Networks
	assert.DeepEqual(t, nets["some-network"].Aliases, []string{"alias1", "alias3"})
	assert.Equal(t, nets["other-network"].Ipv4Address, "172.16.238.10")
	assert.Equal(t, nets["other-network"].Ipv6Address, "2001:3984:3989::10")
	assert.Equal(t, nets["other-network"].MacAddress, "02:42:72:98:65:08")
	assert.Assert(t, nets["other-other-network"] == nil)
}

func TestTopLevelNetworks(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
networks:
  some-network:
  other-network:
    driver: overlay
    driver_opts:
      foo: "bar"
      baz: 1
    ipam:
      driver: overlay
      config:
        - subnet: 172.28.0.0/16
          ip_range: 172.28.5.0/24
          gateway: 172.28.5.254
          aux_addresses:
            host1: 172.28.1.5
            host2: 172.28.1.6
            host3: 172.28.1.7
        - subnet: "2001:3984:3989::/64"
          gateway: "2001:3984:3989::1"
    labels:
      foo: bar
  external-network:
    external: true
  other-external-network:
    external:
      name: my-cool-network
    x-bar: baz
    x-foo: bar
`)

	expect := func(p *types.Project) {
		// Simple network
		assert.Equal(t, p.Networks["some-network"].Driver, "")

		// Network with driver and IPAM
		other := p.Networks["other-network"]
		assert.Equal(t, other.Driver, "overlay")
		assert.DeepEqual(t, other.DriverOpts, types.Options{"foo": "bar", "baz": "1"})
		assert.Equal(t, other.Ipam.Driver, "overlay")
		assert.Equal(t, len(other.Ipam.Config), 2)
		assert.Equal(t, other.Ipam.Config[0].Subnet, "172.28.0.0/16")
		assert.Equal(t, other.Ipam.Config[0].IPRange, "172.28.5.0/24")
		assert.Equal(t, other.Ipam.Config[0].Gateway, "172.28.5.254")
		assert.Equal(t, other.Ipam.Config[0].AuxiliaryAddresses["host1"], "172.28.1.5")
		assert.Equal(t, other.Ipam.Config[1].Subnet, "2001:3984:3989::/64")
		assert.DeepEqual(t, other.Labels, types.Labels{"foo": "bar"})

		// External networks
		assert.Equal(t, p.Networks["external-network"].External, types.External(true))
		assert.Equal(t, p.Networks["other-external-network"].External, types.External(true))
		assert.Equal(t, p.Networks["other-external-network"].Name, "my-cool-network")
	}
	expect(p)
	assert.DeepEqual(t, p.Networks["other-external-network"].Extensions, types.Extensions{
		"x-bar": "baz",
		"x-foo": "bar",
	})

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
