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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	is "gotest.tools/v3/assert/cmp"

	"github.com/compose-spec/compose-go/types"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestNormalizeNetworkNames(t *testing.T) {
	wd, _ := os.Getwd()
	project := types.Project{
		Name:       "myProject",
		WorkingDir: wd,
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
					Context: "./testdata",
					Args: map[string]*string{
						"FOO": nil,
						"ZOT": nil,
					},
				},
				Scale: 1,
			},
		},
	}

	expected := `name: myProject
services:
  foo:
    build:
      context: ./testdata
      dockerfile: Dockerfile
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
`
	err := Normalize(&project)
	assert.NilError(t, err)
	marshal, err := project.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, expected, string(marshal))
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

	absCwd, _ := filepath.Abs(".")
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
		WorkingDir: absCwd,
	}
	err := Normalize(&project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project)
}

func TestNormalizeDependsOn(t *testing.T) {
	project := types.Project{
		Name:     "myProject",
		Networks: types.Networks{},
		Volumes:  types.Volumes{},
		Services: []types.ServiceConfig{
			{
				Name: "foo",
				DependsOn: map[string]types.ServiceDependency{
					"bar": { // explicit depends_on never should be overridden
						Condition: types.ServiceConditionHealthy,
						Restart:   false,
					},
				},
				NetworkMode: "service:zot",
			},
			{
				Name: "bar",
				VolumesFrom: []string{
					"zot",
					"container:xxx",
				},
			},
			{
				Name: "zot",
			},
		},
	}

	expected := `name: myProject
services:
  bar:
    depends_on:
      zot:
        condition: service_started
    networks:
      default: null
    volumes_from:
      - zot
      - container:xxx
  foo:
    depends_on:
      bar:
        condition: service_healthy
      zot:
        condition: service_started
        restart: true
    network_mode: service:zot
  zot:
    networks:
      default: null
networks:
  default:
    name: myProject_default
`
	err := Normalize(&project)
	assert.NilError(t, err)
	marshal, err := project.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, expected, string(marshal))
}

func TestNormalizeImplicitDependencies(t *testing.T) {
	project := types.Project{
		Name: "myProject",
		Services: types.Services{
			types.ServiceConfig{
				Name:        "test",
				Ipc:         "service:foo",
				Cgroup:      "service:bar",
				Uts:         "service:baz",
				Pid:         "service:qux",
				VolumesFrom: []string{"quux"},
				Links:       []string{"corge"},
				DependsOn: map[string]types.ServiceDependency{
					// explicit dependency MUST not be overridden
					"foo": {Condition: types.ServiceConditionHealthy, Restart: false},
				},
			},
		},
	}

	expected := types.DependsOnConfig{
		"foo":   {Condition: types.ServiceConditionHealthy, Restart: false},
		"bar":   {Condition: types.ServiceConditionStarted, Restart: true},
		"baz":   {Condition: types.ServiceConditionStarted, Restart: true},
		"qux":   {Condition: types.ServiceConditionStarted, Restart: true},
		"quux":  {Condition: types.ServiceConditionStarted},
		"corge": {Condition: types.ServiceConditionStarted, Restart: true},
	}
	err := Normalize(&project)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, project.Services[0].DependsOn)
}

func TestImplicitContextPath(t *testing.T) {
	logOut := redirectLogger(t)

	project := &types.Project{
		Name: "myProject",
		Services: types.Services{
			types.ServiceConfig{
				Name:  "test",
				Build: &types.BuildConfig{},
			},
		},
	}

	assert.NilError(t, Normalize(project))
	assert.Equal(t, ".", project.Services[0].Build.Context)
	assert.Check(t, is.Contains(logOut.String(), "service test: using `.` as implicit default for build.context."))
}

// redirectLogger sets the package logger to one that writes to a buffer for the duration of the test.
//
// This is not safe to use in conjunction with `t.Parallel()`.
func redirectLogger(t testing.TB) *bytes.Buffer {
	origLogger := log
	t.Cleanup(func() {
		log = origLogger
	})
	var out bytes.Buffer
	log = logrus.NewEntry(&logrus.Logger{
		Out:       &out,
		Level:     logrus.TraceLevel,
		Formatter: new(logrus.TextFormatter),
		Hooks:     make(logrus.LevelHooks),
	})
	return &out
}
