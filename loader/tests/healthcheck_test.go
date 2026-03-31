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

func TestHealthcheck(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    healthcheck:
      test: echo "hello world"
      interval: 10s
      timeout: 1s
      retries: 5
      start_period: 15s
      start_interval: 5s
`)
	expect := func(p *types.Project) {
		hc := p.Services["foo"].HealthCheck
		assert.DeepEqual(t, hc.Test, types.HealthCheckTest{"CMD-SHELL", `echo "hello world"`})
		assert.Equal(t, *hc.Interval, types.Duration(10*time.Second))
		assert.Equal(t, *hc.Timeout, types.Duration(1*time.Second))
		assert.Equal(t, *hc.Retries, uint64(5))
		assert.Equal(t, *hc.StartPeriod, types.Duration(15*time.Second))
		assert.Equal(t, *hc.StartInterval, types.Duration(5*time.Second))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
