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

// The tests in this file lock the `labels` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#labels
//
// Spec: "`labels` add metadata to containers."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestServiceLabels(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    labels:
      com.example.description: "Accounting webapp"
      com.example.number: 42
      com.example.empty-label:
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    labels:
      com.example.description: "Accounting webapp"
      com.example.number: 42
      com.example.empty-label:
`)
	expect := func(p *types.Project) {
		expected := types.Labels{
			"com.example.description": "Accounting webapp",
			"com.example.number":      "42",
			"com.example.empty-label": "",
		}
		assert.DeepEqual(t, p.Services["foo"].Labels, expected)
		assert.DeepEqual(t, p.Jobs["foo"].Labels, expected)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestServiceLabelsListSyntax(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    labels:
      - "com.example.description=Accounting webapp"
      - "com.example.number=42"
      - "com.example.empty-label"
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    labels:
      - "com.example.description=Accounting webapp"
      - "com.example.number=42"
      - "com.example.empty-label"
`)
	expected := types.Labels{
		"com.example.description": "Accounting webapp",
		"com.example.number":      "42",
		"com.example.empty-label": "",
	}
	assert.DeepEqual(t, p.Services["foo"].Labels, expected)
	assert.DeepEqual(t, p.Jobs["foo"].Labels, expected)
}

// In a KEY=VALUE list the last entry for a key takes effect, so an override
// file repeating a base entry after another value for the same key must still
// win, while an entry repeated as is stays a single one. It holds for the
// labels of every resource, not only of services.
func TestLabelsLastEntryWinsAcrossFiles(t *testing.T) {
	const labels = `
    labels:
      - mode=release
      - keep=1
`
	overridden := `
    labels:
      - mode=debug
      - mode=release
      - keep=1
`
	base := `
name: test
services:
  foo:
    image: alpine
` + labels + `
jobs:
  foo:
    image: alpine
    triggers:
      manual: true
` + labels + `
networks:
  foo:
` + labels + `
volumes:
  foo:
` + labels + `
secrets:
  foo:
    environment: FOO
` + labels + `
configs:
  foo:
    environment: FOO
` + labels
	override := `
services:
  foo:` + overridden + `
jobs:
  foo:` + overridden + `
networks:
  foo:` + overridden + `
volumes:
  foo:` + overridden + `
secrets:
  foo:` + overridden + `
configs:
  foo:` + overridden
	loadFilesAs(t, []string{base, override}, `
name: test
services:
  foo:
    image: alpine
    labels: {mode: release, keep: "1"}
jobs:
  foo:
    name: foo
    image: alpine
    triggers:
      manual: true
    labels: {mode: release, keep: "1"}
networks:
  foo:
    labels: {mode: release, keep: "1"}
volumes:
  foo:
    labels: {mode: release, keep: "1"}
secrets:
  foo:
    environment: FOO
    labels: {mode: release, keep: "1"}
configs:
  foo:
    environment: FOO
    labels: {mode: release, keep: "1"}
`)
}
