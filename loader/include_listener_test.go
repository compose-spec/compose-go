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
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
	"gotest.tools/v3/assert"
)

// TestInclude_EmitsIncludeListenerEvent reproduces the
// docker/compose TestPublishChecks/refuse_to_publish_with_local_include
// regression: consumers must be notified of every include entry via the
// "include" Listener event so they can act on the presence of local
// includes (e.g. refuse publishing).
func TestInclude_EmitsIncludeListenerEvent(t *testing.T) {
	tmpdir := t.TempDir()
	subdir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.MkdirAll(subdir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(subdir, "compose.yaml"), []byte("services:\n  inc:\n    image: alpine\n"), 0o644))

	topYaml := `
include:
  - path: sub/compose.yaml
`
	topPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(topPath, []byte(topYaml), 0o644))

	var events []map[string]any
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
	}, withProjectName("test-include-listener", true), func(opts *Options) {
		opts.Listeners = append(opts.Listeners, func(event string, metadata map[string]any) {
			if event == "include" {
				events = append(events, metadata)
			}
		})
	})
	assert.NilError(t, err)

	assert.Equal(t, len(events), 1, "expected exactly one 'include' event")
	paths, ok := events[0]["path"].([]string)
	assert.Assert(t, ok, "event must carry path as []string")
	assert.Equal(t, len(paths), 1)
	assert.Equal(t, paths[0], filepath.Join(tmpdir, "sub", "compose.yaml"))
	wd, _ := events[0]["workingdir"].(string)
	assert.Equal(t, wd, filepath.Join(tmpdir, "sub"))
}
