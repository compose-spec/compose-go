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

func TestDeviceMapping(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    devices:
      - /dev/source:/dev/target:permissions
      - /dev/single
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["test"].Devices, []types.DeviceMapping{
			{Source: "/dev/source", Target: "/dev/target", Permissions: "permissions"},
			{Source: "/dev/single", Target: "/dev/single", Permissions: "rwm"},
		})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDeviceMappingLongSyntax(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    devices:
      - source: /dev/source
        target: /dev/target
        permissions: permissions
`)
	assert.DeepEqual(t, p.Services["test"].Devices, []types.DeviceMapping{
		{Source: "/dev/source", Target: "/dev/target", Permissions: "permissions"},
	})
}

func TestDeviceReservation(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              capabilities: ["gpu"]
              count: all
              options:
                q_bits: 42
`)
	devs := p.Services["test"].Deploy.Resources.Reservations.Devices
	assert.Equal(t, len(devs), 1)
	assert.Equal(t, devs[0].Driver, "nvidia")
	assert.DeepEqual(t, devs[0].Capabilities, []string{"gpu"})
	assert.Equal(t, devs[0].Count, types.DeviceCount(-1))
	assert.Equal(t, devs[0].Options["q_bits"], "42")
}
