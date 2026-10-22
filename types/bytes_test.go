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
	"math"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
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
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnitBytesUnmarshalJSON_Invalid(t *testing.T) {
	var result UnitBytes
	err := json.Unmarshal([]byte(`"invalid"`), &result)
	assert.ErrorContains(t, err, "invalid size: 'invalid'")
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
			assert.NilError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnitBytesUnmarshalYAML_Invalid(t *testing.T) {
	var result UnitBytes
	err := yaml.Unmarshal([]byte(`"invalid"`), &result)
	assert.ErrorContains(t, err, "invalid size: 'invalid'")
}

func TestUnitBytesJSONRoundTrip(t *testing.T) {
	original := UnitBytes(655360)
	data, err := json.Marshal(original)
	assert.NilError(t, err)

	var result UnitBytes
	err = json.Unmarshal(data, &result)
	assert.NilError(t, err)
	assert.Equal(t, original, result)
}

func TestUnitBytesYAMLRoundTrip(t *testing.T) {
	original := UnitBytes(655360)
	data, err := yaml.Marshal(original)
	assert.NilError(t, err)

	var result UnitBytes
	err = yaml.Unmarshal(data, &result)
	assert.NilError(t, err)
	assert.Equal(t, original, result)
}

func TestUnitBytesJSONRoundTripViaUntypedMap(t *testing.T) {
	type wrapper struct {
		Size UnitBytes `json:"size"`
	}
	original := wrapper{Size: UnitBytes(655360)}

	data, err := json.Marshal(original)
	assert.NilError(t, err)

	var untyped map[string]interface{}
	err = json.Unmarshal(data, &untyped)
	assert.NilError(t, err)

	data2, err := json.Marshal(untyped)
	assert.NilError(t, err)

	var result wrapper
	err = json.Unmarshal(data2, &result)
	assert.NilError(t, err)
	assert.Equal(t, original, result)
}

// A byte value decoded from the YAML model is a whole number of bytes: a
// number with an integer value is accepted whatever its notation, and any
// other number is an error instead of a silent 0.
func TestUnitBytesDecodeMapstructureNumbers(t *testing.T) {
	accepted := []struct {
		name     string
		input    any
		expected UnitBytes
	}{
		{"int", 1024, 1024},
		{"int64", int64(1024), 1024},
		{"uint64", uint64(1024), 1024},
		{"negative int", -1, -1},
		{"float with an integer value", 1024.0, 1024},
		{"float with an exponent", 1e3, 1000},
		{"negative float with an integer value", -1.0, -1},
		{"largest uint64 that fits", uint64(math.MaxInt64), math.MaxInt64},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			var result UnitBytes
			assert.NilError(t, result.DecodeMapstructure(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}

	rejected := []struct {
		name    string
		input   any
		message string
	}{
		{"fraction", 1.5, "invalid size: '1.5': a number of bytes must be an integer"},
		{"fraction below one", 0.5, "invalid size: '0.5': a number of bytes must be an integer"},
		{"not a number", math.NaN(), "invalid size: 'NaN'"},
		{"float beyond 64 bits", 1e19, "invalid size: '1e+19': out of range"},
		{"infinity", math.Inf(1), "invalid size: '+Inf': out of range"},
		{"uint64 beyond 64 bits", uint64(math.MaxInt64) + 1, "invalid size: '9223372036854775808': out of range"},
		{"boolean", true, "unexpected value type bool for a byte value"},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			var result UnitBytes
			assert.ErrorContains(t, result.DecodeMapstructure(tt.input), tt.message)
		})
	}
}
