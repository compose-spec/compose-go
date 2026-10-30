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

package tests

// The tests in this file lock the `volumes`, `volumes (top-level)` attributes:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#volumes
//   https://github.com/compose-spec/compose-spec/blob/main/07-volumes.md
//
// Spec: "`volumes` define mount host paths or named volumes that are
// accessible by service containers."

import (
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestTopLevelVolumes(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
volumes:
  some-volume:
  other-volume:
    driver: flocker
    driver_opts:
      foo: "bar"
      baz: 1
    labels:
      foo: bar
  another-volume:
    name: "user_specified_name"
    driver: vsphere
    driver_opts:
      foo: "bar"
      baz: 1
  external-volume:
    external: true
  other-external-volume:
    external:
      name: my-cool-volume
  external-volume3:
    name: this-is-volume3
    external: true
    x-bar: baz
    x-foo: bar
`)

	expect := func(p *types.Project) {
		// Simple volume
		assert.Equal(t, p.Volumes["some-volume"].Driver, "")

		// Volume with driver and opts
		other := p.Volumes["other-volume"]
		assert.Equal(t, other.Driver, "flocker")
		assert.DeepEqual(t, other.DriverOpts, types.Options{"foo": "bar", "baz": "1"})
		assert.DeepEqual(t, other.Labels, types.Labels{"foo": "bar"})

		// Named volume
		assert.Equal(t, p.Volumes["another-volume"].Name, "user_specified_name")
		assert.Equal(t, p.Volumes["another-volume"].Driver, "vsphere")

		// External volumes
		assert.Equal(t, p.Volumes["external-volume"].External, types.External(true))
		assert.Equal(t, p.Volumes["other-external-volume"].Name, "my-cool-volume")
		assert.Equal(t, p.Volumes["other-external-volume"].External, types.External(true))
		assert.Equal(t, p.Volumes["external-volume3"].Name, "this-is-volume3")
		assert.Equal(t, p.Volumes["external-volume3"].External, types.External(true))
	}
	expect(p)
	assert.DeepEqual(t, p.Volumes["external-volume3"].Extensions, types.Extensions{
		"x-bar": "baz",
		"x-foo": "bar",
	})

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestImageVolume(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: image
        source: app/image
        target: /mnt/image
        image:
          subpath: /foo
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    volumes:
      - type: image
        source: app/image
        target: /mnt/image
        image:
          subpath: /foo
`)
	vol := p.Services["foo"].Volumes[0]
	assert.Equal(t, vol.Type, "image")
	assert.Equal(t, vol.Source, "app/image")
	assert.Equal(t, vol.Target, "/mnt/image")
	assert.Equal(t, vol.Image.SubPath, "/foo")

	jvol := p.Jobs["foo"].Volumes[0]
	assert.Equal(t, jvol.Type, "image")
	assert.Equal(t, jvol.Source, "app/image")
	assert.Equal(t, jvol.Target, "/mnt/image")
	assert.Equal(t, jvol.Image.SubPath, "/foo")
}

func TestNpipeVolume(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: npipe
        source: \\.\pipe\docker_engine
        target: \\.\pipe\docker_engine
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    volumes:
      - type: npipe
        source: \\.\pipe\docker_engine
        target: \\.\pipe\docker_engine
`)
	vol := p.Services["foo"].Volumes[0]
	assert.Equal(t, vol.Type, "npipe")
	assert.Equal(t, vol.Source, "\\\\.\\pipe\\docker_engine")
	assert.Equal(t, vol.Target, "\\\\.\\pipe\\docker_engine")

	jvol := p.Jobs["foo"].Volumes[0]
	assert.Equal(t, jvol.Type, "npipe")
	assert.Equal(t, jvol.Source, "\\\\.\\pipe\\docker_engine")
	assert.Equal(t, jvol.Target, "\\\\.\\pipe\\docker_engine")
}

// Spec, short syntax: "`[SOURCE:]TARGET[:MODE]`, where `SOURCE` can be either a
// host path or volume name." The short syntax is parsed without knowing the
// platform of the engine, as `docker run -v` does: a single letter followed by
// a colon always is a Windows drive. `a:/data` is therefore not the volume `a`
// but an anonymous volume on the path `a:/data`, and a volume named with a
// single letter has to be declared with the long syntax.
func TestVolumeShortSyntaxSingleLetterDrive(t *testing.T) {
	loadsAs(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - a:/data
      - 'C:\Users\me:/home'
      - C:/Users/me:/work:ro
      - type: volume
        source: a
        target: /logs
`, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - a:/data
      - 'C:\Users\me:/home'
      - C:/Users/me:/work:ro
      - type: volume
        source: a
        target: /logs
`)
}

// The `mode` of a `tmpfs` mount accepts the same values as the Docker CLI: a
// number is read as written, and a string is read as an octal number, as
// `--mount type=tmpfs,tmpfs-mode=1777` does. Both are canonicalized to a
// number, whether the mount is declared by a service or by a job.
//
// Spec: "mode: the file mode for the tmpfs mount as Unix permission bits as an
// octal number", a number or a string in the schema:
//
//	https://github.com/compose-spec/compose-spec/blob/main/container_spec.md#volumes
func TestTmpfsMode(t *testing.T) {
	// the canonical form of a job states its name
	document := func(mode string, canonical ...bool) string {
		jobName := ""
		if len(canonical) > 0 {
			jobName = `
    name: foo`
		}
		const container = `
    image: alpine
    volumes:
      - type: tmpfs
        target: /tmp
        tmpfs:
          mode: `
		return `
name: test
services:
  foo:` + container + mode + `
jobs:
  foo:
    triggers:
      manual: true` + jobName + container + mode
	}

	for _, tc := range []struct {
		name, mode, want string
	}{
		{"octal string", `"1777"`, "1023"},
		{"octal string with a leading zero", `"01777"`, "1023"},
		{"octal string of a regular mode", `"0755"`, "493"},
		{"largest octal string", `"37777777777"`, "4294967295"},
		{"YAML octal integer", `01777`, "1023"},
		{"YAML integer is decimal", `1777`, "1777"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loadsAs(t, document(tc.mode), document(tc.want, true))
		})
	}

	// A zero mode is the default one, which the canonical form omits.
	t.Run("zero string", func(t *testing.T) {
		p := load(t, document(`"0"`))
		assert.Equal(t, p.Services["foo"].Volumes[0].Tmpfs.Mode, uint32(0))
	})

	// An octal string reaching the model through an interpolation is read the
	// same way as one written in the file.
	t.Run("interpolated octal string", func(t *testing.T) {
		p := loadWithEnv(t, document("${MODE}"), map[string]string{"MODE": "1777"})
		assert.Equal(t, p.Services["foo"].Volumes[0].Tmpfs.Mode, uint32(0o1777))
		assert.Equal(t, p.Jobs["foo"].Volumes[0].Tmpfs.Mode, uint32(0o1777))
	})

	// The canonical number marshals back to the same mode, in YAML and JSON.
	t.Run("round trip", func(t *testing.T) {
		p := load(t, document(`"1777"`))
		fromYAML, fromJSON := roundTrip(t, p)
		for _, rt := range []*types.Project{fromYAML, fromJSON} {
			assert.Equal(t, rt.Services["foo"].Volumes[0].Tmpfs.Mode, uint32(0o1777))
			assert.Equal(t, rt.Jobs["foo"].Volumes[0].Tmpfs.Mode, uint32(0o1777))
		}
	})

	// The mode is read the same way when the mount comes from a merged file or
	// from an extended service.
	t.Run("merge", func(t *testing.T) {
		loadFilesAs(t, []string{document(`"0755"`), document(`"1777"`)}, document("1023", true))
	})
	t.Run("extends", func(t *testing.T) {
		loadsAs(t, `
name: test
services:
  base:
    image: alpine
    volumes:
      - type: tmpfs
        target: /tmp
        tmpfs:
          mode: "1777"
  foo:
    extends:
      service: base
`, `
name: test
services:
  base:
    image: alpine
    volumes:
      - type: tmpfs
        target: /tmp
        tmpfs:
          mode: 1023
  foo:
    image: alpine
    volumes:
      - type: tmpfs
        target: /tmp
        tmpfs:
          mode: 1023
`)
	})

	// A string the Docker CLI rejects (not an octal number, a sign, a prefix,
	// a blank) is rejected too, with the path of the attribute (the service or the
	// job, whichever is transformed first).
	for _, tc := range []struct{ name, mode string }{
		{"not a number", `"abc"`},
		{"negative", `"-1"`},
		{"0o prefix", `"0o1777"`},
		{"trailing blank", `"1777 "`},
		{"not an octal digit", `"1778"`},
		{"empty", `""`},
	} {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			err := loadErr(t, document(tc.mode))
			assert.ErrorContains(t, err, ".foo.volumes.[].tmpfs.mode")
			assert.ErrorContains(t, err, "must be an octal number")
		})
	}
}

// A volume the short syntax can say is written as a short string, whatever the
// syntax it was declared with, and the others keep the long syntax: the short
// syntax is the canonical form, the long one what it cannot be. The model that
// is loaded stays the same, and the JSON form is left structured.
func TestVolumesCanonicalForm(t *testing.T) {
	input := `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /a
        read_only: true
        bind:
          create_host_path: true
          selinux: z
          propagation: rshared
      - type: volume
        source: cache
        target: /b
        volume:
          nocopy: true
      - type: volume
        target: /c
      - /d:/d
      - type: bind
        source: /data
        target: /not-created
        bind:
          create_host_path: false
      - type: volume
        source: cache
        target: /subpath
        volume:
          subpath: sub
      - type: bind
        source: /data
        target: /colon:in:target
      - type: tmpfs
        target: /scratch
volumes:
  cache:
`
	loadsAs(t, input, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - /data:/a:ro,z,rshared
      - cache:/b:nocopy
      - /c
      - /d:/d
      - type: bind
        source: /data
        target: /not-created
        bind:
          create_host_path: false
      - type: volume
        source: cache
        target: /subpath
        volume:
          subpath: sub
      - type: bind
        source: /data
        target: /colon:in:target
      - type: tmpfs
        target: /scratch
volumes:
  cache: {}
`)

	// what is written reads back as the same project, and as the same text
	p := load(t, input)
	out, err := p.MarshalYAML()
	assert.NilError(t, err)
	again := load(t, string(out))
	assert.DeepEqual(t, again.Services["foo"].Volumes, p.Services["foo"].Volumes)
	outAgain, err := again.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, string(outAgain), string(out))

	// the JSON form, which the hash of a service is computed from, doesn't move
	js, err := p.MarshalJSON()
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(js), `"type": "volume"`), string(js))
	assert.Assert(t, !strings.Contains(string(js), `/data:/a`), string(js))
}
