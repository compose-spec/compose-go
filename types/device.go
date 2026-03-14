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
	"strings"

	"go.yaml.in/yaml/v4"
)

type DeviceRequest struct {
	Capabilities []string    `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Driver       string      `yaml:"driver,omitempty" json:"driver,omitempty"`
	Count        DeviceCount `yaml:"count,omitempty" json:"count,omitempty"`
	IDs          []string    `yaml:"device_ids,omitempty" json:"device_ids,omitempty"`
	Options      Mapping     `yaml:"options,omitempty" json:"options,omitempty"`
}

func (d *DeviceRequest) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	type plain DeviceRequest
	if err := node.Decode((*plain)(d)); err != nil {
		return WrapNodeError(node, err)
	}
	if d.Count == 0 && len(d.IDs) == 0 {
		d.Count = -1
	}
	return nil
}

type DeviceCount int64

func (c *DeviceCount) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	if node.Kind == yaml.ScalarNode && strings.ToLower(node.Value) == "all" {
		*c = -1
		return nil
	}
	var i int64
	if err := node.Decode(&i); err != nil {
		// Try parsing as string (e.g., count: "1")
		if node.Kind == yaml.ScalarNode {
			parsed, parseErr := strconv.ParseInt(node.Value, 10, 64)
			if parseErr == nil {
				*c = DeviceCount(parsed)
				return nil
			}
		}
		return NodeErrorf(node, "invalid value %q, the only value allowed is 'all' or a number", node.Value)
	}
	*c = DeviceCount(i)
	return nil
}

// GpuDevices is a slice of DeviceRequest that handles the short syntax
// `gpus: all` which expands to `[{count: -1}]`.
type GpuDevices []DeviceRequest

func (g *GpuDevices) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	if node.Kind == yaml.ScalarNode {
		// Short syntax: gpus: all
		*g = []DeviceRequest{{Count: -1}}
		return nil
	}
	if node.Kind == yaml.SequenceNode {
		var result []DeviceRequest
		for _, item := range node.Content {
			var d DeviceRequest
			if err := item.Decode(&d); err != nil {
				return WrapNodeError(item, err)
			}
			result = append(result, d)
		}
		*g = result
		return nil
	}
	return NodeErrorf(node, "gpus must be a string or sequence")
}
