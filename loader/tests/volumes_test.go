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
	"fmt"
	"math"
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

// The mode of a tmpfs mount is an unsigned 32 bit number, as the Docker CLI
// reads `tmpfs-mode`, whether it is written as a number (taken as YAML reads
// it) or as octal text: the smallest and the largest values load, and marshal
// to a mode that loads back to the same value.
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#volumes
func TestTmpfsVolumeModeBounds(t *testing.T) {
	for name, tt := range map[string]struct {
		mode string
		want uint32
	}{
		"zero":                   {`0`, 0},
		"octal number":           {`0o1777`, 0o1777},
		"decimal number":         {`1777`, 1777},
		"largest octal number":   {`0o37777777777`, math.MaxUint32},
		"largest decimal number": {`4294967295`, math.MaxUint32},
		"whole float":            {`1e3`, 1000},
		"octal text":             {`"1777"`, 0o1777},
		"octal text, zero first": {`"01777"`, 0o1777},
		"zero text":              {`"0"`, 0},
		"largest octal text":     {`"37777777777"`, math.MaxUint32},
	} {
		t.Run(name, func(t *testing.T) {
			input := `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: tmpfs
        target: /run
        tmpfs:
          mode: ` + tt.mode + `
`
			want := fmt.Sprintf(`
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: tmpfs
        target: /run
        tmpfs:
          mode: %d
`, tt.want)
			if tt.want == 0 {
				// a zero mode is the default one, marshaling drops it
				want = `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: tmpfs
        target: /run
        tmpfs: {}
`
			}
			loadsAs(t, input, want)

			p := load(t, input)
			assert.Equal(t, uint32(p.Services["foo"].Volumes[0].Tmpfs.Mode), tt.want)
			fromYAML, fromJSON := roundTrip(t, p)
			assert.Equal(t, uint32(fromYAML.Services["foo"].Volumes[0].Tmpfs.Mode), tt.want)
			assert.Equal(t, uint32(fromJSON.Services["foo"].Volumes[0].Tmpfs.Mode), tt.want)
		})
	}
}

// The mode of a tmpfs mount is an unsigned 32 bit number, as the Docker CLI
// reads `tmpfs-mode`: a negative number, a number that does not fit in 32 bits
// (it would be silently truncated, 2^32 to 0), a number that is not whole and
// text that is not an octal number that fits in 32 bits (no sign, no `0o`
// prefix) are rejected, with the attribute in the error.
func TestTmpfsVolumeModeInvalid(t *testing.T) {
	for name, value := range map[string]string{
		"a negative number":          `-1`,
		"just beyond 32 bits":        `4294967296`,
		"just beyond 32 bits, octal": `0o40000000000`,
		"far beyond 32 bits":         `0o77777777777`,
		"beyond 64 bits":             `99999999999999999999999`,
		"a fraction":                 `1.5`,
		"a float beyond 32 bits":     `1e10`,
		"text beyond 32 bits":        `"40000000000"`,
		"text beyond 64 bits":        `"7777777777777777777777"`,
		"a negative text":            `"-1"`,
		"a signed text":              `"+1777"`,
		"text with a 0o prefix":      `"0o1777"`,
		"not octal":                  `"0999"`,
		"text":                       `abc`,
		"empty text":                 `""`,
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: tmpfs
        target: /run
        tmpfs:
          mode: `+value+`
`)
			assert.ErrorContains(t, err, "services[foo].volumes[0].tmpfs.mode")
			assert.ErrorContains(t, err, "invalid tmpfs mode")
		})
	}
}
