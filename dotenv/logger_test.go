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

package dotenv

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
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

// captureLogrus records what is written through the standard logrus logger
// for the duration of the test, and restores it afterwards. The standard
// logger is process-global: tests using it must not run in parallel.
func captureLogrus(t *testing.T) *logrustest.Hook {
	t.Helper()
	std := logrus.StandardLogger()
	oldOut := std.Out
	oldHooks := std.ReplaceHooks(make(logrus.LevelHooks))
	std.SetOutput(io.Discard)
	hook := &logrustest.Hook{}
	std.AddHook(hook)
	t.Cleanup(func() {
		std.ReplaceHooks(oldHooks)
		std.SetOutput(oldOut)
	})
	return hook
}

// An env file referencing a variable that is not set raises a warning: it goes
// through the standard logrus logger unless a logger is given, in which case
// the logger receives it and logrus stays silent.
func TestGetEnvFromFileUnsetVariable(t *testing.T) {
	const message = "The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."
	file := filepath.Join(t.TempDir(), ".env")
	assert.NilError(t, os.WriteFile(file, []byte("FOO=${UNSET_VARIABLE}\n"), 0o600))

	// intent: GetEnvFromFile, which takes no logger, keeps writing through
	// logrus as it has always done
	t.Run("default goes through logrus", func(t *testing.T) {
		hook := captureLogrus(t)
		env, err := GetEnvFromFile(map[string]string{}, []string{file})
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(map[string]string{"FOO": ""}, env))
		assert.Check(t, is.Len(hook.AllEntries(), 1))
		assert.Check(t, is.Equal(message, hook.LastEntry().Message))
	})

	// intent: a nil logger is the default one
	t.Run("nil logger goes through logrus", func(t *testing.T) {
		hook := captureLogrus(t)
		_, err := GetEnvFromFileWithLogger(map[string]string{}, []string{file}, nil)
		assert.NilError(t, err)
		assert.Check(t, is.Len(hook.AllEntries(), 1))
	})

	// intent: the logger receives the warning, logrus does not
	t.Run("logger receives it", func(t *testing.T) {
		hook := captureLogrus(t)
		logger := &recordingLogger{}
		env, err := GetEnvFromFileWithLogger(map[string]string{}, []string{file}, logger)
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(map[string]string{"FOO": ""}, env))
		assert.Check(t, is.DeepEqual([]string{message}, logger.warnings))
		assert.Check(t, is.Len(hook.AllEntries(), 0))
	})
}

// ParseWithFormat reads an env_file of the format it is given; the warnings it
// raises (a variable that is not set) go through the standard logrus logger
// unless ParseWithFormatAndLogger is given a logger, which then receives them
// and logrus stays silent.
func TestParseWithFormatUnsetVariable(t *testing.T) {
	const (
		message = "The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."
		content = "FOO=${UNSET_VARIABLE}\n"
	)
	parse := func(format string, l *recordingLogger) (map[string]string, error) {
		vars := map[string]string{}
		var err error
		if l == nil {
			err = ParseWithFormat(strings.NewReader(content), "a.env", vars, nil, format)
		} else {
			err = ParseWithFormatAndLogger(strings.NewReader(content), "a.env", vars, nil, format, l)
		}
		return vars, err
	}
	for _, format := range []string{"", DotEnv} {
		t.Run("format "+format, func(t *testing.T) {
			// intent: ParseWithFormat, which takes no logger, keeps writing
			// through logrus as it has always done
			t.Run("default goes through logrus", func(t *testing.T) {
				hook := captureLogrus(t)
				vars, err := parse(format, nil)
				assert.NilError(t, err)
				assert.Check(t, is.DeepEqual(map[string]string{"FOO": ""}, vars))
				assert.Check(t, is.Len(hook.AllEntries(), 1))
				assert.Check(t, is.Equal(message, hook.LastEntry().Message))
			})

			// intent: a nil logger is the default one
			t.Run("nil logger goes through logrus", func(t *testing.T) {
				hook := captureLogrus(t)
				assert.NilError(t, ParseWithFormatAndLogger(strings.NewReader(content), "a.env", map[string]string{}, nil, format, nil))
				assert.Check(t, is.Len(hook.AllEntries(), 1))
			})

			// intent: the logger receives the warning, logrus does not
			t.Run("logger receives it", func(t *testing.T) {
				hook := captureLogrus(t)
				logger := &recordingLogger{}
				vars, err := parse(format, logger)
				assert.NilError(t, err)
				assert.Check(t, is.DeepEqual(map[string]string{"FOO": ""}, vars))
				assert.Check(t, is.DeepEqual([]string{message}, logger.warnings))
				assert.Check(t, is.Len(hook.AllEntries(), 0))
			})
		})
	}

	// intent: a format added by RegisterFormat is called as it always was, and
	// replacing the built-in .env format is still honored: the logger is not
	// forced on a parser that has no way to receive it
	t.Run("registered format is called as is", func(t *testing.T) {
		t.Cleanup(func() { formats[DotEnv] = format{parseWithLogger: parseDotEnv}; delete(formats, "custom-logger") })
		RegisterFormat("custom-logger", func(_ io.Reader, _ string, vars map[string]string, _ func(string) (string, bool)) error {
			vars["CUSTOM"] = "yes"
			return nil
		})
		logger := &recordingLogger{}
		vars := map[string]string{}
		assert.NilError(t, ParseWithFormatAndLogger(strings.NewReader(content), "a.env", vars, nil, "custom-logger", logger))
		assert.Check(t, is.DeepEqual(map[string]string{"CUSTOM": "yes"}, vars))
		assert.Check(t, is.Len(logger.warnings, 0))

		RegisterFormat(DotEnv, func(_ io.Reader, _ string, vars map[string]string, _ func(string) (string, bool)) error {
			vars["REPLACED"] = "yes"
			return nil
		})
		vars = map[string]string{}
		assert.NilError(t, ParseWithFormatAndLogger(strings.NewReader(content), "a.env", vars, nil, "", logger))
		assert.Check(t, is.DeepEqual(map[string]string{"REPLACED": "yes"}, vars))
	})
}
