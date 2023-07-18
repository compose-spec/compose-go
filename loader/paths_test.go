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
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestResolveBuildContextPaths(t *testing.T) {
	wd, _ := os.Getwd()
	project := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				Build: &types.BuildConfig{
					Context:    "./testdata",
					Dockerfile: "Dockerfile-sample",
				},
				Scale: 1,
			},
		},
	}

	expected := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				Build: &types.BuildConfig{
					Context:    filepath.Join(wd, "testdata"),
					Dockerfile: "Dockerfile-sample",
				},
				Scale: 1,
			},
		},
	}
	err := ResolveRelativePaths(&project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}

func TestResolveAdditionalContexts(t *testing.T) {
	wd, _ := filepath.Abs(".")
	absSubdir := filepath.Join(wd, "dir")
	project := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
		Services: types.Services{
			types.ServiceConfig{
				Name: "test",
				Build: &types.BuildConfig{
					Context:    ".",
					Dockerfile: "Dockerfile",
					AdditionalContexts: map[string]string{
						"image":    "docker-image://foo",
						"oci":      "oci-layout://foo",
						"abs_path": absSubdir,
						"github":   "github.com/compose-spec/compose-go",
						"rel_path": "./testdata",
					},
				},
			},
		},
	}

	expected := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
		Services: types.Services{
			types.ServiceConfig{
				Name: "test",
				Build: &types.BuildConfig{
					Context:    wd,
					Dockerfile: "Dockerfile",
					AdditionalContexts: map[string]string{
						"image":    "docker-image://foo",
						"oci":      "oci-layout://foo",
						"abs_path": absSubdir,
						"github":   "github.com/compose-spec/compose-go",
						"rel_path": filepath.Join(wd, "testdata"),
					},
				},
			},
		},
	}
	err := ResolveRelativePaths(&project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}
