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

package compatibility

import (
	"testing"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestWhiteList(t *testing.T) {
	var checker Checker = &WhiteList{
		Supported: []string{
			"services.network_mode",
			"services.privileged",
			"services.networks",
		},
	}
	dict, err := loader.ParseYAML([]byte(`
version: "3"
services:
  foo:
    image: busybox
    network_mode: host
    privileged: true
    mac_address: "a:b:c:d"
`))
	assert.NilError(t, err)

	project, err := loader.Load(types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "filename.yml", Config: dict},
		},
	})
	assert.NilError(t, err)

	checker.Check(project)
	errors := checker.Errors()
	assert.Check(t, len(errors) == 1)
	assert.Check(t, errdefs.IsUnsupportedError(errors[0]))
	assert.Equal(t, errors[0].Error(), "services.mac_address: unsupported attribute")

	service, err := project.GetService("foo")
	assert.NilError(t, err)
	assert.Check(t, service.MacAddress == "")
}
