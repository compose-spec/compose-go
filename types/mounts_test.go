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

package types

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestServiceVolumeToMount(t *testing.T) {
	tests := []struct {
		name   string
		volume ServiceVolumeConfig
		want   ServiceMountConfig
	}{
		{
			name:   "bind without options creates its source, as a volume does by default",
			volume: ServiceVolumeConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t"},
			want:   ServiceMountConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t", Bind: &ServiceMountBind{CreateSrc: true}},
		},
		{
			name: "bind options",
			volume: ServiceVolumeConfig{
				Type: VolumeTypeBind, Source: "/data", Target: "/t", ReadOnly: true, Consistency: "cached",
				Bind: &ServiceVolumeBind{Propagation: "rshared", CreateHostPath: true, Recursive: "disabled"},
			},
			want: ServiceMountConfig{
				Type: VolumeTypeBind, Source: "/data", Target: "/t", ReadOnly: true, Consistency: "cached",
				Bind: &ServiceMountBind{Propagation: "rshared", CreateSrc: true, Recursive: "disabled"},
			},
		},
		{
			name:   "bind not creating its host path does not create its source",
			volume: ServiceVolumeConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t", Bind: &ServiceVolumeBind{}},
			want:   ServiceMountConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t", Bind: &ServiceMountBind{}},
		},
		{
			name: "volume options",
			volume: ServiceVolumeConfig{
				Type: VolumeTypeVolume, Source: "cache", Target: "/t",
				Volume: &ServiceVolumeVolume{NoCopy: true, Subpath: "sub", Labels: Mapping{"a": "b"}},
			},
			want: ServiceMountConfig{
				Type: VolumeTypeVolume, Source: "cache", Target: "/t",
				Volume: &ServiceMountVolume{NoCopy: true, Subpath: "sub", Labels: Mapping{"a": "b"}},
			},
		},
		{
			name:   "tmpfs and image options",
			volume: ServiceVolumeConfig{Type: VolumeTypeTmpfs, Target: "/t", Tmpfs: &ServiceVolumeTmpfs{Size: 1024, Mode: 0o1777}},
			want:   ServiceMountConfig{Type: VolumeTypeTmpfs, Target: "/t", Tmpfs: &ServiceVolumeTmpfs{Size: 1024, Mode: 0o1777}},
		},
		{
			name:   "extensions",
			volume: ServiceVolumeConfig{Type: VolumeTypeTmpfs, Target: "/t", Extensions: Extensions{"x-a": "b"}},
			want:   ServiceMountConfig{Type: VolumeTypeTmpfs, Target: "/t", Extensions: Extensions{"x-a": "b"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.volume.ToMount()
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestServiceVolumeToMountRefusesSELinux(t *testing.T) {
	_, err := ServiceVolumeConfig{
		Type: VolumeTypeBind, Source: "/data", Target: "/t",
		Bind: &ServiceVolumeBind{SELinux: SELinuxShared},
	}.ToMount()
	assert.ErrorContains(t, err, "volume /t: bind.selinux (z) has no equivalent on a mount")
}
