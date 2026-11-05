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
//
// It also locks its relation to `deploy.restart_policy`:
//   https://github.com/compose-spec/compose-spec/blob/main/deploy.md#restart_policy
//
// Spec: "`restart_policy` configures if and how to restart containers when they
// exit. If `restart_policy` is not set, Compose considers the `restart` field
// set by the service configuration."

import (
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
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

// `restart` and `deploy.restart_policy` are two spellings of one policy
// (`condition` is its name, `max_attempts` its retry count, as in
// `restart: on-failure:3`): each may complete the other, and the model carries
// the outcome in `restart`. An attribute that is not set says nothing, so a
// `restart_policy` with a `delay` alone must leave `restart` as it is
// (https://github.com/docker/compose/issues/14383).
func TestRestartPolicyIsCarriedIntoRestart(t *testing.T) {
	for _, tc := range []struct {
		name    string
		service string
		want    string
	}{
		{"condition on its own", "deploy: {restart_policy: {condition: on-failure}}", "on-failure"},
		{"condition none", "deploy: {restart_policy: {condition: none}}", "no"},
		{"condition any", "deploy: {restart_policy: {condition: any}}", "always"},
		{"condition unless-stopped", "deploy: {restart_policy: {condition: unless-stopped}}", "unless-stopped"},
		{"condition and max_attempts", "deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}", "on-failure:3"},
		{"max_attempts given as a string", "deploy: {restart_policy: {condition: on-failure, max_attempts: '3'}}", "on-failure:3"},
		{"max_attempts completes restart", "restart: on-failure\n    deploy: {restart_policy: {max_attempts: 3}}", "on-failure:3"},
		{"same retry count on both", "restart: on-failure:3\n    deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}", "on-failure:3"},
		{"same name on both, any is always", "restart: always\n    deploy: {restart_policy: {condition: any}}", "always"},
		{"restart keeps its retry count", "restart: on-failure:3\n    deploy: {restart_policy: {condition: on-failure}}", "on-failure:3"},
		{"delay alone leaves restart", "restart: always\n    deploy: {restart_policy: {delay: 5s}}", "always"},
		{"delay alone leaves the retry count", "restart: on-failure:3\n    deploy: {restart_policy: {delay: 5s, window: 1m}}", "on-failure:3"},
		{"max_attempts of zero", "deploy: {restart_policy: {condition: on-failure, max_attempts: 0}}", "on-failure:0"},
		// zero retries is no limit (`on-failure:0` has none either): it does not
		// contradict a policy which has no retry count, and gives none to carry
		{"zero retries completes on-failure", "restart: on-failure\n    deploy: {restart_policy: {max_attempts: 0}}", "on-failure:0"},
		{"zero retries with any is no limit", "deploy: {restart_policy: {condition: any, max_attempts: 0}}", "always"},
		{"zero retries against a policy that retries forever", "restart: always\n    deploy: {restart_policy: {max_attempts: 0}}", "always"},
		{"zero retries against a policy that never restarts", "restart: 'no'\n    deploy: {restart_policy: {max_attempts: 0}}", "no"},
		{"a retry count with any is not a restart value", "deploy: {restart_policy: {condition: any, max_attempts: 3}}", ""},
		{"a retry count with none is not a restart value", "deploy: {restart_policy: {condition: none, max_attempts: 3}}", ""},
		{"a malformed retry count is left to the platform", "restart: on-failure:abc\n    deploy: {restart_policy: {condition: on-failure}}", "on-failure:abc"},
		{"delay alone sets no restart", "deploy: {restart_policy: {delay: 5s}}", ""},
		{"an unknown condition is left to the platform", "restart: always\n    deploy: {restart_policy: {condition: sometimes}}", "always"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := captureWarnings(t)
			p, err := loadConsistent(t, restartService(tc.service))
			assert.NilError(t, err)
			assert.Equal(t, p.Services["foo"].Restart, tc.want)
			// completing one attribute with the other is not a disagreement
			assert.DeepEqual(t, warnings(), []string(nil))

			// the other attributes of restart_policy stay where they are, and
			// the outcome is stable once written out and loaded again
			assert.Equal(t, p.Services["foo"].Deploy.RestartPolicy != nil, true)
			out, err := p.MarshalYAML()
			assert.NilError(t, err)
			again, err := loadConsistent(t, string(out))
			assert.NilError(t, err)
			assert.Equal(t, again.Services["foo"].Restart, tc.want)
		})
	}
}

// Where `restart` and `deploy.restart_policy` both say something different,
// the spec gives the precedence ("if `restart_policy` is not set, Compose
// considers the `restart` field"), and Docker Compose reads `restart_policy`
// alone when it is set: it is not an error, `deploy.restart_policy` wins and is
// written into `restart`, with a warning that names both values. `any` is
// `always`, `none` is `no`, and a retry count only has a meaning for
// `on-failure`.
func TestRestartPolicyWinsOverRestart(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"another policy", `
restart: always
deploy:
  restart_policy: {condition: on-failure}
`, `
restart: on-failure
deploy:
  restart_policy: {condition: on-failure}
`},
		// `any` cannot say "unless stopped": it is the `always` that Docker
		// Compose already applies, so the model says it too
		{"any is not unless-stopped", `
restart: unless-stopped
deploy:
  restart_policy: {condition: any}
`, `
restart: always
deploy:
  restart_policy: {condition: any}
`},
		{"any against no", `
restart: "no"
deploy:
  restart_policy: {condition: any}
`, `
restart: always
deploy:
  restart_policy: {condition: any}
`},
		{"on-failure against no", `
restart: "no"
deploy:
  restart_policy: {condition: on-failure}
`, `
restart: on-failure
deploy:
  restart_policy: {condition: on-failure}
`},
		{"none against a policy that restarts", `
restart: always
deploy:
  restart_policy: {condition: none}
`, `
restart: "no"
deploy:
  restart_policy: {condition: none}
`},
		{"another policy drops the retry count of restart", `
restart: on-failure:3
deploy:
  restart_policy: {condition: any}
`, `
restart: always
deploy:
  restart_policy: {condition: any}
`},
		{"another retry count", `
restart: on-failure:3
deploy:
  restart_policy: {condition: on-failure, max_attempts: 5}
`, `
restart: on-failure:5
deploy:
  restart_policy: {condition: on-failure, max_attempts: 5}
`},
		{"no limit against a retry count", `
restart: on-failure:3
deploy:
  restart_policy: {condition: on-failure, max_attempts: 0}
`, `
restart: on-failure:0
deploy:
  restart_policy: {condition: on-failure, max_attempts: 0}
`},
		// `restart` has no way to say a retry count for a policy that does not
		// retry: the count is not carried, the policy of `restart` is kept
		{"a retry count against a policy that retries forever", `
restart: always
deploy:
  restart_policy: {max_attempts: 2}
`, `
restart: always
deploy:
  restart_policy: {max_attempts: 2}
`},
		{"a retry count against a policy that never restarts", `
restart: "no"
deploy:
  restart_policy: {max_attempts: 3}
`, `
restart: "no"
deploy:
  restart_policy: {max_attempts: 3}
`},
		{"a retry count with any", `
restart: always
deploy:
  restart_policy: {condition: any, max_attempts: 3}
`, `
restart: always
deploy:
  restart_policy: {condition: any, max_attempts: 3}
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := captureWarnings(t)
			loadsNormalizedAs(t, tc.input, tc.want)
			// one warning, which names the service and both values
			got := warnings()
			assert.Equal(t, len(got), 1, "%v", got)
			assert.Assert(t, strings.Contains(got[0], "services.foo"), got[0])
			assert.Assert(t, strings.Contains(got[0], "'deploy.restart_policy' takes precedence"), got[0])
		})
	}
}

// Two spellings of the same policy do not disagree, whatever their notation,
// and nothing is said: `any` is `always`, `none` is `no`, and a policy with no
// count agrees with the count of the other.
func TestRestartPolicyAgreeingWithRestartDoesNotWarn(t *testing.T) {
	for _, tc := range []struct{ name, restart, policy, want string }{
		{"any is always", "always", "{condition: any}", "always"},
		{"none is no", `"no"`, "{condition: none}", "no"},
		{"same name", "unless-stopped", "{condition: unless-stopped}", "unless-stopped"},
		{"same retry count", "on-failure:3", "{condition: on-failure, max_attempts: 3}", "on-failure:3"},
		{"a count completes restart", "on-failure", "{max_attempts: 3}", "on-failure:3"},
		{"restart keeps its count", "on-failure:3", "{condition: on-failure}", "on-failure:3"},
		{"no limit agrees with a policy that has no count", "always", "{max_attempts: 0}", "always"},
		{"delay alone says nothing", "unless-stopped", "{delay: 5s}", "unless-stopped"},
		{"no limit agrees with unless-stopped", "unless-stopped", "{max_attempts: 0}", "unless-stopped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := captureWarnings(t)
			p, err := loadConsistent(t, restartService("restart: "+tc.restart+"\n    deploy: {restart_policy: "+tc.policy+"}"))
			assert.NilError(t, err)
			assert.Equal(t, p.Services["foo"].Restart, tc.want)
			assert.DeepEqual(t, warnings(), []string(nil))
		})
	}
}

// Skipping the consistency check does not change how the two attributes are
// reconciled: it is no longer a consistency matter, the loader never rejects
// the combination.
func TestRestartPolicyWinsWithoutConsistencyCheck(t *testing.T) {
	captureWarnings(t)
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(restartService(
			"restart: unless-stopped\n    deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}"))}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["foo"].Restart, "on-failure:3")
	assert.Equal(t, p.Services["foo"].Deploy.RestartPolicy.Condition, "on-failure")
	assert.Equal(t, *p.Services["foo"].Deploy.RestartPolicy.MaxAttempts, uint64(3))
}

// The two attributes are reconciled on the merged model, not file by file: a
// service that is completed by an override file or by `extends` is judged on
// what it says in the end, so a value that the override replaces never
// contradicts `restart`.
func TestRestartPolicyIsReconciledAfterMerge(t *testing.T) {
	loadFiles := func(files ...string) (*types.Project, error) {
		var configFiles []types.ConfigFile
		for i, content := range files {
			configFiles = append(configFiles, types.ConfigFile{Filename: "compose-" + string(rune('a'+i)) + ".yml", Content: []byte(content)})
		}
		return loader.LoadWithContext(t.Context(), types.ConfigDetails{ConfigFiles: configFiles, Environment: map[string]string{}})
	}

	t.Run("an override file changes the condition", func(t *testing.T) {
		p, err := loadFiles(restartService("deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}"),
			"services:\n  foo:\n    deploy: {restart_policy: {condition: any}}\n")
		assert.NilError(t, err)
		assert.Equal(t, p.Services["foo"].Restart, "")
	})

	t.Run("an override file completes restart", func(t *testing.T) {
		p, err := loadFiles(restartService("restart: on-failure:3"),
			"services:\n  foo:\n    deploy: {restart_policy: {delay: 5s}}\n")
		assert.NilError(t, err)
		assert.Equal(t, p.Services["foo"].Restart, "on-failure:3")
	})

	t.Run("an override file contradicts restart", func(t *testing.T) {
		warnings := captureWarnings(t)
		p, err := loadFiles(restartService("restart: always"),
			"services:\n  foo:\n    deploy: {restart_policy: {condition: none}}\n")
		assert.NilError(t, err)
		assert.Equal(t, p.Services["foo"].Restart, "no")
		assert.Equal(t, len(warnings()), 1)
	})

	t.Run("a service extends one that sets the condition", func(t *testing.T) {
		p, err := loadConsistent(t, `
name: test
services:
  base:
    image: alpine
    deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}
  foo:
    extends: base
    deploy: {restart_policy: {condition: any}}
`)
		assert.NilError(t, err)
		assert.Equal(t, p.Services["foo"].Restart, "")
		assert.Equal(t, p.Services["base"].Restart, "on-failure:3")
	})
}

// The outcome is part of the model that `LoadModelWithContext` hands over, as
// well as of the project: whoever reads the model has not to reconcile the two
// attributes itself.
func TestRestartPolicyIsCarriedIntoTheModel(t *testing.T) {
	captureWarnings(t)
	model, err := loader.LoadModelWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}
  bar:
    image: alpine
    restart: always
    deploy: {restart_policy: {condition: any}}
  qux:
    image: alpine
    restart: always
    deploy: {restart_policy: {condition: none}}
  baz:
    image: alpine
    deploy: {restart_policy: {delay: 5s}}
`)}},
		Environment: map[string]string{},
	})
	assert.NilError(t, err)

	services := model["services"].(map[string]any)
	assert.Equal(t, services["foo"].(map[string]any)["restart"], "on-failure:3")
	assert.Equal(t, services["bar"].(map[string]any)["restart"], "always")
	assert.Equal(t, services["qux"].(map[string]any)["restart"], "no", "deploy.restart_policy wins")
	_, set := services["baz"].(map[string]any)["restart"]
	assert.Assert(t, !set, "a delay alone sets no restart")
}

// The outcome is written when the model is normalized: a caller which skips the
// normalization gets the attributes as written, contradictory or not, and no
// warning.
func TestRestartPolicyIsNotCarriedWithoutNormalization(t *testing.T) {
	warnings := captureWarnings(t)
	loadWith := func(attributes string) (*types.Project, error) {
		return loader.LoadWithContext(t.Context(), types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(restartService(attributes))}},
			Environment: map[string]string{},
		}, func(options *loader.Options) {
			options.SkipNormalization = true
		})
	}

	p, err := loadWith("deploy: {restart_policy: {condition: on-failure, max_attempts: 3}}")
	assert.NilError(t, err)
	assert.Equal(t, p.Services["foo"].Restart, "")

	p, err = loadWith("restart: always\n    deploy: {restart_policy: {condition: none}}")
	assert.NilError(t, err)
	assert.Equal(t, p.Services["foo"].Restart, "always")
	assert.DeepEqual(t, warnings(), []string(nil))
}

// loadsNormalizedAs is loadsAs for the outcome of the normalization, which
// carries `restart`: the service foo of input is loaded with the consistency
// check and the normalization, and its YAML is compared with canonical.
func loadsNormalizedAs(t *testing.T, input, canonical string) {
	t.Helper()
	indent := func(doc string) string {
		return "    " + strings.ReplaceAll(strings.TrimSpace(doc), "\n", "\n    ")
	}
	p, err := loadConsistent(t, "name: test\nservices:\n  foo:\n    image: alpine\n"+indent(input)+"\n")
	assert.NilError(t, err)
	out, err := p.MarshalYAML()
	assert.NilError(t, err)
	var got, want map[string]any
	assert.NilError(t, yaml.Unmarshal(out, &got))
	assert.NilError(t, yaml.Unmarshal([]byte("image: alpine\n"+strings.TrimSpace(canonical)), &want), canonical)
	service := got["services"].(map[string]any)["foo"].(map[string]any)
	// the defaults the normalization adds are not what this locks
	delete(service, "networks")
	assert.DeepEqual(t, service, want)
}

// restartService wraps the attributes in a service named foo, indented as the
// entries of the table cases are.
func restartService(attributes string) string {
	return "name: test\nservices:\n  foo:\n    image: alpine\n    " + strings.TrimSpace(attributes) + "\n"
}
