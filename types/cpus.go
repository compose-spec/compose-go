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
	"strconv"

	"go.yaml.in/yaml/v4"
)

type NanoCPUs float32

func (n *NanoCPUs) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	var f float64
	if err := node.Decode(&f); err != nil {
		if node.Kind == yaml.ScalarNode {
			parsed, parseErr := strconv.ParseFloat(node.Value, 64)
			if parseErr == nil {
				*n = NanoCPUs(parsed)
				return nil
			}
		}
		return WrapNodeError(node, err)
	}
	*n = NanoCPUs(f)
	return nil
}

func (n *NanoCPUs) Value() float32 {
	return float32(*n)
}
