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

func load(t *testing.T, yaml string) *types.Project {
	t.Helper()
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
	})
	assert.NilError(t, err)
	return p
}

func loadWithEnv(t *testing.T, yaml string, env map[string]string) *types.Project {
	t.Helper()
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
		Environment: env,
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
	})
	assert.NilError(t, err)
	return p
}

// loadRoundTrip reloads marshaled content, skipping schema validation since
// marshaled output may not match schema exactly (e.g. missing yaml tags on some structs).
func loadRoundTrip(t *testing.T, content string) *types.Project {
	t.Helper()
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.SkipValidation = true
	})
	assert.NilError(t, err)
	return p
}

// roundTrip marshals the project to YAML and JSON, reloads each, and returns both.
func roundTrip(t *testing.T, p *types.Project) (fromYAML, fromJSON *types.Project) {
	t.Helper()

	yamlBytes, err := p.MarshalYAML()
	assert.NilError(t, err)
	fromYAML = loadRoundTrip(t, string(yamlBytes))

	jsonBytes, err := p.MarshalJSON()
	assert.NilError(t, err)
	fromJSON = loadRoundTrip(t, string(jsonBytes))

	return
}

func ptr[T any](t T) *T {
	return &t
}
