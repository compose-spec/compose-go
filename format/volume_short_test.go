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

package format

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestShortSyntax(t *testing.T) {
	bind := func(b types.ServiceVolumeBind) types.ServiceVolumeConfig {
		return types.ServiceVolumeConfig{Type: types.VolumeTypeBind, Source: "/data", Target: "/t", Bind: &b}
	}
	volume := func(vol *types.ServiceVolumeVolume) types.ServiceVolumeConfig {
		return types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Source: "vol", Target: "/t", Volume: vol}
	}
	with := func(v types.ServiceVolumeConfig, change func(*types.ServiceVolumeConfig)) types.ServiceVolumeConfig {
		change(&v)
		return v
	}
	create := types.ServiceVolumeBind{CreateHostPath: true}

	tests := []struct {
		name   string
		volume types.ServiceVolumeConfig
		short  string // empty when the short syntax can't say it
	}{
		{"bind", bind(create), "/data:/t:rw"},
		{"bind without options", types.ServiceVolumeConfig{Type: types.VolumeTypeBind, Source: "/data", Target: "/t"}, "/data:/t:rw"},
		{"bind read only", with(bind(create), func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "/data:/t:ro"},
		{"bind selinux", bind(types.ServiceVolumeBind{CreateHostPath: true, SELinux: "z"}), "/data:/t:rw,z"},
		{"bind propagation", bind(types.ServiceVolumeBind{CreateHostPath: true, Propagation: "rshared"}), "/data:/t:rw,rshared"},
		{"bind selinux, propagation and read only", with(bind(types.ServiceVolumeBind{CreateHostPath: true, SELinux: "Z", Propagation: "rslave"}),
			func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "/data:/t:ro,Z,rslave"},
		{"relative bind", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "./data" }), "./data:/t:rw"},
		{"bind not creating the host path", bind(types.ServiceVolumeBind{}), ""},
		{"bind recursive", bind(types.ServiceVolumeBind{CreateHostPath: true, Recursive: "disabled"}), ""},
		{"bind source that is not a path", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "data" }), ""},
		{"bind without source", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "" }), ""},
		{"bind with nocopy", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Volume = &types.ServiceVolumeVolume{NoCopy: true} }), ""},
		{"bind with a colon in the target", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Target = "/t:x" }), ""},
		{"bind with a colon in the source", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "/da:ta" }), ""},
		{"bind with a consistency", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Consistency = "cached" }), ""},
		{"bind with an extension", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Extensions = types.Extensions{"x-a": true} }), ""},
		{"named volume", volume(nil), "vol:/t:rw"},
		{"named volume with empty options", volume(&types.ServiceVolumeVolume{}), "vol:/t:rw"},
		{"named volume read only", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "vol:/t:ro"},
		{"named volume nocopy", volume(&types.ServiceVolumeVolume{NoCopy: true}), "vol:/t:rw,nocopy"},
		{"named volume with a one letter name, read as a drive", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Source = "v" }), ""},
		{"named volume subpath", volume(&types.ServiceVolumeVolume{Subpath: "a"}), ""},
		{"named volume labels", volume(&types.ServiceVolumeVolume{Labels: types.Mapping{"a": "b"}}), ""},
		{"named volume with a bind option", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Bind = &types.ServiceVolumeBind{Propagation: "rshared"} }), ""},
		{"named volume with a colon in the target", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Target = "/t:x" }), ""},
		{"named volume named like a path", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Source = "./v" }), ""},
		{"anonymous volume", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Target: "/t"}, "/t"},
		{"anonymous volume read only", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Target: "/t", ReadOnly: true}, ""},
		{"anonymous volume nocopy", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Target: "/t", Volume: &types.ServiceVolumeVolume{NoCopy: true}}, ""},
		{"no target", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Source: "vol"}, ""},
		{"tmpfs", types.ServiceVolumeConfig{Type: types.VolumeTypeTmpfs, Target: "/t"}, ""},
		{"image", types.ServiceVolumeConfig{Type: types.VolumeTypeImage, Source: "alpine", Target: "/t"}, ""},
		{"named pipe", types.ServiceVolumeConfig{Type: types.VolumeTypeNamedPipe, Source: `\\.\pipe\a`, Target: `\\.\pipe\b`}, ""},
		{"cluster", types.ServiceVolumeConfig{Type: types.VolumeTypeCluster, Source: "vol", Target: "/t"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			short, ok := ShortSyntax(tt.volume)
			assert.Equal(t, ok, tt.short != "", "short syntax: %q", short)
			assert.Equal(t, short, tt.short)
			assert.Equal(t, IsShortSyntaxExpressible(tt.volume), tt.short != "")
		})
	}
}

// What the short syntax renders reads back as the volume it was rendered from.
func TestShortSyntaxRoundTrip(t *testing.T) {
	for _, spec := range []string{
		"/data:/t", "./data:/t:ro", "~/data:/t:z", "/data:/t:rw,Z,rshared", "v:/t", "vol:/t:ro", "v:/t:nocopy", "/t",
	} {
		t.Run(spec, func(t *testing.T) {
			volume, err := ParseVolume(spec)
			assert.NilError(t, err)
			short, ok := ShortSyntax(volume)
			assert.Assert(t, ok, "%s is not expressible", spec)
			again, err := ParseVolume(short)
			assert.NilError(t, err)
			assert.DeepEqual(t, again, volume)
		})
	}
}
