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

package transform

import (
	"github.com/compose-spec/compose-go/tree"
)

type transformFunc func(data interface{}, p tree.Path) (interface{}, error)

var transformers = map[tree.Path]transformFunc{}

func init() {
	transformers["services"] = makeServicesSlice
	transformers["services.*.networks"] = transformServiceNetworks
	transformers["services.*.ports"] = trasformPorts
}

// Canonical transforms a compose model into canonical syntax
func Canonical(yaml map[string]interface{}) (map[string]interface{}, error) {
	canonical, err := transform(yaml, tree.NewPath())
	if err != nil {
		return nil, err
	}
	return canonical.(map[string]interface{}), nil
}

func transform(data interface{}, p tree.Path) (interface{}, error) {
	for pattern, transformer := range transformers {
		if p.Matches(pattern) {
			t, err := transformer(data, p)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
	}
	switch data.(type) {
	case map[string]interface{}:
		mapping := data.(map[string]interface{})
		for k, v := range mapping {
			t, err := transform(v, p.Next(k))
			if err != nil {
				return nil, err
			}
			mapping[k] = t
		}
		return mapping, nil
	case []interface{}:
		sequence := data.([]interface{})
		for i, e := range sequence {
			t, err := transform(e, p.Next("[]"))
			if err != nil {
				return nil, err
			}
			sequence[i] = t
		}
		return sequence, nil
	default:
		return data, nil
	}
}
