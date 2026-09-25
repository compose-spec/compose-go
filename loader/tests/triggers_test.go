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
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestTriggersScheduleShortSyntax(t *testing.T) {
	p := load(t, `
name: test
jobs:
  backup:
    image: backup-tool
    triggers:
      schedule:
        - "0 3 * * *"
`)

	expect := func(p *types.Project) {
		schedule := p.Jobs["backup"].Triggers.Schedule
		assert.Equal(t, len(schedule), 1)
		// a plain crontab entry is canonicalized into a schedule object
		assert.Equal(t, schedule[0].Cron, "0 3 * * *")
		assert.Equal(t, schedule[0].Timezone, "")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestTriggersScheduleList(t *testing.T) {
	p := load(t, `
name: test
jobs:
  backup:
    image: backup-tool
    triggers:
      schedule:
        - cron: "0 3 * * *"
          timezone: Europe/Paris
          concurrency: queue
          missed_fires: skip
        - "0 1 * * 0"
`)

	expect := func(p *types.Project) {
		schedule := p.Jobs["backup"].Triggers.Schedule
		assert.Equal(t, len(schedule), 2)
		assert.Equal(t, schedule[0].Cron, "0 3 * * *")
		assert.Equal(t, schedule[0].Timezone, "Europe/Paris")
		assert.Equal(t, schedule[0].Concurrency, "queue")
		assert.Equal(t, schedule[0].MissedFires, "skip")
		// plain crontab list entry is canonicalized too
		assert.Equal(t, schedule[1].Cron, "0 1 * * 0")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// manual is tri-state: an explicit false must survive loading and round-trip
// distinctly from unset, so consumers can refuse manual execution.
func TestTriggersManualExplicitFalse(t *testing.T) {
	p := load(t, `
name: test
jobs:
  nightly:
    image: batch
    triggers:
      manual: false
      schedule:
        - "0 3 * * *"
`)

	expect := func(p *types.Project) {
		manual := p.Jobs["nightly"].Triggers.Manual
		assert.Assert(t, manual != nil)
		assert.Equal(t, *manual, false)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestJobProfiles(t *testing.T) {
	yaml := `
name: test
jobs:
  seed:
    image: myapp
    profiles:
      - debug
    triggers:
      manual: true
  always:
    image: myapp
    triggers:
      manual: true
`
	// a job with a profile is inactive by default
	p := load(t, yaml)
	assert.Equal(t, len(p.Jobs), 1)
	_, enabled := p.Jobs["always"]
	assert.Check(t, enabled)
	_, disabled := p.DisabledJobs["seed"]
	assert.Check(t, disabled)
	assert.DeepEqual(t, p.DisabledJobs["seed"].Profiles, []string{"debug"})

	// activating the profile enables the job
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.Profiles = []string{"debug"}
	})
	assert.NilError(t, err)
	assert.Equal(t, len(p.Jobs), 2)
	assert.Equal(t, len(p.DisabledJobs), 0)
}

func TestTriggersScheduleInvalid(t *testing.T) {
	// cron is required on the schedule object form
	loadErr := func(yaml string) error {
		_, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
			Environment: map[string]string{},
		})
		return err
	}
	// schedule is always a list: a bare crontab expression is rejected
	err := loadErr(`
name: test
jobs:
  backup:
    image: backup-tool
    triggers:
      schedule: "0 3 * * *"
`)
	assert.ErrorContains(t, err, "jobs.backup.triggers.schedule")

	// cron is required on the schedule object form
	err = loadErr(`
name: test
jobs:
  backup:
    image: backup-tool
    triggers:
      schedule:
        - timezone: Europe/Paris
`)
	assert.ErrorContains(t, err, "jobs.backup.triggers.schedule")

	// concurrency values are restricted
	err = loadErr(`
name: test
jobs:
  backup:
    image: backup-tool
    triggers:
      schedule:
        - cron: "0 3 * * *"
          concurrency: replace
`)
	assert.ErrorContains(t, err, "jobs.backup.triggers.schedule")
}
