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
	"path"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestConvertWithEnvVar(t *testing.T) {
	workdir, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	if !strings.HasPrefix(workdir, "C:\\") {
		t.Skip("Test assumes C: as drive")
	}

	t.Setenv("COMPOSE_CONVERT_WINDOWS_PATHS", "1")
	opts, err := NewProjectOptions([]string{"testdata/simple/compose-with-paths.yaml"},
		WithOsEnv,
		WithWorkingDirectory(workdir),
		WithResolvedPaths(true))
	assert.NilError(t, err)

	p, err := ProjectFromOptions(opts)

	assert.NilError(t, err)
	assert.Equal(t, len(p.Services[0].Volumes), 3)
	assert.Equal(t, p.Services[0].Volumes[0].Source, "/c/docker/project")

	unixStyleWorkdir := filepath.ToSlash("/c/" + strings.TrimPrefix(workdir, "C:\\"))
	assert.Equal(t, p.Services[0].Volumes[1].Source, path.Join(unixStyleWorkdir, "relative"))
	assert.Equal(t, p.Services[0].Volumes[2].Source, path.Join(unixStyleWorkdir, "relative2"))
}
