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

func TestNetworkIPAM(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
networks:
  net1:
    ipam:
      driver: default
      options:
        com.docker.network.ipam.foo: bar
        com.docker.network.ipam.baz: "1"
      config:
        - subnet: 172.28.0.0/16
          gateway: 172.28.0.1
`)

	expect := func(p *types.Project) {
		ipam := p.Networks["net1"].Ipam
		assert.Equal(t, ipam.Driver, "default")
		assert.DeepEqual(t, ipam.Options, types.Options{
			"com.docker.network.ipam.foo": "bar",
			"com.docker.network.ipam.baz": "1",
		})
		assert.Equal(t, len(ipam.Config), 1)
		assert.Equal(t, ipam.Config[0].Subnet, "172.28.0.0/16")
		assert.Equal(t, ipam.Config[0].Gateway, "172.28.0.1")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
