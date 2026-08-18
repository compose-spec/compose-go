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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// TestGoldenFiles guards the canonical model produced for `services:` against
// regressions: each testdata/golden/*.yaml corpus file is loaded and the
// resulting project, marshalled to YAML and JSON, must be byte-identical to the
// committed .golden.yaml / .golden.json files.
//
// The committed golden files were verified semantically identical (sorted-key
// JSON comparison) to the model produced by the pre-ContainerSpec-extraction
// parser, proving the refactor behavior-preserving for services. Only the
// marshalling key order changed (service-level fields now serialize before the
// inlined container spec block) — a cosmetic, release-noted difference.
//
// Regenerate with: UPDATE_GOLDEN=1 go test ./loader/ -run TestGoldenFiles
func TestGoldenFiles(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("testdata", "golden", "*.yaml"))
	assert.NilError(t, err)

	update := os.Getenv("UPDATE_GOLDEN") != ""
	seen := 0
	for _, input := range inputs {
		if strings.HasSuffix(input, ".golden.yaml") {
			continue
		}
		seen++
		t.Run(filepath.Base(input), func(t *testing.T) {
			content, err := os.ReadFile(input)
			assert.NilError(t, err)

			p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
				WorkingDir:  filepath.Join("testdata", "golden"),
				ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: content}},
				Environment: map[string]string{
					"GOLDEN_TAG":  "1.2.3",
					"GOLDEN_PORT": "8080",
				},
			}, func(options *Options) {
				options.SetProjectName("golden", true)
				options.Profiles = []string{"*"}
				options.SkipConsistencyCheck = true
				// keep paths as written so golden files are host- and OS-independent
				options.ResolvePaths = false
			})
			assert.NilError(t, err)

			yamlBytes, err := p.MarshalYAML()
			assert.NilError(t, err)
			jsonBytes, err := p.MarshalJSON()
			assert.NilError(t, err)

			base := strings.TrimSuffix(input, ".yaml")
			goldenYAML := base + ".golden.yaml"
			goldenJSON := base + ".golden.json"

			if update {
				assert.NilError(t, os.WriteFile(goldenYAML, yamlBytes, 0o644))
				assert.NilError(t, os.WriteFile(goldenJSON, jsonBytes, 0o644))
				return
			}

			expectedYAML, err := os.ReadFile(goldenYAML)
			assert.NilError(t, err, "missing golden file, run with UPDATE_GOLDEN=1 to create it")
			assert.Equal(t, string(expectedYAML), string(yamlBytes))

			expectedJSON, err := os.ReadFile(goldenJSON)
			assert.NilError(t, err, "missing golden file, run with UPDATE_GOLDEN=1 to create it")
			assert.Equal(t, string(expectedJSON), string(jsonBytes))
		})
	}
	assert.Assert(t, seen > 0, "no golden corpus files found")
}
