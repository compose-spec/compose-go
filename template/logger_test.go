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

package template

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// recordingLogger collects the warnings it receives.
type recordingLogger struct {
	warnings []string
}

func (r *recordingLogger) Warnf(format string, args ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, args...))
}

// An unset variable keeps being reported through the standard logrus logger
// when no Logger is configured: routing warnings elsewhere is strictly opt-in.
func TestUnsetVariableWarnsThroughLogrusByDefault(t *testing.T) {
	hook, restore := captureWarnings(t)
	defer restore()

	result, err := Substitute("${UNSET}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("", result))
	assert.Assert(t, is.Len(hook.AllEntries(), 1))
	assert.Check(t, is.Equal(hook.LastEntry().Message, "The \"UNSET\" variable is not set. Defaulting to a blank string."))
}

// With WithLogger, the "variable is not set" warning, wherever it is raised in
// the template (top level, in a default value, after a braced expression), is
// delivered to the logger and nothing is emitted through logrus.
func TestUnsetVariableWarnsThroughLogger(t *testing.T) {
	tests := []struct {
		name     string
		template string
		want     string
		warnings []string
	}{
		{
			name:     "plain variable",
			template: "${UNSET}",
			want:     "",
			warnings: []string{"The \"UNSET\" variable is not set. Defaulting to a blank string."},
		},
		{
			name:     "variable in a default value",
			template: "${A:-${UNSET}}",
			want:     "",
			warnings: []string{"The \"UNSET\" variable is not set. Defaulting to a blank string."},
		},
		{
			name:     "variable in the text following a braced expression",
			template: "${FOO:-x}-${UNSET}",
			want:     "first-",
			warnings: []string{"The \"UNSET\" variable is not set. Defaulting to a blank string."},
		},
		{
			name:     "set variable does not warn",
			template: "${FOO}",
			want:     "first",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hook, restore := captureWarnings(t)
			defer restore()

			logger := &recordingLogger{}
			result, err := SubstituteWithOptions(tc.template, defaultMapping, WithLogger(logger))
			assert.NilError(t, err)
			assert.Check(t, is.Equal(tc.want, result))
			assert.Check(t, is.DeepEqual(tc.warnings, logger.warnings))
			assert.Check(t, is.Len(hook.AllEntries(), 0))
		})
	}
}

// WithoutLogging silences the warning even when a Logger is configured.
func TestWithoutLoggingTakesPrecedenceOverLogger(t *testing.T) {
	hook, restore := captureWarnings(t)
	defer restore()

	logger := &recordingLogger{}
	_, err := SubstituteWithOptions("${UNSET}", defaultMapping, WithLogger(logger), WithoutLogging)
	assert.NilError(t, err)
	assert.Check(t, is.Len(logger.warnings, 0))
	assert.Check(t, is.Len(hook.AllEntries(), 0))
}
