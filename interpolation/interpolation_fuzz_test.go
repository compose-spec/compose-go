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

package interpolation

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
)

// FuzzInterpolate locks the interpolation of a whole model
// (https://github.com/compose-spec/compose-spec/blob/main/12-interpolation.md):
// whatever the value found at a path, in a mapping or in a list, with or
// without a type cast declared for it, Interpolate never panics and is
// deterministic. A value with nothing to interpolate (no `$`) comes back as it is.
func FuzzInterpolate(f *testing.F) {
	for _, seed := range []string{
		"", "plain", "$FOO", "${FOO}", "${FOO:-x}", "${UNSET:-${FOO}}", "${UNSET:?missing}", "$$FOO", "${",
		"1", "true", "-5", "1.5", "${NUM}", "${FOO:-1}", "yes",
	} {
		f.Add(seed)
	}
	env := map[string]string{"FOO": "foo", "NUM": "42", "EMPTY": ""}
	f.Fuzz(func(t *testing.T, value string) {
		config := map[string]interface{}{
			"scalar": value,
			"nested": map[string]interface{}{"value": value, "number": value, "list": []interface{}{value, 1, nil, true}},
			"list":   []interface{}{value, map[string]interface{}{"value": value}},
		}
		opts := Options{
			LookupValue: func(key string) (string, bool) { v, ok := env[key]; return v, ok },
			TypeCastMapping: map[tree.Path]Cast{
				"nested.number": func(s string) (interface{}, error) { return strconv.Atoi(s) },
				"list.*.value":  func(s string) (interface{}, error) { return strconv.ParseBool(s) },
			},
		}
		got, err := Interpolate(config, opts)
		again, errAgain := Interpolate(config, opts)
		if !reflect.DeepEqual(got, again) || (err == nil) != (errAgain == nil) {
			t.Fatalf("Interpolate(%q) is not deterministic: (%v, %v) then (%v, %v)", value, got, err, again, errAgain)
		}

		if !strings.Contains(value, "$") && got["scalar"] != value && err == nil {
			t.Fatalf("Interpolate(%q) changed a value with nothing to interpolate into %q", value, got["scalar"])
		}
	})
}
