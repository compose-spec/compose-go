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

// The tests in this file lock the `version` top-level element:
//   https://github.com/compose-spec/compose-spec/blob/main/04-version-and-name.md#version-top-level-element-obsolete
//
// Spec: "The top-level `version` property is defined by the Compose
// Specification for backward compatibility. It is only informative you'll
// receive a warning message that it is obsolete if used."

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// recordingLogger collects the warnings it receives.
type recordingLogger struct {
	warnings []string
}

func (r *recordingLogger) Warnf(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

// versionFileSeq makes the name of each loaded file unique: the obsolete
// version warning is emitted once per file name for the whole process, so a
// name reused by a second run of the test (-count=2) would not warn again.
var versionFileSeq atomic.Int64

func versionFilename(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s-%d.yml", t.Name(), versionFileSeq.Add(1))
}

// A file declaring `version` loads, the attribute is ignored, and a warning
// says it is obsolete.
func TestVersionIsObsoleteAndIgnored(t *testing.T) {
	for _, version := range []string{`"3.8"`, `"2"`, `"whatever"`} {
		t.Run(version, func(t *testing.T) {
			filename := versionFilename(t)
			logger := &recordingLogger{}
			p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{{
					Filename: filename,
					Content:  []byte("version: " + version + "\nname: test\nservices:\n  foo:\n    image: busybox\n"),
				}},
				Environment: map[string]string{},
			}, func(o *loader.Options) {
				o.SkipConsistencyCheck = true
				o.SkipNormalization = true
				o.Logger = logger
			})
			assert.NilError(t, err)
			assert.Equal(t, p.Services["foo"].Image, "busybox")
			assert.DeepEqual(t, logger.warnings, []string{
				filename + ": the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion",
			})

			// the version is not part of the model
			out, err := p.MarshalYAML()
			assert.NilError(t, err)
			assert.Assert(t, !strings.Contains(string(out), "version:"), string(out))
		})
	}
}

// Without `version` there is nothing to warn about.
func TestNoVersionNoWarning(t *testing.T) {
	logger := &recordingLogger{}
	_, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{
			Filename: versionFilename(t),
			Content:  []byte("name: test\nservices:\n  foo:\n    image: busybox\n"),
		}},
		Environment: map[string]string{},
	}, func(o *loader.Options) {
		o.SkipConsistencyCheck = true
		o.SkipNormalization = true
		o.Logger = logger
	})
	assert.NilError(t, err)
	assert.Equal(t, len(logger.warnings), 0)
}
