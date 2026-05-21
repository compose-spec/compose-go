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

	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/tree"
	"go.yaml.in/yaml/v4"
)

// setDefaultValuesNode walks the merged yaml.Node tree and injects the
// default values that the legacy transform.SetDefaultValues used to
// populate on the map[string]any projection. Mirrors the patterns
// registered in transform/defaults.go:
//
//   - services.*.build           → context = "."
//   - services.*.secrets.*       → target  = "/run/secrets/<source>"
//   - services.*.ports.*         → protocol = "tcp", mode = "ingress"
//   - services.*.gpus.*          → count = "all" if neither count nor device_ids
//   - services.*.deploy.resources.reservations.devices.* → same as gpus
//   - services.*.volumes.*.bind  → create_host_path = true
//
// The build.context default is left to injectMissingBuildContext, which
// runs slightly earlier and registers the new "." scalar with the main
// NodeContext so the path-resolution pass anchors it at the project
// working directory.
func setDefaultValuesNode(root *yaml.Node) {
	setDefaultValuesWalk(root, tree.NewPath())
}

func setDefaultValuesWalk(node *yaml.Node, p tree.Path) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, c := range node.Content {
			setDefaultValuesWalk(c, p)
		}
	case yaml.MappingNode:
		applyMappingDefaults(node, p)
		for i := 0; i+1 < len(node.Content); i += 2 {
			setDefaultValuesWalk(node.Content[i+1], p.Next(node.Content[i].Value))
		}
	case yaml.SequenceNode:
		for _, c := range node.Content {
			setDefaultValuesWalk(c, p.Next(tree.PathMatchList))
		}
	}
}

func applyMappingDefaults(node *yaml.Node, p tree.Path) {
	switch {
	case p.Matches("services.*.secrets.*"):
		defaultSecretMountNode(node)
	case p.Matches("services.*.ports.*"):
		defaultPortsNode(node)
	case p.Matches("services.*.gpus.*"),
		p.Matches("services.*.deploy.resources.reservations.devices.*"):
		defaultDeviceRequestNode(node)
	case p.Matches("services.*.volumes.*.bind"):
		defaultVolumeBindNode(node)
	}
}

// defaultSecretMountNode mirrors transform.defaultSecretMount.
func defaultSecretMountNode(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	if _, target := override.FindKey(node, "target"); target != nil {
		return
	}
	_, src := override.FindKey(node, "source")
	if src == nil || src.Kind != yaml.ScalarNode {
		return
	}
	override.SetKey(node, "target", override.NewScalar(fmt.Sprintf("/run/secrets/%s", src.Value)))
}

// defaultPortsNode mirrors transform.portDefaults.
func defaultPortsNode(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	if _, v := override.FindKey(node, "protocol"); v == nil {
		override.SetKey(node, "protocol", override.NewScalar("tcp"))
	}
	if _, v := override.FindKey(node, "mode"); v == nil {
		override.SetKey(node, "mode", override.NewScalar("ingress"))
	}
}

// defaultDeviceRequestNode mirrors transform.deviceRequestDefaults.
func defaultDeviceRequestNode(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	_, count := override.FindKey(node, "count")
	_, ids := override.FindKey(node, "device_ids")
	if count == nil && ids == nil {
		override.SetKey(node, "count", override.NewScalar("all"))
	}
}

// defaultVolumeBindNode mirrors transform.defaultVolumeBind.
func defaultVolumeBindNode(node *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	if _, v := override.FindKey(node, "create_host_path"); v == nil {
		override.SetKey(node, "create_host_path", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"})
	}
}
