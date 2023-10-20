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

package loader

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/mitchellh/mapstructure"
	"gotest.tools/v3/assert"
)

func TestDecodeMapStructure(t *testing.T) {
	var target types.ServiceConfig
	data := mapstructure.Metadata{}
	config := &mapstructure.DecoderConfig{
		Result:     &target,
		TagName:    "yaml",
		Metadata:   &data,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(decoderHook),
	}
	decoder, err := mapstructure.NewDecoder(config)
	assert.NilError(t, err)
	err = decoder.Decode(map[string]interface{}{
		"mem_limit":         "640k",
		"command":           "echo hello",
		"stop_grace_period": "60s",
		"labels": []interface{}{
			"FOO=BAR",
		},
		"deploy": map[string]interface{}{
			"labels": map[string]interface{}{
				"FOO": "BAR",
				"BAZ": nil,
				"QIX": 2,
				"ZOT": true,
			},
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, target.MemLimit, types.UnitBytes(640*1024))
	assert.DeepEqual(t, target.Command, types.ShellCommand{"echo", "hello"})
	assert.Equal(t, *target.StopGracePeriod, types.Duration(60_000_000_000))
	assert.DeepEqual(t, target.Labels, types.Labels{"FOO": "BAR"})
	assert.DeepEqual(t, target.Deploy.Labels, types.Labels{
		"FOO": "BAR",
		"BAZ": "",
		"QIX": "2",
		"ZOT": "true",
	})
}
