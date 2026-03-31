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
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestDeployModeReplicas(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    deploy:
      mode: replicated
      replicas: 6
      labels: [FOO=BAR]
      endpoint_mode: dnsrr
`)

	expect := func(p *types.Project) {
		d := p.Services["foo"].Deploy
		assert.Equal(t, d.Mode, "replicated")
		assert.Equal(t, *d.Replicas, 6)
		assert.DeepEqual(t, d.Labels, types.Labels{"FOO": "BAR"})
		assert.Equal(t, d.EndpointMode, "dnsrr")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDeployUpdateRollbackConfig(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    deploy:
      update_config:
        parallelism: 3
        delay: 10s
        failure_action: continue
        monitor: 60s
        max_failure_ratio: 0.3
        order: start-first
      rollback_config:
        parallelism: 3
        delay: 10s
        failure_action: continue
        monitor: 60s
        max_failure_ratio: 0.3
        order: start-first
`)

	expect := func(p *types.Project) {
		uc := p.Services["foo"].Deploy.UpdateConfig
		assert.Equal(t, *uc.Parallelism, uint64(3))
		assert.Equal(t, uc.Delay, types.Duration(10*time.Second))
		assert.Equal(t, uc.FailureAction, "continue")
		assert.Equal(t, uc.Monitor, types.Duration(60*time.Second))
		assert.Equal(t, uc.MaxFailureRatio, float32(0.3))
		assert.Equal(t, uc.Order, "start-first")

		rc := p.Services["foo"].Deploy.RollbackConfig
		assert.Equal(t, *rc.Parallelism, uint64(3))
		assert.Equal(t, rc.Order, "start-first")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDeployResources(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    deploy:
      resources:
        limits:
          cpus: '0.001'
          memory: 50M
        reservations:
          cpus: '0.0001'
          memory: 20M
          generic_resources:
            - discrete_resource_spec:
                kind: 'gpu'
                value: 2
            - discrete_resource_spec:
                kind: 'ssd'
                value: 1
`)

	expect := func(p *types.Project) {
		r := p.Services["foo"].Deploy.Resources
		assert.Equal(t, r.Limits.NanoCPUs, types.NanoCPUs(0.001))
		assert.Equal(t, r.Limits.MemoryBytes, types.UnitBytes(50*1024*1024))
		assert.Equal(t, r.Reservations.NanoCPUs, types.NanoCPUs(0.0001))
		assert.Equal(t, r.Reservations.MemoryBytes, types.UnitBytes(20*1024*1024))
		assert.Equal(t, len(r.Reservations.GenericResources), 2)
		assert.Equal(t, r.Reservations.GenericResources[0].DiscreteResourceSpec.Kind, "gpu")
		assert.Equal(t, r.Reservations.GenericResources[0].DiscreteResourceSpec.Value, int64(2))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDeployRestartPolicy(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    deploy:
      restart_policy:
        condition: on-failure
        delay: 5s
        max_attempts: 3
        window: 120s
`)

	expect := func(p *types.Project) {
		rp := p.Services["foo"].Deploy.RestartPolicy
		assert.Equal(t, rp.Condition, "on-failure")
		assert.Equal(t, *rp.Delay, types.Duration(5*time.Second))
		assert.Equal(t, *rp.MaxAttempts, uint64(3))
		assert.Equal(t, *rp.Window, types.Duration(2*time.Minute))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDeployPlacement(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    deploy:
      placement:
        constraints: [node=foo]
        max_replicas_per_node: 5
        preferences:
          - spread: node.labels.az
`)

	expect := func(p *types.Project) {
		pl := p.Services["foo"].Deploy.Placement
		assert.DeepEqual(t, pl.Constraints, []string{"node=foo"})
		assert.Equal(t, pl.MaxReplicas, uint64(5))
		assert.Equal(t, pl.Preferences[0].Spread, "node.labels.az")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
