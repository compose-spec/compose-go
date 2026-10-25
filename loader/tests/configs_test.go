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

// The tests in this file lock the `configs`, `configs (top-level)` attributes:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#configs
//   https://github.com/compose-spec/compose-spec/blob/main/08-configs.md
//
// Spec: "Configs allow services to adapt their behaviour without the need to
// rebuild a Docker image."

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
jobs:
  foo:
    triggers:
      manual: true
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

	jobConfigs := p.Jobs["foo"].Configs
	assert.Equal(t, len(jobConfigs), 2)
	assert.Equal(t, jobConfigs[0].Source, "config1")
	assert.Equal(t, jobConfigs[1].Source, "config2")
	assert.Equal(t, jobConfigs[1].Target, "/my_config")
	assert.Equal(t, jobConfigs[1].UID, "103")
	assert.Equal(t, jobConfigs[1].GID, "103")
	assert.Equal(t, *jobConfigs[1].Mode, types.FileMode(0o440))
}

func TestTopLevelConfigs(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
jobs:
  foo:
    triggers:
      manual: true
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

// The mode of a config is read as the mode of a secret, with the syntax of the
// Docker CLI (`--config ...,mode=`) wherever it does not contradict the octal
// number Compose has always read.
func TestConfigFileModeCLISyntax(t *testing.T) {
	for name, tt := range map[string]struct {
		mode string
		want types.FileMode
	}{
		"octal prefix":       {`"0o440"`, 0o440},
		"hexadecimal prefix": {`"0x1ff"`, 0o777},
		"binary prefix":      {`"0b101"`, 0b101},
		"no prefix is octal": {`"440"`, 0o440},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, `
name: test
services:
  foo:
    image: alpine
    configs:
      - source: my-config
        mode: `+tt.mode+`
configs:
  my-config:
    external: true
`)
			assert.Equal(t, *p.Services["foo"].Configs[0].Mode, tt.want)
			fromYAML, fromJSON := roundTrip(t, p)
			assert.Equal(t, *fromYAML.Services["foo"].Configs[0].Mode, tt.want)
			assert.Equal(t, *fromJSON.Services["foo"].Configs[0].Mode, tt.want)
		})
	}
}

// The mode of a config is read as the mode of a secret, an unsigned 32 bit
// number: a negative value or a value that does not fit in 32 bits is rejected,
// with the attribute in the error.
func TestConfigFileModeInvalid(t *testing.T) {
	for name, value := range map[string]string{
		"a negative number": `-1`,
		"beyond 32 bits":    `4294967296`,
		"not octal":         `"0999"`,
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, `
name: test
services:
  foo:
    image: alpine
    configs:
      - source: my-config
        mode: `+value+`
configs:
  my-config:
    external: true
`)
			assert.ErrorContains(t, err, "services[foo].configs[0].mode")
			assert.ErrorContains(t, err, "invalid file mode")
		})
	}
}
