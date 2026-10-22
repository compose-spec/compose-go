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
	"fmt"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func load(t *testing.T, content string) *types.Project {
	t.Helper()
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
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
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
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
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
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

// loadErr loads a project expected to be rejected and returns the error.
func loadErr(t *testing.T, content string) error {
	t.Helper()
	_, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
	})
	assert.Assert(t, err != nil, "expected loading to fail")
	return err
}

// loadFilesMap loads a multi-file project, merging the files in order, and
// returns the resulting model as a YAML tree.
func loadFilesMap(t *testing.T, files []string) map[string]any {
	t.Helper()
	var configFiles []types.ConfigFile
	for i, content := range files {
		configFiles = append(configFiles, types.ConfigFile{Filename: fmt.Sprintf("compose-%d.yml", i), Content: []byte(content)})
	}
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: configFiles,
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
	})
	assert.NilError(t, err)
	out, err := p.MarshalYAML()
	assert.NilError(t, err)
	var got map[string]any
	assert.NilError(t, yaml.Unmarshal(out, &got))
	return got
}

// loadFilesAs is loadsAs for a multi-file project: the files are merged in
// order and the result is compared, as YAML, with the canonical document.
func loadFilesAs(t *testing.T, files []string, canonical string) {
	t.Helper()
	var want map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(canonical), &want), canonical)
	assert.DeepEqual(t, loadFilesMap(t, files), want)
}

// serviceWith returns a compose document declaring one service, "foo", whose
// only attribute besides the image is attr set to the given YAML value.
func serviceWith(attr, value string) string {
	return "name: test\nservices:\n  foo:\n    image: alpine\n    " + attr + ": " + value + "\n"
}

// byteValueNotations lists the notations of a byte value, as documented by
//
//	https://github.com/compose-spec/compose-spec/blob/main/11-extension.md#specifying-byte-values
//
// with the number of bytes each one stands for. A plain number (or numeric
// string) is a count of bytes. Attributes typed as a byte value (mem_limit,
// mem_reservation, memswap_limit, shm_size) share this vocabulary.
var byteValueNotations = []struct{ name, notation, bytes string }{
	{"bytes with the b unit", `2b`, "2"},
	{"kilobytes with the k unit", `2048k`, "2097152"},
	{"kilobytes with the kb unit", `1024kb`, "1048576"},
	{"megabytes with the m unit", `300m`, "314572800"},
	{"megabytes with the mb unit", `300mb`, "314572800"},
	{"gigabytes with the g unit", `1g`, "1073741824"},
	{"gigabytes with the gb unit", `1gb`, "1073741824"},
	{"a number of bytes", `1048576`, "1048576"},
	{"a numeric string", `"1048576"`, "1048576"},
}

// loadsByteValueNotations asserts that every notation of a byte value is
// accepted by the service attribute attr, and canonicalizes into a decimal
// number of bytes (a quoted string in the marshaled YAML).
func loadsByteValueNotations(t *testing.T, attr string) {
	t.Helper()
	for _, tc := range byteValueNotations {
		t.Run(tc.name, func(t *testing.T) {
			loadsAs(t, serviceWith(attr, tc.notation), serviceWith(attr, `"`+tc.bytes+`"`))
		})
	}
}

// loadConsistent loads a project with the normalization and consistency
// stages enabled, which the other helpers skip, and returns the outcome.
func loadConsistent(t *testing.T, content string) (*types.Project, error) {
	t.Helper()
	return loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
		Environment: map[string]string{},
	})
}
