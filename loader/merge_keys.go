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

	"go.yaml.in/yaml/v4"
)

// resolveMergeKeys walks the yaml tree and inlines every YAML merge key
// (`<<: *anchor` or `<<: [*a, *b]`) into its surrounding mapping.
//
// Without this pass, the merge keys would still appear as `<<` entries
// in the post-merge dict and fail JSON Schema validation
// ("additional properties '<<' not allowed"). yaml/v4 keeps merge keys
// in the Node tree (Tag "!!merge") and only resolves them when Decode
// is called against a Go struct — never inside an interface{} or map.
//
// Returns an error if a merge key would introduce a cycle: a mapping
// whose `<<` references one of its enclosing mappings (directly or
// through nested aliases).
func resolveMergeKeys(node *yaml.Node) error {
	return resolveMergeKeysVisited(node, map[*yaml.Node]bool{}, map[*yaml.Node]bool{})
}

func resolveMergeKeysVisited(node *yaml.Node, ancestors, visited map[*yaml.Node]bool) error {
	if node == nil {
		return nil
	}
	// ancestors tracks nodes on the current DFS path; visited tracks
	// nodes whose subtree has been fully processed. A node on the
	// ancestors path is a structural cycle (e.g. self-referencing
	// anchor) — skip without diving deeper to avoid an infinite walk.
	if ancestors[node] {
		return nil
	}
	if visited[node] {
		return nil
	}
	ancestors[node] = true
	defer func() {
		delete(ancestors, node)
		visited[node] = true
	}()

	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range node.Content {
			if err := resolveMergeKeysVisited(c, ancestors, visited); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		// Two-pass: descend into children first, then merge keys at this
		// level. Descending first ensures inner mappings get their own
		// merge keys resolved before they are used as merge sources.
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := resolveMergeKeysVisited(node.Content[i+1], ancestors, visited); err != nil {
				return err
			}
		}
		if err := inlineMergeKeys(node, ancestors); err != nil {
			return err
		}
	case yaml.AliasNode:
		if node.Alias != nil {
			if err := resolveMergeKeysVisited(node.Alias, ancestors, visited); err != nil {
				return err
			}
		}
	}
	return nil
}

// inlineMergeKeys removes `<<` entries from a mapping and copies the
// referenced mappings into it. Existing keys take precedence over merged
// ones (the merge key only fills in missing keys), matching the YAML 1.1
// merge key semantics. Returns an error if any source mapping is — or
// reaches — a mapping currently being merged (cycle).
func inlineMergeKeys(node *yaml.Node, ancestors map[*yaml.Node]bool) error {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	var rest []*yaml.Node
	var sources []*yaml.Node
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		val := node.Content[i+1]
		if key.Tag == "!!merge" || key.Value == "<<" {
			sources = append(sources, collectMergeSources(val)...)
			continue
		}
		rest = append(rest, key, val)
	}
	if len(sources) == 0 {
		return nil
	}
	for _, src := range sources {
		if mappingContains(src, ancestors, map[*yaml.Node]bool{}) {
			return fmt.Errorf("merge key cycle detected at line %d column %d", node.Line, node.Column)
		}
	}
	existing := map[string]bool{}
	for i := 0; i+1 < len(rest); i += 2 {
		existing[rest[i].Value] = true
	}
	for _, src := range sources {
		if src == nil || src.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(src.Content); i += 2 {
			k := src.Content[i]
			v := src.Content[i+1]
			if existing[k.Value] {
				continue
			}
			rest = append(rest, k, v)
			existing[k.Value] = true
		}
	}
	node.Content = rest
	return nil
}

// mappingContains reports whether root is or contains any node in targets,
// following aliases. visited prevents traversal cycles on the yaml tree
// itself.
func mappingContains(root *yaml.Node, targets, visited map[*yaml.Node]bool) bool {
	if root == nil || visited[root] {
		return false
	}
	visited[root] = true
	if targets[root] {
		return true
	}
	if root.Kind == yaml.AliasNode && root.Alias != nil {
		return mappingContains(root.Alias, targets, visited)
	}
	for _, c := range root.Content {
		if mappingContains(c, targets, visited) {
			return true
		}
	}
	return false
}

// collectMergeSources unwraps the value of a merge key entry into a flat
// list of source mappings. The value may be a single mapping/alias or a
// sequence of mappings/aliases.
func collectMergeSources(node *yaml.Node) []*yaml.Node {
	if node == nil {
		return nil
	}
	resolved := node
	if resolved.Kind == yaml.AliasNode && resolved.Alias != nil {
		resolved = resolved.Alias
	}
	switch resolved.Kind {
	case yaml.MappingNode:
		return []*yaml.Node{resolved}
	case yaml.SequenceNode:
		var out []*yaml.Node
		for _, item := range resolved.Content {
			out = append(out, collectMergeSources(item)...)
		}
		return out
	}
	return nil
}
