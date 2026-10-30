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

// The tests in this file lock the optional step of the loader that moves to `mounts`
// the `volumes` which the short syntax can't say (Options.ConvertVolumesToMounts):
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#volumes
//
// Spec (proposed): a volume which can't be written in the short syntax is a mount.

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func loadConverting(t *testing.T, content string) (*types.Project, error) {
	t.Helper()
	return loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ConvertVolumesToMounts = true
	})
}

func convertsTo(t *testing.T, input, canonical string) {
	t.Helper()
	p, err := loadConverting(t, input)
	assert.NilError(t, err)
	out, err := p.MarshalYAML()
	assert.NilError(t, err)
	var got, want map[string]any
	assert.NilError(t, yaml.Unmarshal(out, &got))
	assert.NilError(t, yaml.Unmarshal([]byte(canonical), &want), canonical)
	assert.DeepEqual(t, got, want)
}

func TestConvertVolumesToMounts(t *testing.T) {
	convertsTo(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - /data:/simple
      - cache:/cache:nocopy
      - /host:/propagated:ro,z,rshared
      - type: volume
        source: cache
        target: /subpath
        volume:
          subpath: sub
      - type: bind
        source: /data
        target: /colon:in:target
      - type: bind
        source: /data
        target: /exists
        bind:
          create_host_path: false
      - type: tmpfs
        target: /scratch
      - type: image
        source: alpine
        target: /image
volumes:
  cache:
`, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /simple
        bind: {}
      - type: volume
        source: cache
        target: /cache
        volume:
          nocopy: true
      - type: bind
        source: /host
        target: /propagated
        read_only: true
        bind:
          selinux: z
          propagation: rshared
    mounts:
      - type: volume
        source: cache
        target: /subpath
        volume:
          subpath: sub
      - type: bind
        source: /data
        target: /colon:in:target
        bind:
          create_src: true
      - type: bind
        source: /data
        target: /exists
        bind: {}
      - type: tmpfs
        target: /scratch
      - type: image
        source: alpine
        target: /image
volumes:
  cache: {}
`)
}

// What a volume moved to mounts says is kept: the mounts it had, its read only
// flag, and its extensions.
func TestConvertVolumesToMountsKeepsWhatTheVolumeSays(t *testing.T) {
	p, err := loadConverting(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: tmpfs
        target: /mine
    volumes:
      - type: volume
        source: cache
        target: /data
        read_only: true
        x-note: kept
        volume:
          labels:
            tier: cache
volumes:
  cache:
`)
	assert.NilError(t, err)
	foo := p.Services["foo"]
	assert.Equal(t, len(foo.Volumes), 0)
	assert.Equal(t, len(foo.Mounts), 2)
	assert.Equal(t, foo.Mounts[0].Target, "/mine")
	moved := foo.Mounts[1]
	assert.Equal(t, moved.Target, "/data")
	assert.Equal(t, moved.ReadOnly, true)
	assert.DeepEqual(t, moved.Volume.Labels, types.Mapping{"tier": "cache"})
	assert.Equal(t, moved.Extensions["x-note"], "kept")
}

// Jobs and pre_start hooks are containers as well
func TestConvertVolumesToMountsInJobsAndHooks(t *testing.T) {
	p, err := loadConverting(t, `
name: test
services:
  foo:
    image: alpine
    pre_start:
      - image: alpine
        volumes:
          - type: bind
            source: /data
            target: /hook:colon
jobs:
  bar:
    image: alpine
    triggers:
      manual: true
    volumes:
      - type: bind
        source: /data
        target: /job:colon
`)
	assert.NilError(t, err)
	hook := p.Services["foo"].PreStart[0]
	assert.Equal(t, len(hook.Volumes), 0)
	assert.Equal(t, hook.Mounts[0].Target, "/hook:colon")
	job := p.Jobs["bar"]
	assert.Equal(t, len(job.Volumes), 0)
	assert.Equal(t, job.Mounts[0].Target, "/job:colon")
}

// A volume may not be moved to a mount which can't carry what it says: the
// SELinux relabeling of a bind mount has no equivalent there.
func TestConvertVolumesToMountsRefusesWhatAMountCantCarry(t *testing.T) {
	_, err := loadConverting(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /data
        bind:
          selinux: z
          recursive: disabled
`)
	assert.ErrorContains(t, err, `service "foo": volume /data: bind.selinux (z) has no equivalent on a mount`)
}

// The step is optional, and moving what was moved changes nothing
func TestConvertVolumesToMountsIsOptionalAndIdempotent(t *testing.T) {
	input := `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /colon:in:target
`
	kept := load(t, input)
	assert.Equal(t, len(kept.Services["foo"].Volumes), 1)
	assert.Equal(t, len(kept.Services["foo"].Mounts), 0)

	once, err := loadConverting(t, input)
	assert.NilError(t, err)
	assert.NilError(t, loader.ConvertVolumesToMounts(once))
	assert.Equal(t, len(once.Services["foo"].Volumes), 0)
	assert.Equal(t, len(once.Services["foo"].Mounts), 1)

	// the output of `config` loads as the same model
	out, err := once.MarshalYAML()
	assert.NilError(t, err)
	again, err := loadConverting(t, string(out))
	assert.NilError(t, err)
	assert.DeepEqual(t, again.Services["foo"].Mounts, once.Services["foo"].Mounts)
	assert.Equal(t, len(again.Services["foo"].Volumes), 0)
}
