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
// the `volumes` that the function it is given says so of (Options.ConvertVolumesToMounts),
// the ones which the short syntax can't say being the obvious case:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#volumes
//
// Spec (proposed): a volume which can't be written in the short syntax is a mount, a loader
// being told so; what else is one depends on the engine, which the loader leaves to its caller.

import (
	"errors"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// onlyTheLongSyntax is the obvious case: a volume that the short syntax can't say
func onlyTheLongSyntax(volume types.ServiceVolumeConfig) (bool, error) {
	_, ok := volume.ShortSyntax()
	return !ok, nil
}

func loadConverting(t *testing.T, content string) (*types.Project, error) {
	t.Helper()
	return loadConvertingWith(t, content, onlyTheLongSyntax)
}

func loadConvertingWith(t *testing.T, content string, convert func(types.ServiceVolumeConfig) (bool, error)) (*types.Project, error) {
	t.Helper()
	return loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(content)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ConvertVolumesToMounts = convert
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
      - cache:/relabeled:z
      - type: volume
        source: cache
        target: /relabeled-long
        bind:
          selinux: z
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
      - /data:/simple
      - cache:/cache:nocopy
      - /host:/propagated:ro,z,rshared
      - cache:/relabeled:z
      - type: volume
        source: cache
        target: /relabeled-long
        bind:
          selinux: z
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
	assert.NilError(t, loader.ConvertVolumesToMounts(once, onlyTheLongSyntax))
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

// What makes a volume a mount depends on the engine that runs it: the function decides, volume by volume,
// and what it doesn't say so of stays in `volumes`, even when the short syntax can't say it. The decision of
// the one used here is the obvious case, for which the extensions are no reason to move a volume.
func TestConvertVolumesToMountsLeavesTheDecisionToTheCaller(t *testing.T) {
	input := `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /noted
        x-note: kept
      - type: bind
        source: /data
        target: /colon:in:target
      - /data:/plain
`
	t.Run("an extension doesn't make the volume a mount", func(t *testing.T) {
		p, err := loadConvertingWith(t, input, onlyTheLongSyntax)
		assert.NilError(t, err)
		foo := p.Services["foo"]
		assert.Equal(t, len(foo.Volumes), 2)
		assert.Equal(t, foo.Volumes[0].Target, "/noted")
		assert.Equal(t, foo.Volumes[0].Extensions["x-note"], "kept")
		assert.Equal(t, len(foo.Mounts), 1)
		assert.Equal(t, foo.Mounts[0].Target, "/colon:in:target")
	})

	t.Run("a volume that the short syntax can say may be moved all the same", func(t *testing.T) {
		p, err := loadConvertingWith(t, input, func(volume types.ServiceVolumeConfig) (bool, error) {
			return volume.Target == "/plain", nil
		})
		assert.NilError(t, err)
		foo := p.Services["foo"]
		assert.Equal(t, len(foo.Volumes), 2)
		assert.Equal(t, len(foo.Mounts), 1)
		assert.Equal(t, foo.Mounts[0].Target, "/plain")
		assert.Equal(t, foo.Mounts[0].Type, types.VolumeTypeBind)
		assert.Equal(t, foo.Mounts[0].Bind.CreateSrc, true)
	})

	t.Run("a volume the function refuses to move stays, whatever the short syntax can say", func(t *testing.T) {
		p, err := loadConvertingWith(t, input, func(types.ServiceVolumeConfig) (bool, error) { return false, nil })
		assert.NilError(t, err)
		assert.Equal(t, len(p.Services["foo"].Volumes), 3)
		assert.Equal(t, len(p.Services["foo"].Mounts), 0)
	})

	t.Run("it is a mount that can't carry what the volume sets that fails, not the function", func(t *testing.T) {
		_, err := loadConvertingWith(t, input+`      - {type: bind, source: /data, target: /selinux, bind: {selinux: z}}
`, func(types.ServiceVolumeConfig) (bool, error) { return true, nil })
		assert.ErrorContains(t, err, `volume /selinux: bind.selinux (z) has no equivalent on a mount`)
	})
}

// The function sees the volumes as a normalized project has them, which is what a caller gets: a bind
// that creates its host path by default is one the short syntax can say, and stays.
func TestConvertVolumesToMountsSeesNormalizedVolumes(t *testing.T) {
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /kept
      - type: bind
        source: /data
        target: /colon:in:target
`)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.ConvertVolumesToMounts = onlyTheLongSyntax
	})
	assert.NilError(t, err)
	foo := p.Services["foo"]
	assert.Equal(t, len(foo.Volumes), 1)
	assert.Equal(t, foo.Volumes[0].Target, "/kept")
	assert.Equal(t, len(foo.Mounts), 1)
	assert.Equal(t, foo.Mounts[0].Target, "/colon:in:target")
}

// What adds volumes to a project after it was loaded calls the function itself: it reaches the services and
// jobs the profiles disabled, and does nothing without a function to decide.
func TestConvertVolumesToMountsOfAProjectAlreadyLoaded(t *testing.T) {
	p := load(t, `
name: test
services:
  off:
    image: alpine
    profiles: [never]
    volumes:
      - type: bind
        source: /data
        target: /colon:in:target
jobs:
  idle:
    image: alpine
    profiles: [never]
    triggers:
      manual: true
    volumes:
      - type: bind
        source: /data
        target: /job:colon
`)
	assert.Equal(t, len(p.DisabledServices["off"].Volumes), 1)
	assert.Equal(t, len(p.DisabledJobs["idle"].Volumes), 1)

	assert.NilError(t, loader.ConvertVolumesToMounts(p, nil))
	assert.Equal(t, len(p.DisabledServices["off"].Volumes), 1)
	assert.Equal(t, len(p.DisabledServices["off"].Mounts), 0)

	assert.NilError(t, loader.ConvertVolumesToMounts(p, onlyTheLongSyntax))
	assert.Equal(t, len(p.DisabledServices["off"].Volumes), 0)
	assert.Equal(t, p.DisabledServices["off"].Mounts[0].Target, "/colon:in:target")
	assert.Equal(t, len(p.DisabledJobs["idle"].Volumes), 0)
	assert.Equal(t, p.DisabledJobs["idle"].Mounts[0].Target, "/job:colon")
}

// The function may refuse a volume, which is how an implementation says what its engine can't take: the
// load fails with the error, the container and the volume it is about.
func TestConvertVolumesToMountsReturnsTheErrorOfTheFunction(t *testing.T) {
	_, err := loadConvertingWith(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - /data:/fine
      - cache:/refused:rshared
volumes:
  cache:
`, func(volume types.ServiceVolumeConfig) (bool, error) {
		if volume.Target == "/refused" {
			return false, errors.New("the propagation of a volume is not for the engine")
		}
		return false, nil
	})
	assert.ErrorContains(t, err, `service "foo": volume /refused: the propagation of a volume is not for the engine`)
}

// All is converted or nothing is: a project whose conversion fails is as it was, the hooks and the
// mounts it had included, so that the caller who keeps it after the error keeps a model that is valid.
func TestConvertVolumesToMountsLeavesTheProjectAsItWasOnError(t *testing.T) {
	p := load(t, `
name: test
services:
  a:
    image: alpine
    mounts:
      - {type: tmpfs, target: /mine}
    volumes:
      - type: bind
        source: /data
        target: /colon:in:target
    pre_start:
      - image: alpine
        volumes:
          - type: bind
            source: /data
            target: /hook:colon
  b:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /refused
jobs:
  idle:
    image: alpine
    triggers:
      manual: true
    volumes:
      - type: bind
        source: /data
        target: /job:colon
`)
	before, err := p.MarshalYAML()
	assert.NilError(t, err)

	err = loader.ConvertVolumesToMounts(p, func(volume types.ServiceVolumeConfig) (bool, error) {
		if volume.Target == "/refused" {
			return false, errors.New("refused")
		}
		return onlyTheLongSyntax(volume)
	})
	assert.ErrorContains(t, err, `service "b": volume /refused: refused`)

	after, err := p.MarshalYAML()
	assert.NilError(t, err)
	assert.Equal(t, string(after), string(before))
	assert.Equal(t, len(p.Services["a"].Volumes), 1)
	assert.Equal(t, len(p.Services["a"].Mounts), 1)
	assert.Equal(t, len(p.Services["a"].PreStart[0].Volumes), 1)
	assert.Equal(t, len(p.Services["a"].PreStart[0].Mounts), 0)
	assert.Equal(t, len(p.Jobs["idle"].Volumes), 1)

	// and without the refusal, the same project converts
	assert.NilError(t, loader.ConvertVolumesToMounts(p, onlyTheLongSyntax))
	assert.Equal(t, len(p.Services["a"].Volumes), 0)
	assert.Equal(t, len(p.Services["a"].Mounts), 2)
	assert.Equal(t, len(p.Services["a"].PreStart[0].Mounts), 1)
	assert.Equal(t, len(p.Jobs["idle"].Mounts), 1)
}
