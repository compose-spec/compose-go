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
	"github.com/go-viper/mapstructure/v2"
	"gotest.tools/v3/assert"
)

func TestDecodeMapStructure(t *testing.T) {
	var target types.ServiceConfig
	data := mapstructure.Metadata{}
	config := &mapstructure.DecoderConfig{
		Result:     &target,
		TagName:    "yaml",
		Squash:     true,
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

// Transform binds a model to the project types. Services and jobs that are not
// mappings are reported by the decoder as an error naming them instead of
// crashing while their implicit `name` is set. Transform is exported, and is
// also reached with SkipValidation.
func TestTransformRejectsServicesThatAreNotMappings(t *testing.T) {
	tests := []struct {
		name    string
		model   map[string]any
		wantErr string
	}{
		{"services is a list", map[string]any{"services": []any{"a"}}, "'services' expected type"},
		{"services is a string", map[string]any{"services": "a"}, "'services' expected type"},
		{"service is a number", map[string]any{"services": map[string]any{"a": 1}}, "'services[a]' expected a map or struct"},
		{"service is a list", map[string]any{"services": map[string]any{"a": []any{"x"}}}, "'services[a]' expected a map or struct"},
		{"job is a string", map[string]any{"jobs": map[string]any{"a": "x"}}, "'jobs[a]' expected a map or struct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Transform(tt.model, &types.Project{})
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// A service or job left empty (`db:` with nothing below) is an empty
// definition that still carries the name of its key, as the consumers matching
// dependencies against names expect.
func TestTransformNamesEmptyServicesAndJobs(t *testing.T) {
	project := &types.Project{}
	err := Transform(map[string]any{
		"services": map[string]any{"db": nil},
		"jobs":     map[string]any{"backup": nil},
	}, project)
	assert.NilError(t, err)
	assert.Equal(t, project.Services["db"].Name, "db")
	assert.Equal(t, project.Jobs["backup"].Name, "backup")
}
