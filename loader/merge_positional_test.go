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

package loader

import (
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func loadMerge(t *testing.T, base, override string) (*types.Project, error) {
	t.Helper()
	return LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "compose.yaml", Content: []byte(base)},
			{Filename: "compose.override.yaml", Content: []byte(override)},
		},
	}, func(o *Options) { o.SkipNormalization = true })
}

// The `!merge` tag turns the default sequence handling into a positional merge:
// an override sequence is aligned with the base by index, and a `- {}` element
// is a no-op that leaves the corresponding base element untouched. Here a single
// argument of `command` is changed — note that `command` normally *replaces* the
// whole sequence, so this also shows `!merge` taking precedence over that rule.
func TestMerge_CommandSingleArg(t *testing.T) {
	base := `
name: merge-example
services:
  app:
    image: alpine
    command:
      - server
      - --port
      - "8080"
      - --verbose
`
	override := `
services:
  app:
    command: !merge
      - {}
      - {}
      - "9090"
`
	p, err := loadMerge(t, base, override)
	assert.NilError(t, err)
	assert.DeepEqual(t, []string(p.Services["app"].Command), []string{"server", "--port", "9090", "--verbose"})
}

// `dns` normally *appends*. With `!merge` the override replaces the first entry
// positionally and keeps the second (`- {}`), instead of appending.
func TestMerge_DnsPositional(t *testing.T) {
	base := `
name: merge-example
services:
  app:
    image: alpine
    dns:
      - 1.1.1.1
      - 8.8.8.8
`
	override := `
services:
  app:
    dns: !merge
      - 9.9.9.9
      - {}
`
	p, err := loadMerge(t, base, override)
	assert.NilError(t, err)
	assert.DeepEqual(t, []string(p.Services["app"].DNS), []string{"9.9.9.9", "8.8.8.8"})
}

// The tag is ignored on anything that is not a sequence, so it is a harmless
// no-op there and the mapping merges as usual.
func TestMerge_IgnoredOnMapping(t *testing.T) {
	base := `
name: merge-example
services:
  app:
    image: alpine
    environment:
      FOO: bar
`
	override := `
services:
  app:
    environment: !merge
      BAZ: qux
`
	p, err := loadMerge(t, base, override)
	assert.NilError(t, err)
	env := p.Services["app"].Environment
	assert.Equal(t, *env["FOO"], "bar")
	assert.Equal(t, *env["BAZ"], "qux")
}
