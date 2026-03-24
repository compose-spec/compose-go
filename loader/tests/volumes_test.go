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

import (
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
`)
	vol := p.Services["foo"].Volumes[0]
	assert.Equal(t, vol.Type, "image")
	assert.Equal(t, vol.Source, "app/image")
	assert.Equal(t, vol.Target, "/mnt/image")
	assert.Equal(t, vol.Image.SubPath, "/foo")
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
`)
	vol := p.Services["foo"].Volumes[0]
	assert.Equal(t, vol.Type, "npipe")
	assert.Equal(t, vol.Source, "\\\\.\\pipe\\docker_engine")
	assert.Equal(t, vol.Target, "\\\\.\\pipe\\docker_engine")
}
