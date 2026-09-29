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

package override

import (
	"testing"

	"gotest.tools/v3/assert"
)

// Merging sequences ignores entries strictly identical to an already-present
// one: two identical entries never carry more meaning than one, and dropping
// them makes merging a value over an already-merged result idempotent.
func Test_mergeYamlSequenceDropsIdenticalDuplicates(t *testing.T) {
	right := `
services:
  test:
    image: foo
    dns:
      - 8.8.8.8
    cap_add:
      - NET_ADMIN
    volumes:
      - type: volume
        source: data
        target: /data
`
	left := `
services:
  test:
    dns:
      - 8.8.8.8
      - 9.9.9.9
    cap_add:
      - NET_ADMIN
      - SYS_PTRACE
    volumes:
      - type: volume
        source: data
        target: /data
`
	expected := `
services:
  test:
    image: foo
    dns:
      - 8.8.8.8
      - 9.9.9.9
    cap_add:
      - NET_ADMIN
      - SYS_PTRACE
    volumes:
      - type: volume
        source: data
        target: /data
`
	got, err := Merge(unmarshal(t, right), unmarshal(t, left))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, expected))
}

// Strict identity, not equivalence: the same declaration under a different
// form (short vs long syntax) is not deduplicated — the rule never guesses.
func Test_mergeYamlSequenceKeepsEquivalentButDistinctForms(t *testing.T) {
	right := `
services:
  test:
    volumes:
      - data:/data
`
	left := `
services:
  test:
    volumes:
      - type: volume
        source: data
        target: /data
`
	got, err := Merge(unmarshal(t, right), unmarshal(t, left))
	assert.NilError(t, err)
	volumes := got["services"].(map[string]any)["test"].(map[string]any)["volumes"].([]any)
	assert.Equal(t, len(volumes), 2)
}

// Merging an already-merged result with the same override again must be a
// no-op — the property that lets a resolved model be reloaded and re-merged
// without duplicating accumulated entries.
func Test_mergeYamlSequenceIsIdempotent(t *testing.T) {
	base := `
services:
  test:
    dns:
      - 8.8.8.8
    env_file:
      - common.env
    extra_hosts:
      - "host1:127.0.0.1"
`
	override := `
services:
  test:
    dns:
      - 9.9.9.9
    env_file:
      - common.env
      - test.env
    extra_hosts:
      - "host1:127.0.0.1"
      - "host2:127.0.0.2"
`
	once, err := Merge(unmarshal(t, base), unmarshal(t, override))
	assert.NilError(t, err)
	twice, err := Merge(once, unmarshal(t, override))
	assert.NilError(t, err)
	assert.DeepEqual(t, once, twice)
}

// In a KEY=VALUE list the last entry for a key takes effect, so an override
// that repeats a base entry after another value for the same key must still
// win: the repeated entry is not a duplicate to drop, it is the final word.
// Regression of v2.15.0, reported by docker/buildx#4117.
func Test_mergeYamlKeyValueSequenceLastEntryWins(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "environment", path: "environment"},
		{name: "labels", path: "labels"},
		{name: "build args", path: "build:\n      args"},
		{name: "build labels", path: "build:\n      labels"},
		{name: "annotations", path: "annotations"},
		{name: "sysctls", path: "sysctls"},
		{name: "build additional contexts", path: "build:\n      additional_contexts"},
		{name: "deploy labels", path: "deploy:\n      labels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMergeYaml(t, `
services:
  test:
    image: foo
    `+tt.path+`:
      - MODE=release
`, `
services:
  test:
    `+tt.path+`:
      - MODE=debug
      - MODE=release
`, `
services:
  test:
    image: foo
    `+tt.path+`:
      - MODE=release
`)
		})
	}
}

// A file may repeat a key with different values (`[MODE=debug, MODE=release]`
// is a valid list, the last entry takes effect). Merging must compact those
// repeats before applying the override, otherwise the override replaces only
// the last one and leaves two identical entries, which the schema rejects.
// Jobs are used because EnforceUnicity does not cover them and would hide it.
func Test_mergeYamlKeyValueSequenceCompactsRepeatedBaseKeys(t *testing.T) {
	assertMergeYaml(t, `
jobs:
  test:
    image: foo
    labels:
      - MODE=debug
      - MODE=release
`, `
jobs:
  test:
    labels:
      - MODE=debug
`, `
jobs:
  test:
    image: foo
    labels:
      - MODE=debug
`)
}

// Merging two empty KEY=VALUE lists yields an empty list, not a nil one: a nil
// list is not a valid `labels` value for the schema. Secrets are used because
// EnforceUnicity does not cover them and would rebuild the list, hiding it.
func Test_mergeYamlKeyValueSequenceOfEmptyLists(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		override string
	}{
		{name: "empty list over empty list", base: "[]", override: "[]"},
		{name: "empty mapping over empty mapping", base: "{}", override: "{}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMergeYaml(t, `
secrets:
  test:
    labels: `+tt.base+`
`, `
secrets:
  test:
    labels: `+tt.override+`
`, `
secrets:
  test:
    labels: []
`)
		})
	}
}
