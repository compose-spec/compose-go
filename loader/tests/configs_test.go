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

func TestServiceConfigs(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    configs:
      - config1
      - source: config2
        target: /my_config
        uid: '103'
        gid: '103'
        mode: 0440
configs:
  config1:
    file: ./config_data
  config2:
    external: true
`)
	configs := p.Services["foo"].Configs
	assert.Equal(t, len(configs), 2)
	assert.Equal(t, configs[0].Source, "config1")
	assert.Equal(t, configs[1].Source, "config2")
	assert.Equal(t, configs[1].Target, "/my_config")
	assert.Equal(t, configs[1].UID, "103")
	assert.Equal(t, configs[1].GID, "103")
	assert.Equal(t, *configs[1].Mode, types.FileMode(0o440))
}

func TestTopLevelConfigs(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
configs:
  config1:
    file: ./config_data
    labels:
      foo: bar
  config2:
    external:
      name: my_config
  config3:
    external: true
  config4:
    name: foo
    file: ./config_data
    x-bar: baz
    x-foo: bar
`)
	assert.Equal(t, p.Configs["config2"].Name, "my_config")
	assert.Equal(t, p.Configs["config2"].External, types.External(true))
	assert.Equal(t, p.Configs["config3"].External, types.External(true))
	assert.Equal(t, p.Configs["config4"].Name, "foo")
	assert.DeepEqual(t, p.Configs["config1"].Labels, types.Labels{"foo": "bar"})
	assert.DeepEqual(t, p.Configs["config4"].Extensions, types.Extensions{
		"x-bar": "baz",
		"x-foo": "bar",
	})
}
