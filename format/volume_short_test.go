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

	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v2/types"
)

// A volume is expressible in the short syntax when some spelling of it, `[SOURCE:]TARGET[:OPTIONS]`, parses back as
// the very same volume. What the short syntax has no place for, or would read otherwise, is not: options of the long
// syntax only, and paths with a colon (https://github.com/docker/compose/issues/12993).
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
		{"bind", bind(create), "/data:/t"},
		{"bind without options", types.ServiceVolumeConfig{Type: types.VolumeTypeBind, Source: "/data", Target: "/t"}, "/data:/t"},
		{"bind read only", with(bind(create), func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "/data:/t:ro"},
		{"bind selinux", bind(types.ServiceVolumeBind{CreateHostPath: true, SELinux: "z"}), "/data:/t:z"},
		{"bind propagation", bind(types.ServiceVolumeBind{CreateHostPath: true, Propagation: "rshared"}), "/data:/t:rshared"},
		{"bind selinux, propagation and read only", with(bind(types.ServiceVolumeBind{CreateHostPath: true, SELinux: "Z", Propagation: "rslave"}),
			func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "/data:/t:ro,Z,rslave"},
		{"relative bind", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "./data" }), "./data:/t"},
		{"bind of a home directory", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "~/data" }), "~/data:/t"},
		{"bind on a windows drive", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source, v.Target = `C:\data`, `C:\t` }), `C:\data:C:\t`},
		{"bind with an unknown selinux option", bind(types.ServiceVolumeBind{CreateHostPath: true, SELinux: "foo"}), ""},
		{"bind not creating the host path", bind(types.ServiceVolumeBind{}), ""},
		{"bind recursive", bind(types.ServiceVolumeBind{CreateHostPath: true, Recursive: "disabled"}), ""},
		{"bind source that is not a path", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "data" }), ""},
		{"bind without source", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "" }), ""},
		{"bind with nocopy", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Volume = &types.ServiceVolumeVolume{NoCopy: true} }), ""},
		{"bind with a colon in the target", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Target = "/t:x" }), ""},
		{"bind with a colon in the source", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Source = "/da:ta" }), ""},
		{"bind with a consistency", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Consistency = "cached" }), ""},
		{"bind with an extension", with(bind(create), func(v *types.ServiceVolumeConfig) { v.Extensions = types.Extensions{"x-a": true} }), ""},
		{"named volume", volume(nil), "vol:/t"},
		{"named volume with empty options", volume(&types.ServiceVolumeVolume{}), "vol:/t"},
		{"named volume read only", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "vol:/t:ro"},
		{"named volume nocopy", volume(&types.ServiceVolumeVolume{NoCopy: true}), "vol:/t:nocopy"},
		{"named volume with a one letter name, read as a drive", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Source = "v" }), ""},
		{"named volume read only and nocopy", with(volume(&types.ServiceVolumeVolume{NoCopy: true}), func(v *types.ServiceVolumeConfig) { v.ReadOnly = true }), "vol:/t:ro,nocopy"},
		{"named volume subpath", volume(&types.ServiceVolumeVolume{Subpath: "a"}), ""},
		{"named volume labels", volume(&types.ServiceVolumeVolume{Labels: types.Mapping{"a": "b"}}), ""},
		{"named volume selinux", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Bind = &types.ServiceVolumeBind{SELinux: "z"} }), "vol:/t:z"},
		{"named volume propagation", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Bind = &types.ServiceVolumeBind{Propagation: "rshared"} }), "vol:/t:rshared"},
		// the loader sets create_host_path on every bind, a volume's too, where it means nothing
		{"named volume selinux with the defaulted create_host_path", with(volume(nil), func(v *types.ServiceVolumeConfig) {
			v.Bind = &types.ServiceVolumeBind{SELinux: "Z", CreateHostPath: true}
		}), "vol:/t:Z"},
		{"named volume selinux and nocopy", with(volume(&types.ServiceVolumeVolume{NoCopy: true}), func(v *types.ServiceVolumeConfig) {
			v.Bind = &types.ServiceVolumeBind{SELinux: "z"}
		}), "vol:/t:z,nocopy"},
		{"named volume bind recursive", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Bind = &types.ServiceVolumeBind{Recursive: "disabled"} }), ""},
		{"named volume bind extension", with(volume(nil), func(v *types.ServiceVolumeConfig) {
			v.Bind = &types.ServiceVolumeBind{Extensions: types.Extensions{"x-a": true}}
		}), ""},
		{"named volume with a colon in the target", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Target = "/t:x" }), ""},
		// https://github.com/docker/compose/issues/12993
		{"named volume at a path made of colons", with(volume(nil), func(v *types.ServiceVolumeConfig) {
			v.Target = "/cloudsql/bar-baz:europe-west4:foo-db-1234567890"
		}), ""},
		{"anonymous volume with a colon in the target", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Target: "/t:x"}, ""},
		{"named volume named like a path", with(volume(nil), func(v *types.ServiceVolumeConfig) { v.Source = "./v" }), ""},
		{"anonymous volume", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Target: "/t"}, "/t"},
		{"anonymous volume with an empty options block", types.ServiceVolumeConfig{Type: types.VolumeTypeVolume, Target: "/t", Volume: &types.ServiceVolumeVolume{}}, "/t"},
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
		"/data:/t", "./data:/t:ro", "~/data:/t:z", "/data:/t:rw,Z,rshared", "v:/t", "vol:/t:ro", "v:/t:nocopy", "/t", "vol:/t:z", "vol:/t:ro,Z,nocopy",
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
