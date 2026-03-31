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

func TestVolumeBindPropagation(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /host
        target: /container
        bind:
          propagation: rslave
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Volumes[0].Bind.Propagation, "rslave")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestVolumeBindSELinux(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /host
        target: /container
        bind:
          selinux: z
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Volumes[0].Bind.SELinux, "z")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestVolumeNoCopy(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: volume
        source: mydata
        target: /data
        volume:
          nocopy: true
volumes:
  mydata:
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].Volumes[0].Volume.NoCopy, true)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestVolumeBindCreateHostPathDefault(t *testing.T) {
	p := load(t, `
name: test
services:
  short:
    image: alpine
    volumes:
      - /host:/container
  long:
    image: alpine
    volumes:
      - type: bind
        source: /host
        target: /container
        bind: {}
`)
	assert.Check(t, p.Services["short"].Volumes[0].Bind.CreateHostPath == true)
	assert.Check(t, p.Services["long"].Volumes[0].Bind.CreateHostPath == true)
}
