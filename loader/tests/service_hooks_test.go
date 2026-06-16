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
	assert.DeepEqual(t, p.Services["test"].PreStart, []types.ServiceHook{
		{
			Command:    types.ShellCommand{"./manage.py", "migrate"},
			User:       "root",
			WorkingDir: "/app",
			Environment: types.MappingWithEquals{
				"FOO": ptr("BAR"),
			},
		},
		{
			Image:      "busybox",
			Command:    types.ShellCommand{"sh", "-c", "chown -R 1000:1000 /data"},
			Privileged: true,
			PerReplica: true,
		},
		{
			Image: "migrator:latest",
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
