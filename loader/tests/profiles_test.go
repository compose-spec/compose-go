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

func TestProfiles(t *testing.T) {
	yaml := `
name: test
services:
  foo:
    image: alpine
    profiles:
      - debug
      - dev
  bar:
    image: alpine
`
	// Without profiles, "foo" is disabled
	p := load(t, yaml)
	_, hasFoo := p.Services["foo"]
	assert.Assert(t, !hasFoo)
	assert.Equal(t, p.Services["bar"].Image, "alpine")

	// With matching profile, "foo" is enabled
	p2, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.Profiles = []string{"debug"}
	})
	assert.NilError(t, err)
	assert.Equal(t, p2.Services["foo"].Image, "alpine")
	assert.DeepEqual(t, p2.Services["foo"].Profiles, []string{"debug", "dev"})
}
