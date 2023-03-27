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
	"fmt"
	"strconv"

	"github.com/compose-spec/compose-go/tree"
	"github.com/compose-spec/compose-go/types"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type TransformFunc func(data interface{}) (interface{}, error)

var Transformers = map[tree.Path]TransformFunc{
	"services.*.ports": trasformPorts,
}

func transform(e interface{}, p tree.Path) (interface{}, error) {
	for pattern, transformer := range Transformers {
		if p.Matches(pattern) {
			t, err := transformer(e)
			if err != nil {
				return nil, err
			}
			return t, nil
		}
	}
	switch e.(type) {
	case map[string]interface{}:
		mapping := e.(map[string]interface{})
		for k, v := range mapping {
			t, err := transform(v, p.Next(k))
			if err != nil {
				return nil, err
			}
			mapping[k] = t
		}
		return mapping, nil
	case []interface{}:
		sequence := e.([]interface{})
		for i, e := range sequence {
			t, err := transform(e, p.Next("[]"))
			if err != nil {
				return nil, err
			}
			sequence[i] = t
		}
		return sequence, nil
	default:
		return e, nil
	}
}

func trasformPorts(data interface{}) (interface{}, error) {
	switch entries := data.(type) {
	case []interface{}:
		// We process the list instead of individual items here.
		// The reason is that one entry might be mapped to multiple ServicePortConfig.
		// Therefore we take an input of a list and return an output of a list.
		var ports []interface{}
		for _, entry := range entries {
			switch value := entry.(type) {
			case int:
				parsed, err := types.ParsePortConfig(fmt.Sprint(value))
				if err != nil {
					return data, err
				}
				for _, v := range parsed {
					m := map[string]interface{}{}
					err := mapstructure.Decode(v, &m)
					if err != nil {
						return nil, err
					}
					ports = append(ports, m)
				}
			case string:
				parsed, err := types.ParsePortConfig(value)
				if err != nil {
					return data, err
				}
				for _, v := range parsed {
					m := map[string]interface{}{}
					err := mapstructure.Decode(v, &m)
					if err != nil {
						return nil, err
					}
					ports = append(ports, m)
				}
			case map[string]interface{}:
				published := value["published"]
				if v, ok := published.(int); ok {
					value["published"] = strconv.Itoa(v)
				}
				ports = append(ports, groupXFieldsIntoExtensions(value))
			default:
				return data, errors.Errorf("invalid type %T for port", value)
			}
		}
		return ports, nil
	default:
		return data, errors.Errorf("invalid type %T for port", entries)
	}
}
