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

package transform

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

func legacyExternal(name string) map[string]any {
	return map[string]any{"external": map[string]any{"name": name}}
}

// Canonical rewrites the model in place: each run needs its own.

// A resource declaring `external.name` raises a deprecation warning, whatever
// the section it belongs to, in the order of the names of the resources. A
// resource that does not use the deprecated attribute raises none.
func TestCanonicalWarnsOfDeprecatedExternalName(t *testing.T) {
	for name, tc := range map[string]struct {
		model func() map[string]any
		want  []string
	}{
		"each section": {
			model: func() map[string]any {
				return map[string]any{
					"volumes":  map[string]any{"v": legacyExternal("a")},
					"networks": map[string]any{"n": legacyExternal("b")},
					"secrets":  map[string]any{"s": legacyExternal("c")},
					"configs":  map[string]any{"c": legacyExternal("d")},
				}
			},
			want: []string{
				"volumes.v: external.name is deprecated. Please set name and external: true",
				"networks.n: external.name is deprecated. Please set name and external: true",
				"secrets.s: external.name is deprecated. Please set name and external: true",
				"configs.c: external.name is deprecated. Please set name and external: true",
			},
		},
		"resources in the order of their names": {
			model: func() map[string]any {
				return map[string]any{
					"volumes": map[string]any{"b": legacyExternal("x"), "a": legacyExternal("y"), "c": legacyExternal("z")},
				}
			},
			want: []string{
				"volumes.a: external.name is deprecated. Please set name and external: true",
				"volumes.b: external.name is deprecated. Please set name and external: true",
				"volumes.c: external.name is deprecated. Please set name and external: true",
			},
		},
		"external as a boolean": {
			model: func() map[string]any {
				return map[string]any{
					"volumes": map[string]any{"v": map[string]any{"external": true}},
				}
			},
		},
		"external without a name": {
			model: func() map[string]any {
				return map[string]any{
					"volumes": map[string]any{"v": map[string]any{"external": map[string]any{}}},
				}
			},
		},
		"not external": {
			model: func() map[string]any {
				return map[string]any{
					"volumes": map[string]any{"v": map[string]any{"driver": "local"}, "w": nil},
				}
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			// intent: with no logger, the warnings are still written through
			// the standard logrus logger, as they have always been
			t.Run("default goes through logrus", func(t *testing.T) {
				hook := captureLogrus(t)
				_, err := Canonical(tc.model(), false)
				assert.NilError(t, err)
				var got []string
				for _, e := range hook.AllEntries() {
					got = append(got, e.Message)
				}
				assert.Check(t, is.DeepEqual(tc.want, got))
			})

			// intent: with a logger, it receives the warnings and logrus
			// does not
			t.Run("logger receives it", func(t *testing.T) {
				hook := captureLogrus(t)
				logger := &recordingLogger{}
				_, err := CanonicalWithLogger(tc.model(), false, logger)
				assert.NilError(t, err)
				assert.Check(t, is.DeepEqual(tc.want, logger.warnings))
				assert.Check(t, is.Len(hook.AllEntries(), 0))
			})
		})
	}
}

// The legacy name still conflicts with the name of the resource, and the
// warning is raised all the same.
func TestCanonicalExternalNameConflictStillWarns(t *testing.T) {
	logger := &recordingLogger{}
	_, err := CanonicalWithLogger(map[string]any{
		"volumes": map[string]any{
			"v": map[string]any{"name": "one", "external": map[string]any{"name": "other"}},
		},
	}, false, logger)
	assert.ErrorContains(t, err, "name and external.name conflict")
	assert.Check(t, is.DeepEqual([]string{"volumes.v: external.name is deprecated. Please set name and external: true"}, logger.warnings))
}
