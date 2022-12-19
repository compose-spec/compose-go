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

	yaml3 "gopkg.in/yaml.v3"
)

func mappingNode(m map[string]interface{}) *yaml3.Node {
	nodes := make([]*yaml3.Node, 0, len(m))
	for k, v := range m {
		var (
			value string
			tag   string
		)
		switch v.(type) {
		case string:
			value = v.(string)
			tag = "!!str"
		case bool:
			value = fmt.Sprint(v)
			tag = "!!bool"
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			value = fmt.Sprint(v)
			tag = "!!int"
		}
		nodes = append(nodes,
			&yaml3.Node{
				Kind:  yaml3.ScalarNode,
				Value: k,
				Tag:   "!!str",
			},
			&yaml3.Node{
				Kind:  yaml3.ScalarNode,
				Value: value,
				Tag:   tag,
			},
		)
	}
	return &yaml3.Node{
		Kind:    yaml3.MappingNode,
		Content: nodes,
	}
}
