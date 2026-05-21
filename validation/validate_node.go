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
	"net"
	"strings"

	"github.com/compose-spec/compose-go/v3/consts"
	"github.com/compose-spec/compose-go/v3/tree"
	"go.yaml.in/yaml/v4"
)

// ValidateSemantics applies the Compose semantic checks on a merged
// yaml.Node tree. It mirrors Validate (which works on map[string]any)
// but operates directly on yaml.Node so the v3 loader can avoid the
// map round-trip.
func ValidateSemantics(root *yaml.Node) error {
	if root == nil {
		return nil
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}
	return checkNode(root, tree.NewPath())
}

type nodeCheckerFunc func(node *yaml.Node, p tree.Path) error

var nodeChecks = map[tree.Path]nodeCheckerFunc{
	"volumes.*":                       checkVolumeNode,
	"configs.*":                       checkFileObjectNode("file", "environment", "content"),
	"secrets.*":                       checkFileObjectNode("file", "environment"),
	"services.*.ports.*":              checkIPAddressNode,
	"services.*.develop.watch.*.path": checkPathNode,
	"services.*.deploy.resources.reservations.devices.*": checkDeviceRequestNode,
	"services.*.gpus.*": checkDeviceRequestNode,
}

func checkNode(node *yaml.Node, p tree.Path) error {
	for pattern, fn := range nodeChecks {
		if p.Matches(pattern) {
			return fn(node, p)
		}
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := checkNode(node.Content[i+1], p.Next(node.Content[i].Value)); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, c := range node.Content {
			if err := checkNode(c, p.Next(tree.PathMatchList)); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkVolumeNode(node *yaml.Node, p tree.Path) error {
	if node == nil || node.Kind == yaml.ScalarNode && node.Tag == "!!null" {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expected volume, got %s", nodeKindName(node.Kind))
	}
	return checkExternalNode(node, p)
}

func checkFileObjectNode(keys ...string) nodeCheckerFunc {
	return func(node *yaml.Node, p tree.Path) error {
		if node == nil || node.Kind != yaml.MappingNode {
			return nil
		}
		count := 0
		for _, k := range keys {
			if _, v := mapValueNode(node, k); v != nil {
				count++
			}
		}
		if count > 1 {
			return fmt.Errorf("%s: %s attributes are mutually exclusive", p, strings.Join(keys, "|"))
		}
		if count == 0 {
			if _, v := mapValueNode(node, "driver"); v != nil {
				return nil
			}
			if _, v := mapValueNode(node, "external"); v == nil {
				return fmt.Errorf("%s: one of %s must be set", p, strings.Join(keys, "|"))
			}
		}
		return nil
	}
}

func checkPathNode(node *yaml.Node, p tree.Path) error {
	if node == nil || node.Kind != yaml.ScalarNode || node.Value == "" {
		return fmt.Errorf("%s: value can't be blank", p)
	}
	return nil
}

func checkDeviceRequestNode(node *yaml.Node, p tree.Path) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	_, hasCount := mapValueNode(node, "count")
	_, hasIDs := mapValueNode(node, "device_ids")
	if hasCount != nil && hasIDs != nil {
		return fmt.Errorf(`%s: "count" and "device_ids" attributes are exclusive`, p)
	}
	return nil
}

func checkIPAddressNode(node *yaml.Node, p tree.Path) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	_, ip := mapValueNode(node, "host_ip")
	if ip == nil {
		return nil
	}
	if ip.Kind == yaml.ScalarNode && net.ParseIP(ip.Value) == nil {
		return fmt.Errorf("%s: invalid ip address: %s", p, ip.Value)
	}
	return nil
}

func checkExternalNode(node *yaml.Node, p tree.Path) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	_, ext := mapValueNode(node, "external")
	if ext == nil {
		return nil
	}
	// external must be true (yaml bool) for the check to trigger
	if ext.Kind != yaml.ScalarNode || ext.Tag != "!!bool" || !strings.EqualFold(ext.Value, "true") {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
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

// mapValueNode returns (key node, value node) of the given key in a
// MappingNode, or (nil, nil) if the key is missing.
//
//nolint:unparam // key node intentionally returned for parity with override.FindKey
func mapValueNode(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], m.Content[i+1]
		}
	}
	return nil, nil
}

func nodeKindName(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	}
	return fmt.Sprintf("kind(%d)", k)
}
