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

func TestServiceHooks(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
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
