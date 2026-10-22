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

package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

func logrusMessages(hook *logrustest.Hook) []string {
	var messages []string
	for _, e := range hook.AllEntries() {
		messages = append(messages, e.Message)
	}
	return messages
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
}

// Warnings raised while discovering files and loading a project are reported
// through the standard logrus logger unless WithLogger is used, in which case
// the logger receives them and logrus stays silent. Each warning of this
// package is covered: several default config files, several default override
// files, an inaccessible default .env file, an unset variable referenced by the
// .env file, a project name taken from a
// symlinked directory, and a warning of the loader.
func TestProjectOptionsWarnings(t *testing.T) {
	tests := []struct {
		name string
		// setup prepares the working directory and returns the options to
		// apply, after WithLogger, to load it; want returns the expected
		// warnings for that working directory
		setup func(t *testing.T, dir string) []ProjectOptionsFn
		want  func(dir string) []string
		// load makes the test load the project, not only apply the options
		load bool
	}{
		{
			name: "several default config files",
			setup: func(t *testing.T, dir string) []ProjectOptionsFn {
				writeFile(t, filepath.Join(dir, "compose.yaml"), "services: {}\n")
				writeFile(t, filepath.Join(dir, "docker-compose.yml"), "services: {}\n")
				return []ProjectOptionsFn{WithWorkingDirectory(dir), WithDefaultConfigPath}
			},
			want: func(dir string) []string {
				return []string{
					"Found multiple config files with supported names: " + filepath.Join(dir, "compose.yaml") + ", " + filepath.Join(dir, "docker-compose.yml"),
					"Using " + filepath.Join(dir, "compose.yaml"),
				}
			},
		},
		{
			name: "several default override files",
			setup: func(t *testing.T, dir string) []ProjectOptionsFn {
				writeFile(t, filepath.Join(dir, "compose.yaml"), "services: {}\n")
				writeFile(t, filepath.Join(dir, "compose.override.yml"), "services: {}\n")
				writeFile(t, filepath.Join(dir, "docker-compose.override.yml"), "services: {}\n")
				return []ProjectOptionsFn{WithWorkingDirectory(dir), WithDefaultConfigPath}
			},
			want: func(dir string) []string {
				return []string{
					"Found multiple override files with supported names: " + filepath.Join(dir, "compose.override.yml") + ", " + filepath.Join(dir, "docker-compose.override.yml"),
					"Using " + filepath.Join(dir, "compose.override.yml"),
				}
			},
		},
		{
			name: "inaccessible default env file",
			setup: func(t *testing.T, dir string) []ProjectOptionsFn {
				if runtime.GOOS == "windows" {
					t.Skip("chmod 0 does not prevent stat on Windows")
				}
				if os.Geteuid() == 0 {
					t.Skip("root bypasses permission checks")
				}
				locked := filepath.Join(dir, "locked")
				assert.NilError(t, os.Mkdir(locked, 0o700))
				writeFile(t, filepath.Join(locked, ".env"), "FOO=bar\n")
				assert.NilError(t, os.Chmod(locked, 0o000))
				t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
				return []ProjectOptionsFn{WithWorkingDirectory(locked), WithEnvFiles()}
			},
			want: func(dir string) []string {
				env := filepath.Join(dir, "locked", ".env")
				return []string{"cannot access " + env + ", ignoring default env file: stat " + env + ": permission denied"}
			},
		},
		{
			name: "unset variable in the .env file",
			setup: func(t *testing.T, dir string) []ProjectOptionsFn {
				writeFile(t, filepath.Join(dir, ".env"), "FOO=${UNSET_VARIABLE}\n")
				return []ProjectOptionsFn{WithWorkingDirectory(dir), WithEnvFiles(), WithDotEnv}
			},
			want: func(string) []string {
				return []string{"The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."}
			},
		},
		{
			name: "project name from a symlink",
			setup: func(t *testing.T, dir string) []ProjectOptionsFn {
				if runtime.GOOS == "windows" {
					t.Skip("symlinks need privileges on Windows")
				}
				target := filepath.Join(dir, "real")
				assert.NilError(t, os.Mkdir(target, 0o700))
				writeFile(t, filepath.Join(target, "compose.yaml"), "services: {}\n")
				link := filepath.Join(dir, "link")
				assert.NilError(t, os.Symlink(target, link))
				return []ProjectOptionsFn{WithWorkingDirectory(link), WithDefaultConfigPath}
			},
			want: func(string) []string {
				return []string{"project has been loaded without an explicit name from a symlink. Using name \"link\""}
			},
			load: true,
		},
		{
			name: "warning of the loader",
			setup: func(t *testing.T, dir string) []ProjectOptionsFn {
				writeFile(t, filepath.Join(dir, "compose.yaml"), "name: warn\nservices:\n  foo:\n    image: busybox\n    environment:\n      FOO: \"${UNSET_VARIABLE}\"\n")
				return []ProjectOptionsFn{WithWorkingDirectory(dir), WithDefaultConfigPath}
			},
			want: func(string) []string {
				return []string{"The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."}
			},
			load: true,
		},
	}
	for _, tc := range tests {
		run := func(t *testing.T, logger *recordingLogger) (dir string) {
			t.Helper()
			dir, err := filepath.EvalSymlinks(t.TempDir())
			assert.NilError(t, err)
			var opts []ProjectOptionsFn
			if logger != nil {
				opts = append(opts, WithLogger(logger))
			}
			opts = append(opts, tc.setup(t, dir)...)
			options, err := NewProjectOptions(nil, opts...)
			assert.NilError(t, err)
			if tc.load {
				_, err = options.LoadProject(t.Context())
				assert.NilError(t, err)
			}
			return dir
		}
		t.Run(tc.name, func(t *testing.T) {
			// intent: with no logger configured, the warning is still
			// written through the standard logrus logger, as it has always been
			t.Run("default goes through logrus", func(t *testing.T) {
				hook := captureLogrus(t)
				dir := run(t, nil)
				assert.Check(t, is.DeepEqual(tc.want(dir), logrusMessages(hook)))
			})

			// intent: with WithLogger, the logger receives the warning and
			// logrus does not
			t.Run("logger receives it", func(t *testing.T) {
				hook := captureLogrus(t)
				logger := &recordingLogger{}
				dir := run(t, logger)
				assert.Check(t, is.DeepEqual(tc.want(dir), logger.warnings))
				assert.Check(t, is.Len(hook.AllEntries(), 0))
			})
		})
	}
}
