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
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestConvertWindowsVolumePath(t *testing.T) {
	testcases := []struct {
		windowsPath           string
		expectedConvertedPath string
	}{
		{
			windowsPath:           "c:\\hello\\docker",
			expectedConvertedPath: "/c/hello/docker",
		},
		{
			windowsPath:           "d:\\compose",
			expectedConvertedPath: "/d/compose",
		},
		{
			windowsPath:           "e:\\path with spaces\\compose",
			expectedConvertedPath: "/e/path with spaces/compose",
		},
	}
	for _, testcase := range testcases {
		volume := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: testcase.windowsPath,
			Target: "/test",
		}

		assert.Equal(t, testcase.expectedConvertedPath, convertVolumePath(volume).Source)
	}
}

func TestConvertWindowsMountPath(t *testing.T) {
	project := &types.Project{Services: types.Services{"foo": {ContainerSpec: types.ContainerSpec{
		Volumes: []types.ServiceVolumeConfig{{Type: types.VolumeTypeBind, Source: "d:\\compose", Target: "/volume"}},
		Mounts: []types.ServiceMountConfig{
			{Type: types.VolumeTypeBind, Source: "c:\\hello\\docker", Target: "/test"},
			{Type: types.VolumeTypeImage, Source: "a:1", Target: "/image"},
		},
	}}}}

	convertWindowsPaths(project)

	service := project.Services["foo"]
	assert.Equal(t, service.Volumes[0].Source, "/d/compose")
	assert.Equal(t, service.Mounts[0].Source, "/c/hello/docker")
	assert.Equal(t, service.Mounts[1].Source, "a:1")
}
