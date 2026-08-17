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
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func load(t *testing.T, content string) *types.Project {
	t.Helper()
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
	})
	assert.NilError(t, err)
	return p
}

func loadWithEnv(t *testing.T, content string, env map[string]string) *types.Project {
	t.Helper()
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
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

// loadsAs asserts that the input compose YAML loads into the canonical model
// expressed as YAML: the input is loaded, marshaled back to YAML, and both
// documents are compared as YAML trees. The expectation then reads as
// compose documentation, with no knowledge of types.Project required.
// Prefer it over field-by-field assertions when the behavior under test is
// how a syntax is parsed and canonicalized; keep typed assertions for
// behaviors the canonical form does not surface.
func loadsAs(t *testing.T, input, canonical string) {
	t.Helper()
	p := load(t, input)
	out, err := p.MarshalYAML()
	assert.NilError(t, err)
	var got, want map[string]any
	assert.NilError(t, yaml.Unmarshal(out, &got))
	assert.NilError(t, yaml.Unmarshal([]byte(canonical), &want), canonical)
	assert.DeepEqual(t, got, want)
}
