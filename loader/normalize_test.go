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
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gopkg.in/yaml.v2"
	"gotest.tools/v3/assert"
)

func TestNormalizeNetworkNames(t *testing.T) {
	project := types.Project{
		Name:       "myProject",
		WorkingDir: "/some/path",
		Environment: map[string]string{
			"FOO": "BAR",
		},
		Networks: types.Networks{
			"myExternalnet": {
				Name:     "myExternalnet", // this is automaticaly setup by loader for externa networks before reaching normalization
				External: types.External{External: true},
			},
			"mynet": {},
			"myNamedNet": {
				Name: "CustomName",
			},
		},
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				Build: &types.BuildConfig{
					Context: "foo",
					Args: map[string]*string{
						"FOO": nil,
						"ZOT": nil,
					},
				},
			},
		},
	}

	expected := strings.ReplaceAll(`services:
  foo:
    build:
      context: /some/path/foo
      dockerfile: /some/path/foo/Dockerfile
      args:
        FOO: BAR
        ZOT: null
    networks:
      default: null
networks:
  default:
    name: myProject_default
  myExternalnet:
    name: myExternalnet
    external: true
  myNamedNet:
    name: CustomName
  mynet:
    name: myProject_mynet
`, "/", string(filepath.Separator))
	err := normalize(&project)
	assert.NilError(t, err)
	marshal, err := yaml.Marshal(project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, string(marshal))
}

func TestNormalizeVolumes(t *testing.T) {
	project := types.Project{
		Name:     "myProject",
		Networks: types.Networks{},
		Volumes: types.Volumes{
			"myExternalVol": {
				Name:     "myExternalVol", // this is automaticaly setup by loader for externa networks before reaching normalization
				External: types.External{External: true},
			},
			"myvol": {},
			"myNamedVol": {
				Name: "CustomName",
			},
		},
	}

	expected := types.Project{
		Name:     "myProject",
		Networks: types.Networks{"default": {Name: "myProject_default"}},
		Volumes: types.Volumes{
			"myExternalVol": {
				Name:     "myExternalVol",
				External: types.External{External: true},
			},
			"myvol": {Name: "myProject_myvol"},
			"myNamedVol": {
				Name: "CustomName",
			},
		},
	}
	err := normalize(&project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}
