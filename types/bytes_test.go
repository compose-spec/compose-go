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

package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestUnitBytesUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected UnitBytes
	}{
		{"plain integer", `655360`, UnitBytes(655360)},
		{"string integer", `"655360"`, UnitBytes(655360)},
		{"negative integer", `-1`, UnitBytes(-1)},
		{"string negative", `"-1"`, UnitBytes(-1)},
		{"human readable 1g", `"1g"`, UnitBytes(1073741824)},
		{"human readable 512m", `"512m"`, UnitBytes(536870912)},
		{"zero", `0`, UnitBytes(0)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result UnitBytes
			err := json.Unmarshal([]byte(tt.input), &result)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnitBytesUnmarshalJSON_Invalid(t *testing.T) {
	var result UnitBytes
	err := json.Unmarshal([]byte(`"invalid"`), &result)
	assert.Error(t, err)
}

func TestUnitBytesUnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected UnitBytes
	}{
		{"plain integer", `655360`, UnitBytes(655360)},
		{"quoted string integer", `"655360"`, UnitBytes(655360)},
		{"negative integer", `-1`, UnitBytes(-1)},
		{"human readable 1g", `"1g"`, UnitBytes(1073741824)},
		{"human readable 512m", `"512m"`, UnitBytes(536870912)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result UnitBytes
			err := yaml.Unmarshal([]byte(tt.input), &result)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnitBytesUnmarshalYAML_Invalid(t *testing.T) {
	var result UnitBytes
	err := yaml.Unmarshal([]byte(`"invalid"`), &result)
	assert.Error(t, err)
}

func TestUnitBytesJSONRoundTrip(t *testing.T) {
	original := UnitBytes(655360)
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var result UnitBytes
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestUnitBytesYAMLRoundTrip(t *testing.T) {
	original := UnitBytes(655360)
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var result UnitBytes
	err = yaml.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestUnitBytesJSONRoundTripViaUntypedMap(t *testing.T) {
	type wrapper struct {
		Size UnitBytes `json:"size"`
	}
	original := wrapper{Size: UnitBytes(655360)}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var untyped map[string]interface{}
	err = json.Unmarshal(data, &untyped)
	require.NoError(t, err)

	data2, err := json.Marshal(untyped)
	require.NoError(t, err)

	var result wrapper
	err = json.Unmarshal(data2, &result)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}
