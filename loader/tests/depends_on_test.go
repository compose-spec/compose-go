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

func TestDependsOnList(t *testing.T) {
	p := load(t, `
name: test
services:
  web:
    image: alpine
    depends_on:
      - db
      - redis
  db:
    image: postgres
  redis:
    image: redis
`)
	expect := func(p *types.Project) {
		deps := p.Services["web"].DependsOn
		assert.Equal(t, len(deps), 2)
		assert.Equal(t, deps["db"].Condition, types.ServiceConditionStarted)
		assert.Equal(t, deps["db"].Required, true)
		assert.Equal(t, deps["redis"].Condition, types.ServiceConditionStarted)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDependsOnObject(t *testing.T) {
	p := load(t, `
name: test
services:
  web:
    image: alpine
    depends_on:
      db:
        condition: service_healthy
        restart: true
      redis:
        condition: service_started
  db:
    image: postgres
  redis:
    image: redis
`)
	expect := func(p *types.Project) {
		deps := p.Services["web"].DependsOn
		assert.Equal(t, deps["db"].Condition, "service_healthy")
		assert.Equal(t, deps["db"].Restart, true)
		assert.Equal(t, deps["redis"].Condition, types.ServiceConditionStarted)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDependsOnXService(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: test
    depends_on:
      - x-foo
  x-foo:
    image: foo
`)
	assert.DeepEqual(t, p.Services["test"].DependsOn, types.DependsOnConfig{
		"x-foo": types.ServiceDependency{
			Condition: types.ServiceConditionStarted,
			Required:  true,
		},
	})
}
