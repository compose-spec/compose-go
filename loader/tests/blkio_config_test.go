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

func TestBlkioConfig(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: busybox
    blkio_config:
      weight: 300
      weight_device:
        - path: /dev/sda
          weight: 400
      device_read_bps:
        - path: /dev/sda
          rate: 1024k
      device_write_bps:
        - path: /dev/sda
          rate: 1024
      device_read_iops:
        - path: /dev/sda
          rate: 100
      device_write_iops:
        - path: /dev/sda
          rate: 200
`)
	expect := func(p *types.Project) {
		bc := p.Services["foo"].BlkioConfig
		assert.Equal(t, bc.Weight, uint16(300))
		assert.Equal(t, bc.WeightDevice[0].Path, "/dev/sda")
		assert.Equal(t, bc.WeightDevice[0].Weight, uint16(400))
		assert.Equal(t, bc.DeviceReadBps[0].Path, "/dev/sda")
		assert.Equal(t, bc.DeviceReadBps[0].Rate, types.UnitBytes(1024*1024))
		assert.Equal(t, bc.DeviceWriteBps[0].Rate, types.UnitBytes(1024))
		assert.Equal(t, bc.DeviceReadIOps[0].Rate, types.UnitBytes(100))
		assert.Equal(t, bc.DeviceWriteIOps[0].Rate, types.UnitBytes(200))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
