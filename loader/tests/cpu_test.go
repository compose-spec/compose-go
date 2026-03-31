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

func TestCPUFields(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    cpu_count: 4
    cpu_percent: 50
    cpu_shares: 1024
    cpu_quota: 50000
    cpu_period: 100000
    cpu_rt_period: 1000000
    cpu_rt_runtime: 950000
    cpus: 1.5
    cpuset: "0,1"
`)
	expect := func(p *types.Project) {
		s := p.Services["foo"]
		assert.Equal(t, s.CPUCount, int64(4))
		assert.Equal(t, s.CPUPercent, float32(50))
		assert.Equal(t, s.CPUShares, int64(1024))
		assert.Equal(t, s.CPUQuota, int64(50000))
		assert.Equal(t, s.CPUPeriod, int64(100000))
		assert.Equal(t, s.CPURTPeriod, int64(1000000))
		assert.Equal(t, s.CPURTRuntime, int64(950000))
		assert.Equal(t, s.CPUS, float32(1.5))
		assert.Equal(t, s.CPUSet, "0,1")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestMemoryFields(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mem_limit: 512m
    mem_reservation: 256m
    mem_swappiness: 60
    memswap_limit: 1g
    shm_size: 64m
`)
	expect := func(p *types.Project) {
		s := p.Services["foo"]
		assert.Equal(t, s.MemLimit, types.UnitBytes(512*1024*1024))
		assert.Equal(t, s.MemReservation, types.UnitBytes(256*1024*1024))
		assert.Equal(t, s.MemSwappiness, types.UnitBytes(60))
		assert.Equal(t, s.MemSwapLimit, types.UnitBytes(1024*1024*1024))
		assert.Equal(t, s.ShmSize, types.UnitBytes(64*1024*1024))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
