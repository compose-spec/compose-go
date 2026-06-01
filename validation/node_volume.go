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

package validation

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/consts"
	"github.com/compose-spec/compose-go/v3/tree"
)

func checkVolumeNode(n *yaml.Node, p tree.Path) error {
	if n == nil {
		return nil
	}
	if n.Kind != yaml.MappingNode {
		// A `!!null` scalar (empty volume entry) is valid.
		if n.Kind == yaml.ScalarNode && n.Tag == "!!null" {
			return nil
		}
		return fmt.Errorf("expected volume, got %s", n.Value)
	}
	return checkExternalNode(n, p)
}

func checkExternalNode(n *yaml.Node, p tree.Path) error {
	external := mappingFieldNode(n, "external")
	if external == nil {
		return nil
	}
	if external.Kind != yaml.ScalarNode || external.Value != "true" {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		switch k {
		case "name", "external", consts.Extensions:
			continue
		default:
			if strings.HasPrefix(k, "x-") {
				continue
			}
			return fmt.Errorf("%s: conflicting parameters \"external\" and %q specified", p, k)
		}
	}
	return nil
}
