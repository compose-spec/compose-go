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
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestResolveComposeFilePaths(t *testing.T) {
	absWorkingDir, _ := filepath.Abs("testdata")
	absComposeFile, _ := filepath.Abs(filepath.Join("testdata", "simple", "compose.yaml"))
	absOverrideFile, _ := filepath.Abs(filepath.Join("testdata", "simple", "compose-with-overrides.yaml"))

	project := types.Project{
		Name:         "myProject",
		WorkingDir:   absWorkingDir,
		ComposeFiles: []string{filepath.Join("testdata", "simple", "compose.yaml"), filepath.Join("testdata", "simple", "compose-with-overrides.yaml")},
	}

	expected := types.Project{
		Name:         "myProject",
		WorkingDir:   absWorkingDir,
		ComposeFiles: []string{absComposeFile, absOverrideFile},
	}
	err := ResolveRelativePaths(&project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}

func TestResolveBuildContextPaths(t *testing.T) {

	yaml := `
name: test-resolve-build-context-paths
services:
  foo:
    build:
      context: ./testdata
      dockerfile: Dockerfile-sample
`
	project, err := loadYAML(yaml)
	assert.NilError(t, err)

	wd, err := os.Getwd()
	assert.NilError(t, err)

	expected := types.BuildConfig{
		Context:    filepath.Join(wd, "testdata"),
		Dockerfile: "Dockerfile-sample",
	}
	assert.DeepEqual(t, expected, *project.Services[0].Build)
}

func TestResolveAdditionalContexts(t *testing.T) {
	abs, err := filepath.Abs("/dir")
	assert.NilError(t, err)
	yaml := fmt.Sprintf(`
name: test-resolve-additional-contexts
services:
  test:
    build:
      context: .
      dockerfile: Dockerfile
      additional_contexts:
        image: docker-image://foo
        oci:  oci-layout://foo
        abs_path: %s
        github:   github.com/compose-spec/compose-go
        rel_path: ./testdata
`, abs)
	project, err := loadYAML(yaml)
	assert.NilError(t, err)

	wd, err := os.Getwd()
	assert.NilError(t, err)

	expected := types.BuildConfig{
		Context:    wd,
		Dockerfile: "Dockerfile",
		AdditionalContexts: map[string]string{
			"image":    "docker-image://foo",
			"oci":      "oci-layout://foo",
			"abs_path": abs,
			"github":   "github.com/compose-spec/compose-go",
			"rel_path": filepath.Join(wd, "testdata"),
		},
	}
	assert.DeepEqual(t, expected, *project.Services[0].Build)
}
