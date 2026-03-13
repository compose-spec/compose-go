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
	"slices"

	"github.com/compose-spec/compose-go/v2/tree"
	"go.yaml.in/yaml/v4"
)

// KeyValue is a key-value pair for building yaml.Node mappings.
type KeyValue struct {
	Key   string
	Value *yaml.Node
}

// FindKey finds a key in a MappingNode, returns (key node, value node) or (nil, nil).
func FindKey(node *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i], node.Content[i+1]
		}
	}
	return nil, nil
}

// SetKey adds or replaces a key in a MappingNode.
func SetKey(node *yaml.Node, key string, value *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, NewScalar(key), value)
}

// DeleteKey removes a key from a MappingNode.
func DeleteKey(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	content := make([]*yaml.Node, 0, len(node.Content))
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value != key {
			content = append(content, node.Content[i], node.Content[i+1])
		}
	}
	node.Content = content
}

// NewScalar creates a new ScalarNode with the given value.
func NewScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"}
}

// NewMapping creates a new MappingNode from key-value pairs.
func NewMapping(pairs ...KeyValue) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, p := range pairs {
		n.Content = append(n.Content, NewScalar(p.Key), p.Value)
	}
	return n
}

// NewSequence creates a new SequenceNode from items.
func NewSequence(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items}
}

type nodeMerger func(*yaml.Node, *yaml.Node, tree.Path) (*yaml.Node, error)

var mergeNodeSpecials map[tree.Path]nodeMerger

func init() {
	mergeNodeSpecials = map[tree.Path]nodeMerger{
		"networks.*.ipam.config":                mergeIPAMConfigNode,
		"networks.*.labels":                     mergeToSequenceNode,
		"volumes.*.labels":                      mergeToSequenceNode,
		"services.*.annotations":                mergeToSequenceNode,
		"services.*.build":                      mergeBuildNode,
		"services.*.build.args":                 mergeToSequenceNode,
		"services.*.build.additional_contexts":   mergeToSequenceNode,
		"services.*.build.extra_hosts":           mergeExtraHostsNode,
		"services.*.build.labels":               mergeToSequenceNode,
		"services.*.command":                    overrideNode,
		"services.*.depends_on":                 mergeDependsOnNode,
		"services.*.deploy.labels":              mergeToSequenceNode,
		"services.*.dns":                        mergeToSequenceNode,
		"services.*.dns_opt":                    mergeToSequenceNode,
		"services.*.dns_search":                 mergeToSequenceNode,
		"services.*.entrypoint":                 overrideNode,
		"services.*.env_file":                   mergeToSequenceNode,
		"services.*.label_file":                 mergeToSequenceNode,
		"services.*.environment":                mergeToSequenceNode,
		"services.*.extra_hosts":                mergeExtraHostsNode,
		"services.*.healthcheck.test":           overrideNode,
		"services.*.labels":                     mergeToSequenceNode,
		"services.*.volumes.*.volume.labels":    mergeToSequenceNode,
		"services.*.logging":                    mergeLoggingNode,
		"services.*.models":                     mergeModelsNode,
		"services.*.networks":                   mergeNetworksNode,
		"services.*.sysctls":                    mergeToSequenceNode,
		"services.*.tmpfs":                      mergeToSequenceNode,
		"services.*.ulimits.*":                  mergeUlimitNode,
	}
}

// MergeNodes merges two yaml.Node trees following the same rules as MergeYaml.
func MergeNodes(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	for pattern, merger := range mergeNodeSpecials {
		if path.Matches(pattern) {
			return merger(base, override, path)
		}
	}
	if override == nil {
		return base, nil
	}
	if base == nil {
		return override, nil
	}
	switch {
	case base.Kind == yaml.MappingNode && override.Kind == yaml.MappingNode:
		return mergeNodeMappings(base, override, path)
	case base.Kind == yaml.SequenceNode && override.Kind == yaml.SequenceNode:
		result := &yaml.Node{
			Kind: yaml.SequenceNode,
			Tag:  base.Tag,
		}
		result.Content = append(result.Content, base.Content...)
		result.Content = append(result.Content, override.Content...)
		return result, nil
	case base.Kind == yaml.MappingNode || override.Kind == yaml.MappingNode:
		return nil, fmt.Errorf("cannot override %s", path)
	default:
		// scalar: override wins
		return override, nil
	}
}

func mergeNodeMappings(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	result := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  base.Tag,
	}
	// Copy all base pairs
	result.Content = append(result.Content, base.Content...)

	// Merge override pairs
	for i := 0; i+1 < len(override.Content); i += 2 {
		key := override.Content[i].Value
		val := override.Content[i+1]
		_, existing := FindKey(result, key)
		if existing == nil {
			result.Content = append(result.Content, override.Content[i], val)
			continue
		}
		next := path.Next(key)
		merged, err := MergeNodes(existing, val, next)
		if err != nil {
			return nil, err
		}
		SetKey(result, key, merged)
	}
	return result, nil
}

func overrideNode(_, override *yaml.Node, _ tree.Path) (*yaml.Node, error) {
	return override, nil
}

// convertNodeToSequence converts a MappingNode into a SequenceNode of "key=value" scalars,
// mirroring convertIntoSequence from merge.go.
func convertNodeToSequence(node *yaml.Node) *yaml.Node {
	if node == nil {
		return NewSequence()
	}
	switch node.Kind {
	case yaml.MappingNode:
		var entries []string
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i].Value
			v := node.Content[i+1]
			if v.Tag == "!!null" || (v.Kind == yaml.ScalarNode && v.Value == "") && v.Tag == "" {
				entries = append(entries, k)
			} else if v.Kind == yaml.SequenceNode {
				for _, item := range v.Content {
					entries = append(entries, fmt.Sprintf("%s=%s", k, item.Value))
				}
			} else {
				entries = append(entries, fmt.Sprintf("%s=%s", k, v.Value))
			}
		}
		slices.Sort(entries)
		items := make([]*yaml.Node, len(entries))
		for i, e := range entries {
			items[i] = NewScalar(e)
		}
		return NewSequence(items...)
	case yaml.SequenceNode:
		return node
	case yaml.ScalarNode:
		return NewSequence(node)
	}
	return NewSequence()
}

func mergeToSequenceNode(base, override *yaml.Node, _ tree.Path) (*yaml.Node, error) {
	right := convertNodeToSequence(base)
	left := convertNodeToSequence(override)
	result := NewSequence()
	result.Content = append(result.Content, right.Content...)
	result.Content = append(result.Content, left.Content...)
	return result, nil
}

func mergeExtraHostsNode(base, override *yaml.Node, _ tree.Path) (*yaml.Node, error) {
	right := convertNodeToSequence(base)
	left := convertNodeToSequence(override)

	// Deduplicate: remove from left any entries already in right
	rightValues := make(map[string]bool, len(right.Content))
	for _, n := range right.Content {
		rightValues[n.Value] = true
	}
	var deduped []*yaml.Node
	for _, n := range left.Content {
		if !rightValues[n.Value] {
			deduped = append(deduped, n)
		}
	}
	result := NewSequence()
	result.Content = append(result.Content, right.Content...)
	result.Content = append(result.Content, deduped...)
	return result, nil
}

func mergeBuildNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	toBuildMapping := func(n *yaml.Node) *yaml.Node {
		if n == nil {
			return NewMapping()
		}
		if n.Kind == yaml.ScalarNode {
			return NewMapping(KeyValue{Key: "context", Value: n})
		}
		return n
	}
	return mergeNodeMappings(toBuildMapping(base), toBuildMapping(override), path)
}

func mergeDependsOnNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	defaultVal := func() *yaml.Node {
		return NewMapping(
			KeyValue{Key: "condition", Value: NewScalar("service_started")},
			KeyValue{Key: "required", Value: &yaml.Node{Kind: yaml.ScalarNode, Value: "true", Tag: "!!bool"}},
		)
	}
	right := convertNodeToMapping(base, defaultVal)
	left := convertNodeToMapping(override, defaultVal)
	return mergeNodeMappings(right, left, path)
}

func mergeNetworksNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	right := convertNodeToMapping(base, nil)
	left := convertNodeToMapping(override, nil)
	return mergeNodeMappings(right, left, path)
}

func mergeModelsNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	right := convertNodeToMapping(base, nil)
	left := convertNodeToMapping(override, nil)
	return mergeNodeMappings(right, left, path)
}

// convertNodeToMapping converts a SequenceNode to a MappingNode.
// If defaultValue is non-nil, each key gets a copy of the default; otherwise keys map to null.
func convertNodeToMapping(node *yaml.Node, defaultValue func() *yaml.Node) *yaml.Node {
	if node == nil {
		return NewMapping()
	}
	switch node.Kind {
	case yaml.MappingNode:
		return node
	case yaml.SequenceNode:
		result := NewMapping()
		for _, item := range node.Content {
			var val *yaml.Node
			if defaultValue != nil {
				val = defaultValue()
			} else {
				val = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"}
			}
			result.Content = append(result.Content, NewScalar(item.Value), val)
		}
		return result
	}
	return NewMapping()
}

func mergeLoggingNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	if base == nil || base.Kind != yaml.MappingNode {
		return override, nil
	}
	if override == nil || override.Kind != yaml.MappingNode {
		return base, nil
	}
	_, baseDriver := FindKey(base, "driver")
	_, overDriver := FindKey(override, "driver")

	bothSet := baseDriver != nil && overDriver != nil
	sameDriver := bothSet && baseDriver.Value == overDriver.Value

	if !bothSet || sameDriver {
		return mergeNodeMappings(base, override, path)
	}
	return override, nil
}

func mergeUlimitNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	if base != nil && base.Kind == yaml.MappingNode && override != nil && override.Kind == yaml.MappingNode {
		return mergeNodeMappings(base, override, path)
	}
	return override, nil
}

func mergeIPAMConfigNode(base, override *yaml.Node, path tree.Path) (*yaml.Node, error) {
	if base == nil || base.Kind != yaml.SequenceNode {
		return override, fmt.Errorf("%s: unexpected node kind", path)
	}
	if override == nil || override.Kind != yaml.SequenceNode {
		return override, fmt.Errorf("%s: unexpected node kind", path)
	}

	var ipamConfigs []*yaml.Node

	for _, original := range base.Content {
		right := convertNodeToMapping(original, nil)
		for _, over := range override.Content {
			left := convertNodeToMapping(over, nil)
			_, rightSubnet := FindKey(right, "subnet")
			_, leftSubnet := FindKey(left, "subnet")

			rightVal := ""
			if rightSubnet != nil {
				rightVal = rightSubnet.Value
			}
			leftVal := ""
			if leftSubnet != nil {
				leftVal = leftSubnet.Value
			}

			if leftVal != rightVal {
				// Add left if not already present
				if !slices.ContainsFunc(ipamConfigs, func(n *yaml.Node) bool {
					_, s := FindKey(n, "subnet")
					return s != nil && s.Value == leftVal
				}) {
					ipamConfigs = append(ipamConfigs, left)
				}
				continue
			}
			merged, err := mergeNodeMappings(right, left, path)
			if err != nil {
				return nil, err
			}
			_, mergedSubnet := FindKey(merged, "subnet")
			mergedVal := ""
			if mergedSubnet != nil {
				mergedVal = mergedSubnet.Value
			}
			idx := slices.IndexFunc(ipamConfigs, func(n *yaml.Node) bool {
				_, s := FindKey(n, "subnet")
				return s != nil && s.Value == mergedVal
			})
			if idx >= 0 {
				ipamConfigs[idx] = merged
			} else {
				ipamConfigs = append(ipamConfigs, merged)
			}
		}
	}
	return NewSequence(ipamConfigs...), nil
}

// ExtendServiceNode merges a base service node with an override service node,
// using the same merge rules as ExtendService but operating on yaml.Node trees.
func ExtendServiceNode(base, override *yaml.Node) (*yaml.Node, error) {
	return MergeNodes(base, override, tree.NewPath("services.x"))
}
