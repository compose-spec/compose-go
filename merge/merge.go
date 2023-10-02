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

package merge

import (
	"fmt"

	"github.com/compose-spec/compose-go/tree"
	"golang.org/x/exp/slices"
)

// Merge applies subsequent overrides to a config model
func Merge(configs ...map[string]interface{}) (map[string]interface{}, error) {
	right := configs[0]
	for i := 1; i < len(configs); i++ {
		left := configs[i]
		merged, err := mergeYaml(right, left, tree.NewPath())
		if err != nil {
			return nil, err
		}
		right = merged.(map[string]interface{})
	}
	return right, nil
}

type merger func(interface{}, interface{}, tree.Path) (interface{}, error)

// mergeSpecials defines the custom rules applied by compose when merging yaml trees
var mergeSpecials = map[tree.Path]merger{}

func init() {
	mergeSpecials["services.*.logging"] = mergeLogging
	mergeSpecials["services.*.command"] = override
	mergeSpecials["services.*.entrypoint"] = override
	mergeSpecials["services.*.healthcheck.test"] = override
	mergeSpecials["services.*.environment"] = mergeEnvironment
}

// mergeYaml merges map[string]interface{} yaml trees handling special rules
func mergeYaml(e interface{}, o interface{}, p tree.Path) (interface{}, error) {
	for pattern, merger := range mergeSpecials {
		if p.Matches(pattern) {
			merged, err := merger(e, o, p)
			if err != nil {
				return nil, err
			}
			return merged, nil
		}
	}
	switch value := e.(type) {
	case map[string]interface{}:
		other, ok := o.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("cannont override %s", p)
		}
		return mergeMappings(value, other, p)
	case []interface{}:
		other, ok := o.([]interface{})
		if !ok {
			return nil, fmt.Errorf("cannont override %s", p)
		}
		return append(value, other...), nil
	default:
		return o, nil
	}
}

func mergeMappings(mapping map[string]interface{}, other map[string]interface{}, p tree.Path) (map[string]interface{}, error) {
	for k, v := range other {
		next := p.Next(k)
		e, ok := mapping[k]
		if !ok {
			mapping[k] = v
			continue
		}
		merged, err := mergeYaml(e, v, next)
		if err != nil {
			return nil, err
		}
		mapping[k] = merged
	}
	return mapping, nil
}

// logging driver options are merged only when both compose file define the same driver
func mergeLogging(c interface{}, o interface{}, p tree.Path) (interface{}, error) {
	config := c.(map[string]interface{})
	other := o.(map[string]interface{})
	// we override logging config if source and override have the same driver set, or none
	d, ok1 := other["driver"]
	o, ok2 := config["driver"]
	if d == o || !ok1 || !ok2 {
		return mergeMappings(config, other, p)
	}
	return other, nil
}

// environment must be first converted into yaml sequence syntax so we can append
func mergeEnvironment(c interface{}, o interface{}, p tree.Path) (interface{}, error) {
	right := convertIntoSequence(c)
	left := convertIntoSequence(o)
	return append(right, left...), nil
}

func convertIntoSequence(value interface{}) []interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		seq := make([]interface{}, len(v))
		i := 0
		for k, v := range v {
			if v == nil {
				seq[i] = k
			} else {
				seq[i] = fmt.Sprintf("%s=%s", k, v)
			}
			i++
		}
		slices.SortFunc(seq, func(a, b interface{}) bool {
			return a.(string) < b.(string)
		})
		return seq
	case []interface{}:
		return v
	}
	return nil
}

func override(c interface{}, other interface{}, p tree.Path) (interface{}, error) {
	return other, nil
}
