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

	"go.yaml.in/yaml/v4"

	"gotest.tools/v3/assert"
)

func TestShortSyntax(t *testing.T) {
	bind := func(b ServiceVolumeBind) ServiceVolumeConfig {
		return ServiceVolumeConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t", Bind: &b}
	}
	volume := func(vol *ServiceVolumeVolume) ServiceVolumeConfig {
		return ServiceVolumeConfig{Type: VolumeTypeVolume, Source: "vol", Target: "/t", Volume: vol}
	}
	with := func(v ServiceVolumeConfig, change func(*ServiceVolumeConfig)) ServiceVolumeConfig {
		change(&v)
		return v
	}
	create := ServiceVolumeBind{CreateHostPath: true}

	tests := []struct {
		name   string
		volume ServiceVolumeConfig
		short  string // empty when the short syntax can't say it
	}{
		{"bind", bind(create), "/data:/t"},
		{"bind without options", ServiceVolumeConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t"}, "/data:/t"},
		{"bind read only", with(bind(create), func(v *ServiceVolumeConfig) { v.ReadOnly = true }), "/data:/t:ro"},
		{"bind selinux", bind(ServiceVolumeBind{CreateHostPath: true, SELinux: "z"}), "/data:/t:z"},
		{"bind propagation", bind(ServiceVolumeBind{CreateHostPath: true, Propagation: "rshared"}), "/data:/t:rshared"},
		{"bind selinux, propagation and read only", with(bind(ServiceVolumeBind{CreateHostPath: true, SELinux: "Z", Propagation: "rslave"}),
			func(v *ServiceVolumeConfig) { v.ReadOnly = true }), "/data:/t:ro,Z,rslave"},
		{"relative bind", with(bind(create), func(v *ServiceVolumeConfig) { v.Source = "./data" }), "./data:/t"},
		{"bind not creating the host path", bind(ServiceVolumeBind{}), ""},
		{"bind recursive", bind(ServiceVolumeBind{CreateHostPath: true, Recursive: "disabled"}), ""},
		{"bind source that is not a path", with(bind(create), func(v *ServiceVolumeConfig) { v.Source = "data" }), ""},
		{"bind without source", with(bind(create), func(v *ServiceVolumeConfig) { v.Source = "" }), ""},
		{"bind with nocopy", with(bind(create), func(v *ServiceVolumeConfig) { v.Volume = &ServiceVolumeVolume{NoCopy: true} }), ""},
		{"bind with a colon in the target", with(bind(create), func(v *ServiceVolumeConfig) { v.Target = "/t:x" }), ""},
		{"bind with a colon in the source", with(bind(create), func(v *ServiceVolumeConfig) { v.Source = "/da:ta" }), ""},
		{"bind with a consistency", with(bind(create), func(v *ServiceVolumeConfig) { v.Consistency = "cached" }), ""},
		{"bind with an extension", with(bind(create), func(v *ServiceVolumeConfig) { v.Extensions = Extensions{"x-a": true} }), ""},
		{"named volume", volume(nil), "vol:/t"},
		{"named volume with empty options", volume(&ServiceVolumeVolume{}), "vol:/t"},
		{"named volume read only", with(volume(nil), func(v *ServiceVolumeConfig) { v.ReadOnly = true }), "vol:/t:ro"},
		{"named volume nocopy", volume(&ServiceVolumeVolume{NoCopy: true}), "vol:/t:nocopy"},
		{"named volume with a one letter name, read as a drive", with(volume(nil), func(v *ServiceVolumeConfig) { v.Source = "v" }), ""},
		{"named volume subpath", volume(&ServiceVolumeVolume{Subpath: "a"}), ""},
		{"named volume labels", volume(&ServiceVolumeVolume{Labels: Mapping{"a": "b"}}), ""},
		{"named volume selinux", with(volume(nil), func(v *ServiceVolumeConfig) { v.Bind = &ServiceVolumeBind{SELinux: "z"} }), "vol:/t:z"},
		{"named volume propagation", with(volume(nil), func(v *ServiceVolumeConfig) { v.Bind = &ServiceVolumeBind{Propagation: "rshared"} }), "vol:/t:rshared"},
		// the loader sets create_host_path on every bind, a volume's too, where it means nothing
		{"named volume selinux with the defaulted create_host_path", with(volume(nil), func(v *ServiceVolumeConfig) {
			v.Bind = &ServiceVolumeBind{SELinux: "Z", CreateHostPath: true}
		}), "vol:/t:Z"},
		{"named volume selinux and nocopy", with(volume(&ServiceVolumeVolume{NoCopy: true}), func(v *ServiceVolumeConfig) {
			v.Bind = &ServiceVolumeBind{SELinux: "z"}
		}), "vol:/t:z,nocopy"},
		{"named volume bind recursive", with(volume(nil), func(v *ServiceVolumeConfig) { v.Bind = &ServiceVolumeBind{Recursive: "disabled"} }), ""},
		{"named volume bind extension", with(volume(nil), func(v *ServiceVolumeConfig) { v.Bind = &ServiceVolumeBind{Extensions: Extensions{"x-a": true}} }), ""},
		{"named volume with a colon in the target", with(volume(nil), func(v *ServiceVolumeConfig) { v.Target = "/t:x" }), ""},
		{"named volume named like a path", with(volume(nil), func(v *ServiceVolumeConfig) { v.Source = "./v" }), ""},
		{"anonymous volume", ServiceVolumeConfig{Type: VolumeTypeVolume, Target: "/t"}, "/t"},
		{"anonymous volume read only", ServiceVolumeConfig{Type: VolumeTypeVolume, Target: "/t", ReadOnly: true}, ""},
		{"anonymous volume nocopy", ServiceVolumeConfig{Type: VolumeTypeVolume, Target: "/t", Volume: &ServiceVolumeVolume{NoCopy: true}}, ""},
		{"no target", ServiceVolumeConfig{Type: VolumeTypeVolume, Source: "vol"}, ""},
		{"tmpfs", ServiceVolumeConfig{Type: VolumeTypeTmpfs, Target: "/t"}, ""},
		{"image", ServiceVolumeConfig{Type: VolumeTypeImage, Source: "alpine", Target: "/t"}, ""},
		{"named pipe", ServiceVolumeConfig{Type: VolumeTypeNamedPipe, Source: `\\.\pipe\a`, Target: `\\.\pipe\b`}, ""},
		{"cluster", ServiceVolumeConfig{Type: VolumeTypeCluster, Source: "vol", Target: "/t"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			short, ok := tt.volume.ShortSyntax()
			assert.Equal(t, ok, tt.short != "", "short syntax: %q", short)
			assert.Equal(t, short, tt.short)
			assert.Equal(t, tt.volume.IsShortSyntaxExpressible(), tt.short != "")
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
			short, ok := volume.ShortSyntax()
			assert.Assert(t, ok, "%s is not expressible", spec)
			again, err := ParseVolume(short)
			assert.NilError(t, err)
			assert.DeepEqual(t, again, volume)
		})
	}
}

func TestServiceVolumeMarshalYAML(t *testing.T) {
	short, err := yaml.Marshal(ServiceVolumeConfig{
		Type: VolumeTypeBind, Source: "/data", Target: "/t", ReadOnly: true, Bind: &ServiceVolumeBind{CreateHostPath: true},
	})
	assert.NilError(t, err)
	assert.Equal(t, string(short), "/data:/t:ro\n")

	long, err := yaml.Marshal(ServiceVolumeConfig{
		Type: VolumeTypeVolume, Source: "vol", Target: "/t", Volume: &ServiceVolumeVolume{Subpath: "sub"},
	})
	assert.NilError(t, err)
	assert.Equal(t, string(long), "type: volume\nsource: vol\ntarget: /t\nvolume:\n    subpath: sub\n")
}

// A volume is written as a short string only if the short syntax reads back with
// the option blocks it has: the JSON of a service, which its hash is computed
// from, doesn't move when its configuration is written and read again.
func TestServiceVolumeMarshalYAMLKeepsTheOptionBlocks(t *testing.T) {
	named := func(vol *ServiceVolumeVolume, bind *ServiceVolumeBind) ServiceVolumeConfig {
		return ServiceVolumeConfig{Type: VolumeTypeVolume, Source: "vol", Target: "/t", Volume: vol, Bind: bind}
	}
	host := func(bind *ServiceVolumeBind, vol *ServiceVolumeVolume) ServiceVolumeConfig {
		return ServiceVolumeConfig{Type: VolumeTypeBind, Source: "/data", Target: "/t", Bind: bind, Volume: vol}
	}
	const (
		longVolume = "type: volume\nsource: vol\ntarget: /t\n"
		longBind   = "type: bind\nsource: /data\ntarget: /t\n"
	)
	for name, tt := range map[string]struct {
		volume ServiceVolumeConfig
		want   string
	}{
		"volume block read back":      {named(&ServiceVolumeVolume{}, nil), "vol:/t\n"},
		"no volume block":             {named(nil, nil), longVolume},
		"bind block read back":        {named(&ServiceVolumeVolume{}, &ServiceVolumeBind{SELinux: "z", CreateHostPath: true}), "vol:/t:z\n"},
		"bind block not read back":    {named(&ServiceVolumeVolume{}, &ServiceVolumeBind{CreateHostPath: true}), longVolume + "bind: {}\nvolume: {}\n"},
		"only a bind block":           {named(nil, &ServiceVolumeBind{SELinux: "z"}), longVolume + "bind:\n    selinux: z\n    create_host_path: false\n"},
		"named, create_host_path off": {named(&ServiceVolumeVolume{}, &ServiceVolumeBind{SELinux: "z"}), longVolume + "bind:\n    selinux: z\n    create_host_path: false\nvolume: {}\n"},
		"anonymous, short target":     {ServiceVolumeConfig{Type: VolumeTypeVolume, Target: "/t", Volume: &ServiceVolumeVolume{}}, "type: volume\ntarget: /t\nvolume: {}\n"},
		"anonymous, short, no block":  {ServiceVolumeConfig{Type: VolumeTypeVolume, Target: "/t"}, "/t\n"},
		"anonymous, volume block":     {ServiceVolumeConfig{Type: VolumeTypeVolume, Target: "/tmp", Volume: &ServiceVolumeVolume{}}, "/tmp\n"},
		"bind without its block":      {host(nil, nil), longBind},
		"bind with a volume block":    {host(&ServiceVolumeBind{CreateHostPath: true}, &ServiceVolumeVolume{}), longBind + "bind: {}\nvolume: {}\n"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := yaml.Marshal(tt.volume)
			assert.NilError(t, err)
			assert.Equal(t, string(out), tt.want)
		})
	}
}
