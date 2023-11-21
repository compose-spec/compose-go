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

	"github.com/compose-spec/compose-go/v2/types"
)

func withVersionExampleConfig() *types.Config {
	return &types.Config{
		Services: withVersionServices(),
		Networks: withVersionNetworks(),
		Volumes:  withVersionVolumes(),
	}
}

func withVersionServices() types.Services {
	buildCtx, _ := filepath.Abs("./Dockerfile")
	return types.Services{
		"web": {
			Name: "web",

			Build: &types.BuildConfig{
				Context: buildCtx,
			},
			Environment: types.MappingWithEquals{},
			Networks: map[string]*types.ServiceNetworkConfig{
				"front":   nil,
				"default": nil,
			},
			VolumesFrom: []string{"other"},
		},
		"other": {
			Name: "other",

			Image:       "busybox:1.31.0-uclibc",
			Command:     []string{"top"},
			Environment: types.MappingWithEquals{},
			Volumes: []types.ServiceVolumeConfig{
				{Target: "/data", Type: "volume", Volume: &types.ServiceVolumeVolume{}},
			},
		},
	}
}

func withVersionNetworks() map[string]types.NetworkConfig {
	return map[string]types.NetworkConfig{
		"front": {},
	}
}

func withVersionVolumes() map[string]types.VolumeConfig {
	return map[string]types.VolumeConfig{
		"data": {
			Driver: "local",
		},
	}
}
