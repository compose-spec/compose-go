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
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestConvertWithEnvVar(t *testing.T) {
	os.Setenv("COMPOSE_CONVERT_WINDOWS_PATHS", "1")
	defer os.Unsetenv("COMPOSE_CONVERT_WINDOWS_PATHS")
	opts, _ := NewProjectOptions([]string{"testdata/simple/compose-with-paths.yaml"},
		WithOsEnv,
		WithWorkingDirectory("C:\\project-dir\\"),
		WithResolvedPaths(true))

	p, err := ProjectFromOptions(opts)

	assert.NilError(t, err)
	assert.Equal(t, len(p.Services[0].Volumes), 3)
	assert.Equal(t, p.Services[0].Volumes[0].Source, "/c/docker/project")
	assert.Equal(t, p.Services[0].Volumes[1].Source, "/c/project-dir/relative")
	assert.Equal(t, p.Services[0].Volumes[2].Source, "/c/project-dir/relative2")
}
