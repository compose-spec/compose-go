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
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestServiceHooks(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    pre_start:
      - command: ["./manage.py", "migrate"]
        user: root
        working_dir: /app
        environment:
          - FOO=BAR
      - image: busybox
        command: sh -c 'chown -R 1000:1000 /data'
        privileged: true
        per_replica: true
      - image: migrator:latest
    post_start:
      - command: echo start
        user: root
        privileged: true
        working_dir: /
        environment:
          - FOO=BAR
    pre_stop:
      - command: echo stop
        user: root
        working_dir: /
        environment:
          FOO: BAR
`)
	assert.DeepEqual(t, p.Services["test"].PreStart, []types.PreStartHook{
		{
			ContainerSpec: types.ContainerSpec{
				Command:    types.ShellCommand{"./manage.py", "migrate"},
				User:       "root",
				WorkingDir: "/app",
				Environment: types.MappingWithEquals{
					"FOO": ptr("BAR"),
				},
			},
		},
		{
			ContainerSpec: types.ContainerSpec{
				Image:      "busybox",
				Command:    types.ShellCommand{"sh", "-c", "chown -R 1000:1000 /data"},
				Privileged: true,
			},
			PerReplica: true,
		},
		{
			ContainerSpec: types.ContainerSpec{
				Image: "migrator:latest",
			},
		},
	})
	assert.DeepEqual(t, p.Services["test"].PostStart, []types.ServiceHook{
		{
			Command:    types.ShellCommand{"echo", "start"},
			User:       "root",
			Privileged: true,
			WorkingDir: "/",
			Environment: types.MappingWithEquals{
				"FOO": ptr("BAR"),
			},
		},
	})
	assert.DeepEqual(t, p.Services["test"].PreStop, []types.ServiceHook{
		{
			Command:    types.ShellCommand{"echo", "stop"},
			User:       "root",
			WorkingDir: "/",
			Environment: types.MappingWithEquals{
				"FOO": ptr("BAR"),
			},
		},
	})
}

func TestPreStartInheritsServiceImage(t *testing.T) {
	yaml := `
name: test
services:
  test:
    image: alpine
    pre_start:
      - command: ["migrate"]
      - image: busybox
        command: ["echo", "hi"]
`
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
		Environment: map[string]string{},
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["test"].PreStart[0].Image, "alpine")
	assert.Equal(t, p.Services["test"].PreStart[1].Image, "busybox")
}

// TestPreStartAcceptsContainerSpec locks the compose-spec#656 contract: a
// pre_start hook is a full container specification, so runtime attributes
// (volumes, init, networks, …) load into the hook instead of being dropped.
func TestPreStartAcceptsContainerSpec(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: myapp
    volumes:
      - data:/data:ro
    pre_start:
      - image: busybox
        command: chown -R 1000:1000 /data
        user: root
        init: true
        volumes:
          - data:/data:rw
volumes:
  data: {}
`)
	hook := p.Services["test"].PreStart[0]
	assert.Equal(t, hook.Image, "busybox")
	assert.Equal(t, hook.User, "root")
	assert.Assert(t, hook.Init != nil && *hook.Init)
	assert.Equal(t, len(hook.Volumes), 1)
	assert.Equal(t, hook.Volumes[0].Source, "data")
	assert.Equal(t, hook.Volumes[0].Target, "/data")
	assert.Assert(t, !hook.Volumes[0].ReadOnly, "hook redeclares the volume read-write")
	// the service's own mount is untouched
	assert.Assert(t, p.Services["test"].Volumes[0].ReadOnly)
}

// Exec hooks (post_start/pre_stop) run inside the service container: they
// take a command, not a container specification.
func TestPostStartRejectsContainerSpec(t *testing.T) {
	_, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  test:
    image: alpine
    post_start:
      - image: busybox
        command: echo hi
`)}},
		Environment: map[string]string{},
	})
	assert.ErrorContains(t, err, "additional properties 'image' not allowed")
}

// Interpolation casts are registered per specification layer: container_spec
// attributes cast wherever a container is declared — including pre_start
// hooks, not only services.
func TestLayeredInterpolationCasts(t *testing.T) {
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  test:
    image: alpine
    pre_start:
      - command: setup
        init: ${INIT}
`)}},
		Environment: map[string]string{"INIT": "true"},
	})
	assert.NilError(t, err)
	hook := p.Services["test"].PreStart[0]
	assert.Assert(t, hook.Init != nil && *hook.Init, "hook init must cast to boolean")
}
