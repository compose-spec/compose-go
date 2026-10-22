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
	"fmt"
	"math"
	"strconv"

	"github.com/docker/go-units"
	"go.yaml.in/yaml/v4"
)

// UnitBytes is the bytes type
type UnitBytes int64

// MarshalYAML makes UnitBytes implement yaml.Marshaller
func (u UnitBytes) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%d", u), nil
}

// MarshalJSON makes UnitBytes implement json.Marshaler
func (u UnitBytes) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%d"`, u)), nil
}

// parseString parses a string into a UnitBytes value, supporting plain
// integers, negative values (e.g., "-1"), and human-readable byte units
// (e.g., "1g", "512m").
func (u *UnitBytes) parseString(s string) error {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		*u = UnitBytes(n)
		return nil
	}
	b, err := units.RAMInBytes(s)
	*u = UnitBytes(b)
	return err
}

// UnmarshalJSON makes UnitBytes implement json.Unmarshaler
func (u *UnitBytes) UnmarshalJSON(data []byte) error {
	var v int64
	if err := json.Unmarshal(data, &v); err == nil {
		*u = UnitBytes(v)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return u.parseString(s)
}

// UnmarshalYAML makes UnitBytes implement yaml.Unmarshaler
func (u *UnitBytes) UnmarshalYAML(value *yaml.Node) error {
	var v int64
	if err := value.Decode(&v); err == nil {
		*u = UnitBytes(v)
		return nil
	}
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	return u.parseString(s)
}

// DecodeMapstructure decodes a byte value from the YAML model: a number of
// bytes, or a string with an optional unit. A number that is not a whole
// number of bytes, or does not fit, is an error rather than a silent 0.
func (u *UnitBytes) DecodeMapstructure(value interface{}) error {
	switch v := value.(type) {
	case nil:
		return nil
	case int:
		*u = UnitBytes(v)
	case int64:
		*u = UnitBytes(v)
	case uint64:
		if v > math.MaxInt64 {
			return fmt.Errorf("invalid size: '%d': out of range, must fit in a signed 64 bits integer", v)
		}
		*u = UnitBytes(v)
	case float64:
		// the schema types these attributes as a number, and a number such as
		// 1e3 or 1024.0 has an integer value
		if v != math.Trunc(v) {
			return fmt.Errorf("invalid size: '%s': a number of bytes must be an integer", strconv.FormatFloat(v, 'g', -1, 64))
		}
		if v >= float64(1<<63) || v < -float64(1<<63) {
			return fmt.Errorf("invalid size: '%s': out of range, must fit in a signed 64 bits integer", strconv.FormatFloat(v, 'g', -1, 64))
		}
		*u = UnitBytes(v)
	case string:
		return u.parseString(v)
	default:
		return fmt.Errorf("unexpected value type %T for a byte value", value)
	}
	return nil
}
