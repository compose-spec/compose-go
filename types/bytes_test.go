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
		{"int32", int32(1024), 1024},
		{"uint8", uint8(255), 255},
		{"float32", float32(1024), 1024},
		{"UnitBytes", UnitBytes(1024), 1024},
		{"nil", nil, 0},
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

// A byte value written as a string is {amount}{unit}, as documented by
// https://github.com/compose-spec/compose-spec/blob/main/11-extension.md#specifying-byte-values
// Amounts that cannot be a size, or that do not fit in 64 bits, are rejected
// instead of being silently truncated, clamped or turned into -1.
func TestUnitBytesParseString(t *testing.T) {
	accepted := []struct {
		input    string
		expected UnitBytes
	}{
		{"0", 0},
		{"-1", -1},
		{"2b", 2},
		{"2B", 2},
		{"2k", 2048},
		{"2kb", 2048},
		{"2KiB", 2048},
		{"2 g", 2 << 30},
		{"1t", 1 << 40},
		{"1p", 1 << 50},
		{"1.5g", 1610612736},
		{"1.1g", 1181116006},
		{".5m", 524288},
		{"1.", 1},
		{"1e3", 1000},
		{"9223372036854775807", math.MaxInt64},
		{"7p", 7 << 50},
	}
	for _, tt := range accepted {
		t.Run("accepts "+tt.input, func(t *testing.T) {
			var result UnitBytes
			assert.NilError(t, result.parseString(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}

	rejected := []struct{ input, message string }{
		{"", "invalid size: ''"},
		{"abc", "invalid size: 'abc'"},
		{"-1g", "invalid size: '-1g'"},
		{"+1g", "invalid size: '+1g'"},
		{"0x10", "invalid size: '0x10'"},
		{"0x1p3", "invalid size: '0x1p3'"},
		{"1_000", "invalid size: '1_000'"},
		{"inf", "invalid size: 'inf'"},
		{"NaN", "invalid size: 'NaN'"},
		{" 1g", "invalid size: ' 1g'"},
		{"1x", "invalid suffix: 'x'"},
		{"1ib", "invalid suffix: 'ib'"},
		{"1bb", "invalid suffix: 'bb'"},
		{"1kx", "invalid suffix: 'kx'"},
		{"0.5", "a number of bytes must be an integer"},
		{"1.5b", "a number of bytes must be an integer"},
		{"1e-1", "a number of bytes must be an integer"},
		{"9223372036854775808", "out of range"},
		{"9999999999g", "out of range"},
		{"8192p", "out of range"},
		{"8e", "invalid suffix: 'e'"},
		{"1e999", "invalid size: '1e999'"},
	}
	for _, tt := range rejected {
		t.Run("rejects "+tt.input, func(t *testing.T) {
			var result UnitBytes
			assert.ErrorContains(t, result.parseString(tt.input), tt.message)
		})
	}
}

// A JSON number that is not an integer literal is a byte value when its value
// is a whole number of bytes (as 1e3), and an error otherwise.
func TestUnitBytesUnmarshalJSONNumbers(t *testing.T) {
	var result UnitBytes
	assert.NilError(t, json.Unmarshal([]byte(`1e3`), &result))
	assert.Equal(t, UnitBytes(1000), result)

	assert.ErrorContains(t, json.Unmarshal([]byte(`1.5`), &result), "a number of bytes must be an integer")
	assert.ErrorContains(t, json.Unmarshal([]byte(`1e19`), &result), "out of range")
	assert.ErrorContains(t, json.Unmarshal([]byte(`true`), &result), "cannot unmarshal")
}

// A YAML number that is not an integer is a byte value when its value is a
// whole number of bytes (as 1e3), and an error otherwise.
func TestUnitBytesUnmarshalYAMLNumbers(t *testing.T) {
	var result UnitBytes
	assert.NilError(t, yaml.Unmarshal([]byte(`1e3`), &result))
	assert.Equal(t, UnitBytes(1000), result)

	assert.ErrorContains(t, yaml.Unmarshal([]byte(`1.5`), &result), "a number of bytes must be an integer")
	assert.ErrorContains(t, yaml.Unmarshal([]byte(`9223372036854775808`), &result), "out of range")
}
