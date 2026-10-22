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

	"github.com/compose-spec/compose-go/v2/types"
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

// Loading warnings are reported through the standard logrus logger unless
// Options.Logger is set, in which case the logger receives them and logrus
// stays silent. Each category of warning raised while loading is covered:
// obsolete version, YAML 1.1 booleans (cast during interpolation and when
// decoding the model), unset variables (including in a default value) and
// a misconfigured list of resource loaders.
func TestLoadWarnings(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		options func(*Options)
		// want lists the expected warnings; {file} stands for the name of the
		// loaded file, unique to each run as the obsolete version warning is
		// emitted once per file name for the whole process
		want []string
	}{
		{
			name: "obsolete version",
			yaml: "version: \"3.8\"\nname: warn\nservices:\n  foo:\n    image: busybox\n",
			want: []string{"{file}: the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion"},
		},
		{
			name: "YAML 1.1 true boolean",
			yaml: "name: warn\nservices:\n  foo:\n    image: busybox\n    read_only: \"yes\"\n",
			want: []string{"\"yes\" for boolean is not supported by YAML 1.2, please use `true`"},
		},
		{
			name: "YAML 1.1 false boolean",
			yaml: "name: warn\nservices:\n  foo:\n    image: busybox\n    read_only: \"off\"\n",
			want: []string{"\"off\" for boolean is not supported by YAML 1.2, please use `false`"},
		},
		{
			name: "YAML 1.1 boolean decoded without interpolation",
			yaml: "name: warn\nservices:\n  foo:\n    image: busybox\n    read_only: \"on\"\n",
			options: func(o *Options) {
				o.SkipInterpolation = true
				o.SkipValidation = true
			},
			want: []string{"\"on\" for boolean is not supported by YAML 1.2, please use `true`"},
		},
		{
			name: "unset variable",
			yaml: "name: warn\nservices:\n  foo:\n    image: ${UNSET_VARIABLE}\n",
			want: []string{"The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."},
		},
		{
			name: "unset variable in a default value",
			yaml: "name: warn\nservices:\n  foo:\n    image: ${OTHER_UNSET:-${UNSET_VARIABLE}}\n",
			want: []string{"The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."},
		},
		{
			name: "deprecated external.name",
			yaml: "name: warn\nservices:\n  foo:\n    image: busybox\nvolumes:\n  data:\n    external:\n      name: other\n",
			want: []string{"volumes.data: external.name is deprecated. Please set name and external: true"},
		},
		{
			name: "YAML 1.1 boolean in an extension",
			yaml: "name: warn\nx-flag: \"yes\"\nservices:\n  foo:\n    image: busybox\n",
			options: func(o *Options) {
				o.KnownExtensions = map[string]any{"x-flag": true}
			},
			want: []string{"\"yes\" for boolean is not supported by YAML 1.2, please use `true`"},
		},
	}
	for _, tc := range tests {
		load := func(t *testing.T, logger *recordingLogger) {
			t.Helper()
			details := buildConfigDetails(tc.yaml, nil)
			details.ConfigFiles[0].Filename = t.Name() + ".yml"
			_, err := LoadWithContext(t.Context(), details, func(o *Options) {
				o.SkipConsistencyCheck = true
				o.SkipNormalization = true
				if tc.options != nil {
					tc.options(o)
				}
				if logger != nil {
					o.Logger = logger
				}
			})
			assert.NilError(t, err)
		}
		expected := func(t *testing.T) []string {
			t.Helper()
			want := make([]string, len(tc.want))
			for i, w := range tc.want {
				want[i] = strings.ReplaceAll(w, "{file}", t.Name()+".yml")
			}
			return want
		}

		t.Run(tc.name, func(t *testing.T) {
			// intent: with no Logger configured, the warning is still written
			// through the standard logrus logger, as it has always been
			t.Run("default goes through logrus", func(t *testing.T) {
				hook := captureLogrus(t)
				load(t, nil)
				assert.Check(t, is.DeepEqual(expected(t), logrusMessages(hook)))
			})

			// intent: with a Logger configured, it receives the warning and
			// logrus does not
			t.Run("logger receives it", func(t *testing.T) {
				hook := captureLogrus(t)
				logger := &recordingLogger{}
				load(t, logger)
				assert.Check(t, is.DeepEqual(expected(t), logger.warnings))
				assert.Check(t, is.Len(hook.AllEntries(), 0))
			})
		})
	}
}

// A localResourceLoader that is not the last resource loader is a
// misconfiguration reported as a warning, routed like the other ones.
func TestResourceLoadersMisconfigurationWarning(t *testing.T) {
	const message = "misconfiguration of ResourceLoaders: localResourceLoader should be last"
	misconfigured := func(o *Options) {
		o.ResourceLoaders = []ResourceLoader{localResourceLoader{}, localResourceLoader{}}
	}

	// intent: no Logger, no change: the warning goes through logrus
	t.Run("default goes through logrus", func(t *testing.T) {
		hook := captureLogrus(t)
		opts := &Options{}
		misconfigured(opts)
		opts.RemoteResourceLoaders()
		assert.Check(t, is.DeepEqual([]string{message}, logrusMessages(hook)))
	})

	// intent: with a Logger, it receives the warning and logrus does not
	t.Run("logger receives it", func(t *testing.T) {
		hook := captureLogrus(t)
		logger := &recordingLogger{}
		opts := &Options{Logger: logger}
		misconfigured(opts)
		opts.RemoteResourceLoaders()
		assert.Check(t, is.DeepEqual([]string{message}, logger.warnings))
		assert.Check(t, is.Len(hook.AllEntries(), 0))
	})
}

// The Logger set on the options reaches the files loaded through include and
// extends, which are loaded with a copy of the options.
func TestLoggerIsInheritedByIncludedFiles(t *testing.T) {
	const message = "The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string."
	tests := []struct {
		name  string
		main  string
		other string
	}{
		{
			name:  "include",
			main:  "name: warn\ninclude:\n  - other.yaml\n",
			other: "services:\n  foo:\n    image: \"${UNSET_VARIABLE}\"\n",
		},
		{
			name:  "extends file",
			main:  "name: warn\nservices:\n  foo:\n    extends:\n      file: other.yaml\n      service: base\n",
			other: "services:\n  base:\n    image: \"${UNSET_VARIABLE}\"\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(tc.main), 0o600))
			assert.NilError(t, os.WriteFile(filepath.Join(dir, "other.yaml"), []byte(tc.other), 0o600))
			hook := captureLogrus(t)
			logger := &recordingLogger{}

			_, err := LoadWithContext(t.Context(), types.ConfigDetails{
				WorkingDir:  dir,
				ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
				Environment: map[string]string{},
			}, func(o *Options) {
				o.SkipConsistencyCheck = true
				o.Logger = logger
			})
			assert.NilError(t, err)
			assert.Check(t, is.DeepEqual([]string{message}, logger.warnings))
			assert.Check(t, is.Len(hook.AllEntries(), 0))
		})
	}
}

// The warnings raised while resolving the entries of the include section are
// routed like the other ones: an unset variable referenced by the env_file of
// an entry. (The entries have no boolean attribute, so the YAML 1.1 boolean
// warning cannot be raised there today; the decoding of the entries is given
// the logger all the same.)
func TestIncludeEntryWarnings(t *testing.T) {
	tests := []struct {
		name string
		main string
		want string
	}{
		{
			name: "unset variable in an env_file",
			main: "name: warn\ninclude:\n  - path: other.yaml\n    env_file: include.env\n",
			want: "The \"UNSET_VARIABLE\" variable is not set. Defaulting to a blank string.",
		},
	}
	for _, tc := range tests {
		run := func(t *testing.T, logger *recordingLogger) {
			t.Helper()
			dir := t.TempDir()
			assert.NilError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(tc.main), 0o600))
			assert.NilError(t, os.WriteFile(filepath.Join(dir, "other.yaml"), []byte("services: {}\n"), 0o600))
			assert.NilError(t, os.WriteFile(filepath.Join(dir, "include.env"), []byte("FOO=${UNSET_VARIABLE}\n"), 0o600))
			_, err := LoadWithContext(t.Context(), types.ConfigDetails{
				WorkingDir:  dir,
				ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
				Environment: map[string]string{},
			}, func(o *Options) {
				o.SkipConsistencyCheck = true
				if logger != nil {
					o.Logger = logger
				}
			})
			assert.NilError(t, err)
		}
		t.Run(tc.name, func(t *testing.T) {
			// intent: with no Logger configured, the warning is still written
			// through the standard logrus logger, as it has always been
			t.Run("default goes through logrus", func(t *testing.T) {
				hook := captureLogrus(t)
				run(t, nil)
				assert.Check(t, is.DeepEqual([]string{tc.want}, logrusMessages(hook)))
			})

			// intent: with a Logger configured, it receives the warning and
			// logrus does not
			t.Run("logger receives it", func(t *testing.T) {
				hook := captureLogrus(t)
				logger := &recordingLogger{}
				run(t, logger)
				assert.Check(t, is.DeepEqual([]string{tc.want}, logger.warnings))
				assert.Check(t, is.Len(hook.AllEntries(), 0))
			})
		})
	}
}
