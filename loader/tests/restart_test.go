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

// The tests in this file lock the `restart` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#restart
//
// Spec: "`restart` defines the policy that the platform applies on container
// termination."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestRestart(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    restart: always
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Restart, "always")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// When both are set, restart and deploy.restart_policy must agree on the
// values each of them defines: condition and max_attempts are compared
// after mapping them to the engine vocabulary, so `restart: always` and
// `condition: any` are the same value, while an attribute left unset on
// one side is a complement, not a conflict.
func TestRestartConsistentWithDeployRestartPolicy(t *testing.T) {
	model := func(restart, policy string) string {
		return "name: test\nservices:\n  foo:\n    image: alpine\n    restart: " + restart +
			"\n    deploy:\n      restart_policy:\n" + policy
	}

	t.Run("same value in different vocabularies", func(t *testing.T) {
		_, err := loadConsistent(t, model("always", "        condition: any\n"))
		assert.NilError(t, err)
	})

	t.Run("no and none are the same policy", func(t *testing.T) {
		_, err := loadConsistent(t, model("no", "        condition: none\n"))
		assert.NilError(t, err)
	})

	t.Run("matching condition and attempts", func(t *testing.T) {
		_, err := loadConsistent(t, model("on-failure:3",
			"        condition: on-failure\n        max_attempts: 3\n"))
		assert.NilError(t, err)
	})

	t.Run("deploy sets what restart leaves unset", func(t *testing.T) {
		_, err := loadConsistent(t, model("on-failure", "        max_attempts: 3\n"))
		assert.NilError(t, err)
	})

	t.Run("restart_policy without condition or attempts is complementary", func(t *testing.T) {
		_, err := loadConsistent(t, model("always", "        delay: 5s\n"))
		assert.NilError(t, err)
	})

	t.Run("distinct conditions", func(t *testing.T) {
		_, err := loadConsistent(t, model("unless-stopped", "        condition: on-failure\n"))
		assert.ErrorContains(t, err, "services.foo")
		assert.ErrorContains(t, err, "can't set distinct values on 'restart' and 'deploy.restart_policy.condition'")
	})

	t.Run("condition contradicting the service policy", func(t *testing.T) {
		_, err := loadConsistent(t, model("on-failure:3", "        condition: any\n"))
		assert.ErrorContains(t, err, "services.foo")
		assert.ErrorContains(t, err, "can't set distinct values on 'restart' and 'deploy.restart_policy.condition'")
	})

	t.Run("distinct max_attempts", func(t *testing.T) {
		_, err := loadConsistent(t, model("on-failure:3",
			"        condition: on-failure\n        max_attempts: 5\n"))
		assert.ErrorContains(t, err, "services.foo")
		assert.ErrorContains(t, err, "can't set distinct values on 'restart' and 'deploy.restart_policy.max_attempts'")
	})
}

func TestRestartNotAllowedOnJob(t *testing.T) {
	// jobs run to completion: a restart policy is a service-only attribute
	_, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
jobs:
  foo:
    image: alpine
    restart: always
    triggers:
      manual: true
`)}},
		Environment: map[string]string{},
	})
	assert.ErrorContains(t, err, "restart")
}
