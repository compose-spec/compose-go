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

// The tests in this file lock the `healthcheck` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#healthcheck
//
// Spec: "`healthcheck` declares a check that's run to determine whether or not
// the service containers are "healthy"."

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
jobs:
  foo:
    triggers:
      manual: true
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
		jhc := p.Jobs["foo"].HealthCheck
		assert.DeepEqual(t, jhc.Test, types.HealthCheckTest{"CMD-SHELL", `echo "hello world"`})
		assert.Equal(t, *jhc.Interval, types.Duration(10*time.Second))
		assert.Equal(t, *jhc.Timeout, types.Duration(1*time.Second))
		assert.Equal(t, *jhc.Retries, uint64(5))
		assert.Equal(t, *jhc.StartPeriod, types.Duration(15*time.Second))
		assert.Equal(t, *jhc.StartInterval, types.Duration(5*time.Second))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// The healthcheck set by the image can be disabled, either by `test: [NONE]` or
// by `disable: true`, which the specification gives as the alternative.
// https://github.com/compose-spec/compose-spec/blob/main/workload_spec.md#healthcheck
//
// Spec: "`NONE` disables the healthcheck [...]. Alternatively, the healthcheck
// set by the image can be disabled by setting `disable: true`".
func TestHealthcheckDisable(t *testing.T) {
	p := load(t, `
name: test
services:
  by_flag:
    image: alpine
    healthcheck:
      disable: true
  by_test:
    image: alpine
    healthcheck:
      test: ["NONE"]
jobs:
  by_flag:
    image: alpine
    triggers:
      manual: true
    healthcheck:
      disable: true
`)
	expect := func(p *types.Project) {
		assert.Assert(t, p.Services["by_flag"].HealthCheck.Disable)
		assert.Assert(t, !p.Services["by_test"].HealthCheck.Disable)
		assert.DeepEqual(t, p.Services["by_test"].HealthCheck.Test, types.HealthCheckTest{"NONE"})
		assert.Assert(t, p.Jobs["by_flag"].HealthCheck.Disable)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// `disable` is a boolean: anything else is rejected, with the attribute in the
// error.
func TestHealthcheckDisableInvalid(t *testing.T) {
	err := loadErr(t, `
name: test
services:
  foo:
    image: alpine
    healthcheck:
      disable: [true]
`)
	assert.ErrorContains(t, err, "services.foo.healthcheck.disable")
}

// A healthcheck test entry that is not a string is rejected with an error
// naming the attribute, even when schema validation is skipped, instead of
// crashing the loader.
func TestHealthcheckTestWithNonStringEntry(t *testing.T) {
	err := loadUncheckedErr(t, serviceWith("healthcheck", "{test: [CMD, 1]}"))
	assert.ErrorContains(t, err, "services[foo].healthcheck.test")
	assert.ErrorContains(t, err, "invalid type int for healthcheck.test entry")
}
