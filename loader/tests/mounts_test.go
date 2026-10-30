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

// The tests in this file lock the `mounts` attribute, proposed for
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#mounts
//
// Spec (proposed): "`mounts` defines mounts of the container, with the structure
// of the `--mount` flag of `docker run`: unlike an entry of `volumes`, the
// implementation has no latitude on how a mount is made."

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestServiceMounts(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: bind
        source: /data
        target: /bind
        read_only: true
        bind:
          propagation: rshared
          create_src: true
          recursive: disabled
      - type: volume
        source: cache
        target: /cache
        volume:
          nocopy: true
          subpath: sub
          labels:
            tier: cache
          driver: local
          driver_opts:
            type: tmpfs
      - type: tmpfs
        target: /scratch
        tmpfs:
          size: 1m
          mode: "1777"
      - type: image
        source: alpine
        target: /image
        image:
          subpath: /etc
volumes:
  cache:
`)
	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].Mounts, []types.ServiceMountConfig{
			{
				Type: "bind", Source: "/data", Target: "/bind", ReadOnly: true,
				Bind: &types.ServiceMountBind{Propagation: "rshared", CreateSrc: true, Recursive: "disabled"},
			},
			{
				Type: "volume", Source: "cache", Target: "/cache",
				Volume: &types.ServiceMountVolume{
					NoCopy: true, Subpath: "sub", Labels: types.Mapping{"tier": "cache"},
					Driver: "local", DriverOpts: types.Options{"type": "tmpfs"},
				},
			},
			{Type: "tmpfs", Target: "/scratch", Tmpfs: &types.ServiceVolumeTmpfs{Size: 1048576, Mode: 0o1777}},
			{Type: "image", Source: "alpine", Target: "/image", Image: &types.ServiceVolumeImage{SubPath: "/etc"}},
		})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// A bind mount doesn't create its source unless it says so, as `bind-create-src`
// of the flag; the `create_host_path` of a volume is on unless it says not.
func TestMountsDoNotCreateTheHostPathByDefault(t *testing.T) {
	loadsAs(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: bind
        source: /data
        target: /bind
        bind: {}
`, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: bind
        source: /data
        target: /bind
        bind: {}
`)
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    volumes:
      - type: bind
        source: /data
        target: /bind
        bind: {}
`)
	assert.Equal(t, bool(p.Services["foo"].Volumes[0].Bind.CreateHostPath), true)
}

// The target of a mount may hold what a bind string can't: this is what mounts
// are for.
func TestMountTargetWithColonAndComma(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: volume
        source: cache
        target: /cloudsql/project:region:db,ro
volumes:
  cache:
`)
	assert.Equal(t, p.Services["foo"].Mounts[0].Target, "/cloudsql/project:region:db,ro")
}

func TestMountsInJobsAndHooks(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: tmpfs
        target: /service
    pre_start:
      - image: alpine
        mounts:
          - type: tmpfs
            target: /hook
jobs:
  bar:
    image: alpine
    triggers:
      manual: true
    mounts:
      - type: tmpfs
        target: /job
`)
	foo := p.Services["foo"]
	assert.Equal(t, len(foo.Mounts), 1)
	assert.Equal(t, foo.Mounts[0].Target, "/service")
	// a hook keeps its own mounts and does not inherit those of the service
	assert.Equal(t, len(foo.PreStart[0].Mounts), 1)
	assert.Equal(t, foo.PreStart[0].Mounts[0].Target, "/hook")
	assert.Equal(t, p.Jobs["bar"].Mounts[0].Target, "/job")
}

func TestMountsInterpolation(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - type: bind
        source: ${SRC}
        target: /bind
        read_only: ${RO}
        bind:
          create_src: ${CREATE}
      - type: volume
        source: cache
        target: /cache
        volume:
          nocopy: ${NOCOPY}
volumes:
  cache:
`, map[string]string{"SRC": "/data", "RO": "true", "CREATE": "true", "NOCOPY": "true"})
	mounts := p.Services["foo"].Mounts
	assert.Equal(t, mounts[0].Source, "/data")
	assert.Equal(t, mounts[0].ReadOnly, true)
	assert.Equal(t, mounts[0].Bind.CreateSrc, true)
	assert.Equal(t, mounts[1].Volume.NoCopy, true)
}

func TestMountsAreCheckedByTheSchema(t *testing.T) {
	for name, tc := range map[string]struct{ mount, err string }{
		"no short syntax":           {`"/data:/bind"`, "mounts"},
		"unknown option":            {"{type: bind, source: /data, target: /bind, bind: {create_host_path: true}}", "create_host_path"},
		"missing target":            {"{type: bind, source: /data}", "target"},
		"missing type":              {"{source: /data, target: /bind}", "type"},
		"selinux has no equivalent": {"{type: bind, source: /data, target: /bind, bind: {selinux: z}}", "selinux"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - `+tc.mount+`
`)
			assert.Assert(t, strings.Contains(err.Error(), tc.err), "%v", err)
		})
	}
}

// The target of a mount can't be the one of a volume. As for volumes, a mount
// declared twice for a target is one mount: the last declaration is kept.
func TestMountTargetsAreUnique(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mounts:
      - {type: tmpfs, target: /data}
      - {type: volume, source: cache, target: /data}
volumes:
  cache:
`)
	mounts := p.Services["foo"].Mounts
	assert.Equal(t, len(mounts), 1)
	assert.Equal(t, mounts[0].Type, "volume")

	_, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    volumes:
      - cache:/data
    mounts:
      - {type: tmpfs, target: /data}
volumes:
  cache:
`)}},
		Environment: map[string]string{},
	})
	assert.ErrorContains(t, err, "target /data already mounted as services.foo.volumes[0]")
}

func TestMountOfUndefinedVolume(t *testing.T) {
	_, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    mounts:
      - {type: volume, source: missing, target: /data}
`)}},
		Environment: map[string]string{},
	})
	assert.ErrorContains(t, err, "undefined volume missing")
}

// A relative source of a bind mount is relative to the project directory, as the
// one of a volume is
func TestMountBindSourceIsRelativeToTheProjectDirectory(t *testing.T) {
	p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
		WorkingDir: "/project",
		ConfigFiles: []types.ConfigFile{{Filename: "/project/compose.yml", Content: []byte(`
name: test
services:
  foo:
    image: alpine
    mounts:
      - {type: bind, source: ./data, target: /bind}
      - {type: volume, source: cache, target: /cache}
    pre_start:
      - image: alpine
        mounts:
          - {type: bind, source: ./hook, target: /hook}
jobs:
  bar:
    image: alpine
    triggers:
      manual: true
    mounts:
      - {type: bind, source: ./job, target: /job}
volumes:
  cache:
`)}},
		Environment: map[string]string{},
	})
	assert.NilError(t, err)
	foo := p.Services["foo"]
	assert.Equal(t, foo.Mounts[0].Source, filepath.FromSlash("/project/data"))
	assert.Equal(t, foo.Mounts[1].Source, "cache", "a volume name is left alone")
	assert.Equal(t, foo.PreStart[0].Mounts[0].Source, filepath.FromSlash("/project/hook"))
	assert.Equal(t, p.Jobs["bar"].Mounts[0].Source, filepath.FromSlash("/project/job"))
}
