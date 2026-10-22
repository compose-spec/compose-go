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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestParseYAMLFiles(t *testing.T) {
	model, err := loadYamlModel(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "test.yaml",
				Content: []byte(`
x-extension:
    test1: first

services:
  test:
    image: foo
    command: echo hello
    init: true
`),
			},
			{
				Filename: "override.yaml",
				Content: []byte(`
x-extension:
    test2: second

services:
  test:
    image: bar
    command: echo world
    init: false
`),
			},
		},
	}, &Options{}, &cycleTracker{}, nil)
	assert.NilError(t, err)
	assert.DeepEqual(t, model, map[string]interface{}{
		"services": map[string]interface{}{
			"test": map[string]interface{}{
				"image":   "bar",
				"command": "echo world",
				"init":    false,
			},
		},
		"x-extension": map[string]interface{}{
			"test1": "first",
			"test2": "second",
		},
	})
}

func TestParseYAMLFilesMergeOverride(t *testing.T) {
	model, err := loadYamlModel(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "override.yaml",
				Content: []byte(`
services:
  base:
    configs:
      - source: credentials
        target: /credentials/file1
  x: &x
    extends:
      base
    configs: !override
      - source: credentials
        target: /literally-anywhere-else

  y:
    <<: *x

configs:
  credentials:
    content: |
      dummy value
`),
			},
		},
	}, &Options{}, &cycleTracker{}, nil)
	assert.NilError(t, err)
	assert.DeepEqual(t, model, map[string]interface{}{
		"configs": map[string]interface{}{"credentials": map[string]interface{}{"content": string("dummy value\n")}},
		"services": map[string]interface{}{
			"base": map[string]interface{}{
				"configs": []interface{}{
					map[string]interface{}{"source": string("credentials"), "target": string("/credentials/file1")},
				},
			},
			"x": map[string]interface{}{
				"configs": []interface{}{
					map[string]interface{}{"source": string("credentials"), "target": string("/literally-anywhere-else")},
				},
			},
			"y": map[string]interface{}{
				"configs": []interface{}{
					map[string]interface{}{"source": string("credentials"), "target": string("/literally-anywhere-else")},
				},
			},
		},
	})
}

// loadYamlFile must fail, without producing a model, on every kind of
// unusable input: unreadable file, malformed YAML, a document that is not a
// mapping, and documents that cannot be merged or deduplicated.
// https://github.com/compose-spec/compose-spec/blob/main/03-compose-file.md
func TestLoadYamlFileErrors(t *testing.T) {
	tests := []struct {
		name  string
		files []types.ConfigFile
		err   string
	}{
		{
			name:  "missing file",
			files: []types.ConfigFile{{Filename: filepath.Join(t.TempDir(), "nope.yaml")}},
			err:   "nope.yaml",
		},
		{
			name:  "malformed yaml",
			files: []types.ConfigFile{{Filename: "a.yaml", Content: []byte("a: [")}},
			err:   "failed to parse a.yaml: ",
		},
		{
			name:  "top-level is not a mapping",
			files: []types.ConfigFile{{Filename: "a.yaml", Content: []byte("- a")}},
			err:   "top-level object must be a mapping",
		},
		{
			name:  "second document is not a mapping",
			files: []types.ConfigFile{{Filename: "a.yaml", Content: []byte("services: {}\n---\n- a")}},
			err:   "top-level object must be a mapping",
		},
		{
			name: "files cannot be merged",
			files: []types.ConfigFile{
				{Filename: "a.yaml", Content: []byte("services:\n  a:\n    logging:\n      driver: x\n")},
				{Filename: "b.yaml", Content: []byte("services:\n  a:\n    logging: foo\n")},
			},
			err: "services.a.logging: cannot merge logging: expected a mapping, got string",
		},
		{
			name:  "volume without target",
			files: []types.ConfigFile{{Filename: "a.yaml", Content: []byte("services:\n  a:\n    volumes:\n      - type: volume\n")}},
			err:   "service volume services.a.volumes.[0] is missing a mount target",
		},
		{
			name:  "schema validation",
			files: []types.ConfigFile{{Filename: "a.yaml", Content: []byte("services:\n  a:\n    unknown_attribute: true\n")}},
			err:   "validating a.yaml: ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := loadYamlModel(t.Context(), types.ConfigDetails{ConfigFiles: tt.files}, &Options{}, &cycleTracker{}, nil)
			assert.ErrorContains(t, err, tt.err)
			assert.Check(t, model == nil)
		})
	}

	t.Run("missing file keeps the os error", func(t *testing.T) {
		_, err := loadYamlModel(t.Context(), types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(t.TempDir(), "nope.yaml")}},
		}, &Options{}, &cycleTracker{}, nil)
		assert.Check(t, errors.Is(err, fs.ErrNotExist))
	})
}

// An empty file contributes nothing: the model accumulated so far is handed
// back untouched and no post-processor is produced.
// https://github.com/compose-spec/compose-spec/blob/main/13-merge.md
func TestLoadYamlFileEmptyContent(t *testing.T) {
	dict := map[string]any{"services": map[string]any{"a": map[string]any{"image": "foo"}}}
	got, processor, err := loadYamlFile(t.Context(), types.ConfigFile{Filename: "empty.yaml", Content: []byte("")},
		&Options{}, "", nil, &cycleTracker{}, dict, nil)
	assert.NilError(t, err)
	assert.Check(t, processor == nil)
	assert.DeepEqual(t, got, map[string]any{"services": map[string]any{"a": map[string]any{"image": "foo"}}})
}

// A ConfigFile carrying an already-parsed Config goes through the same
// pipeline as parsed content, but has no YAML tags to post-process, so no
// post-processor is returned.
// https://github.com/compose-spec/compose-spec/blob/main/03-compose-file.md
func TestLoadYamlFilePreParsedConfig(t *testing.T) {
	got, processor, err := loadYamlFile(t.Context(), types.ConfigFile{
		Filename: "config.yaml",
		Config: map[string]any{
			"services": map[string]any{"a": map[string]any{"image": "foo"}},
		},
	}, &Options{}, "", nil, &cycleTracker{}, map[string]any{}, nil)
	assert.NilError(t, err)
	assert.Check(t, processor == nil)
	assert.DeepEqual(t, got, map[string]any{"services": map[string]any{"a": map[string]any{"image": "foo"}}})
}

// Each document of a multi-document YAML file is merged over the previous
// ones, and a `!reset` tag in a later document removes the attribute declared
// by an earlier one.
// https://github.com/compose-spec/compose-spec/blob/main/13-merge.md#reset-value
func TestLoadYamlFileMultiDocument(t *testing.T) {
	input := `
services:
  a:
    image: foo
    environment:
      FOO: bar
    command: first
---
services:
  a:
    environment: !reset null
    command: second
  b:
    image: bar
`
	model, err := loadYamlModel(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: []byte(input)}},
	}, &Options{}, &cycleTracker{}, nil)
	assert.NilError(t, err)

	var expected map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(`
services:
  a:
    image: foo
    command: second
  b:
    image: bar
`), &expected))
	assert.DeepEqual(t, model, expected)
}

// The include stack grows with every document of a multi-document file: the
// file is pushed once per document, and the include cycle report shows it
// that many times. This locks the current report, not an obligation of the
// spec.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md
func TestLoadYamlFileMultiDocumentIncludeStack(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")
	assert.NilError(t, os.WriteFile(a, []byte("services: {a: {image: x}}\n---\ninclude:\n  - b.yaml\n"), 0o600))
	assert.NilError(t, os.WriteFile(b, []byte("include:\n  - a.yaml\n"), 0o600))

	_, err := LoadWithContext(t.Context(), types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: a}},
	}, withProjectName("multi-doc", true))
	assert.Error(t, err, fmt.Sprintf("include cycle detected:\n%s\n include %s\n include %s\n include %s", a, a, b, a))
}
