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

package paths

import (
	"fmt"
	"io"
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

// A "~" that cannot be expanded because the environment lacks HOME is left
// as is and reported by a warning, while the paths are resolved. The warning
// goes through the standard logrus logger unless a logger is given, in which
// case the logger receives it and logrus stays silent.
func TestResolveRelativePathsWarnsWithoutHome(t *testing.T) {
	const message = "cannot expand '~', because the environment lacks HOME"
	setHome(t, "")
	project := func() map[string]any {
		return map[string]any{
			"secrets": map[string]any{"token": map[string]any{"file": "~/token"}},
		}
	}

	// intent: ResolveRelativePaths, which takes no logger, keeps writing
	// through logrus as it has always done
	t.Run("default goes through logrus", func(t *testing.T) {
		hook := captureLogrus(t)
		assert.NilError(t, ResolveRelativePaths(project(), t.TempDir(), nil))
		assert.Check(t, is.Len(hook.AllEntries(), 1))
		assert.Check(t, is.Equal(message, hook.LastEntry().Message))
	})

	// intent: a nil logger is the default one
	t.Run("nil logger goes through logrus", func(t *testing.T) {
		hook := captureLogrus(t)
		assert.NilError(t, ResolveRelativePathsWithLogger(project(), t.TempDir(), nil, nil))
		assert.Check(t, is.Len(hook.AllEntries(), 1))
	})

	// intent: the logger receives the warning, logrus does not
	t.Run("logger receives it", func(t *testing.T) {
		hook := captureLogrus(t)
		logger := &recordingLogger{}
		assert.NilError(t, ResolveRelativePathsWithLogger(project(), t.TempDir(), nil, logger))
		assert.Check(t, is.DeepEqual([]string{message}, logger.warnings))
		assert.Check(t, is.Len(hook.AllEntries(), 0))
	})
}
