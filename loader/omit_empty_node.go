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
	"github.com/compose-spec/compose-go/v3/tree"
	"go.yaml.in/yaml/v4"
)

// omitEmptyNodePatterns lists the tree paths whose children must be
// dropped when empty (nil or empty string scalar). Mirrors the legacy
// `omitempty` table in loader/omitEmpty.go.
var omitEmptyNodePatterns = []tree.Path{
	"services.*.dns",
}

// omitEmptyNodes walks the merged yaml.Node tree and removes any entry
// that matches one of the omit patterns when its value is "empty"
// (yaml-null, empty scalar, or an empty sequence/mapping). It mirrors
// loader.OmitEmpty (which operates on map[string]any) but works on the
// yaml.Node tree so the v3 pipeline can drop the map-based pass.
func omitEmptyNodes(root *yaml.Node) {
	omitEmptyNodeWalk(root, tree.NewPath())
}

func omitEmptyNodeWalk(node *yaml.Node, p tree.Path) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, c := range node.Content {
			omitEmptyNodeWalk(c, p)
		}
	case yaml.MappingNode:
		kept := node.Content[:0]
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			childPath := p.Next(key.Value)
			if isEmptyYamlNode(val) && mustOmitNode(childPath) {
				continue
			}
			omitEmptyNodeWalk(val, childPath)
			kept = append(kept, key, val)
		}
		node.Content = kept
	case yaml.SequenceNode:
		kept := node.Content[:0]
		for _, item := range node.Content {
			childPath := p.Next(tree.PathMatchList)
			if isEmptyYamlNode(item) && mustOmitNode(childPath) {
				continue
			}
			omitEmptyNodeWalk(item, childPath)
			kept = append(kept, item)
		}
		node.Content = kept
	}
}

func mustOmitNode(p tree.Path) bool {
	for _, pattern := range omitEmptyNodePatterns {
		if p.Matches(pattern) {
			return true
		}
	}
	return false
}

// isEmptyYamlNode mirrors loader.isEmpty for yaml.Node values: a node is
// empty when it is a null scalar, an empty string scalar, or an empty
// mapping/sequence.
func isEmptyYamlNode(n *yaml.Node) bool {
	if n == nil {
		return true
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "!!null" {
			return true
		}
		return n.Value == ""
	case yaml.MappingNode, yaml.SequenceNode:
		return len(n.Content) == 0
	}
	return false
}
