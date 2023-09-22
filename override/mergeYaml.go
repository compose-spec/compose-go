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

package override

import (
	"fmt"

	"github.com/compose-spec/compose-go/tree"
)

type merger func(interface{}, interface{}, tree.Path) (interface{}, error)

// mergeSpecials defines the custom rules applied by compose when merging yaml trees
var mergeSpecials = map[tree.Path]merger{}

func init() {
	mergeSpecials["services.*.logging"] = mergeLogging
	mergeSpecials["services.*.volumes"] = mergeVolumes
	mergeSpecials["services.*.ports"] = mergePorts
}

// MergeYaml merges map[string]interface{} yaml trees handling special rules
func MergeYaml(e interface{}, o interface{}, p tree.Path) (interface{}, error) {
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

func mergeSlices(c []interface{}, o []interface{}, keyFn func(interface{}) interface{}, path tree.Path) (interface{}, error) {
	merged := map[interface{}]interface{}{}
	for _, v := range c {
		merged[keyFn(v)] = v
	}
	for _, v := range o {
		key := keyFn(v)
		e, ok := merged[key]
		if !ok {
			merged[key] = v
			continue
		}
		MergeYaml(e, v, path.Next("[]"))
	}
	sequence := make([]interface{}, 0, len(merged))
	for _, v := range merged {
		sequence = append(sequence, v)
	}
	return sequence, nil
}

func mergeMappings(mapping map[string]interface{}, other map[string]interface{}, p tree.Path) (map[string]interface{}, error) {
	for k, v := range other {
		next := p.Next(k)
		e, ok := mapping[k]
		if !ok {
			mapping[k] = v
			continue
		}
		merged, err := MergeYaml(e, v, next)
		if err != nil {
			return nil, err
		}
		mapping[k] = merged
	}
	return mapping, nil
}

// ports is actually a map, indexed by ip:port:target/protocol
func mergePorts(v interface{}, o interface{}, path tree.Path) (interface{}, error) {
	type port struct {
		target    interface{}
		published interface{}
		ip        interface{}
		protocol  interface{}
	}
	return mergeSlices(v.([]interface{}), o.([]interface{}), func(i interface{}) interface{} {
		m := i.(map[string]interface{})
		return port{
			target:    m["target"],
			published: m["published"],
			ip:        m["ip"],
			protocol:  m["protocol"],
		}
	}, path)
}

// volumes is actually a map, indexed by mount path in container
func mergeVolumes(v interface{}, o interface{}, path tree.Path) (interface{}, error) {
	return mergeSlices(v.([]interface{}), o.([]interface{}), func(i interface{}) interface{} {
		m := i.(map[string]interface{})
		return m["target"]
	}, path)
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
