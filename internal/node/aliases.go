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

package node

import (
	"fmt"

	"go.yaml.in/yaml/v4"
)

// NormalizeAliases walks the yaml.Node tree and removes every AliasNode by
// substituting a deep copy of the alias target in its place, then folds YAML
// merge keys (`<<: *ref`, `<<: [*a, *b]`) into the surrounding mapping with
// surrounding-wins semantics.
//
// After NormalizeAliases returns, no AliasNode remains in the tree and no
// mapping has a `<<` key. The subsequent pipeline phases (cross-file merge,
// interpolation, transform, decode) can therefore operate without any alias
// indirection, which is what makes the per-file Layer model self-contained.
//
// Aliases are deep-copied (rather than reused) because the merge phase
// mutates nodes in place: a node shared between two locations would otherwise
// be corrupted by the first merge involving it. Anchor names are not
// preserved on the copies; once the unfold pass completes, no anchor remains
// reachable.
//
// Cycles in alias chains (A references B which references A) are detected
// during the unfold pass and reported as errors. Cycles created by merge
// keys that resolve to the surrounding mapping are detected the same way
// because the merge value is itself an alias.
func NormalizeAliases(root *yaml.Node) error {
	if root == nil {
		return nil
	}
	if err := unfoldAliases(root, map[*yaml.Node]bool{}, map[*yaml.Node]bool{}); err != nil {
		return err
	}
	foldMergeKeys(root)
	return nil
}

// unfoldAliases replaces AliasNode children of n with deep copies of their
// resolved targets. inProgress tracks targets whose unfolding is on the
// current call stack so cycles are detected; cleaned remembers targets that
// have already been fully unfolded so anchor reuse stays linear in the
// number of distinct anchors (defense against alias bombs).
func unfoldAliases(n *yaml.Node, inProgress, cleaned map[*yaml.Node]bool) error {
	if n == nil {
		return nil
	}
	for i, child := range n.Content {
		if child == nil {
			continue
		}
		if child.Kind == yaml.AliasNode {
			target := child.Alias
			if target == nil {
				continue
			}
			if inProgress[target] {
				return fmt.Errorf("cycle detected in alias chain at line %d", child.Line)
			}
			if !cleaned[target] {
				inProgress[target] = true
				if err := unfoldAliases(target, inProgress, cleaned); err != nil {
					return err
				}
				delete(inProgress, target)
				cleaned[target] = true
			}
			n.Content[i] = deepCopy(target)
			continue
		}
		if err := unfoldAliases(child, inProgress, cleaned); err != nil {
			return err
		}
	}
	return nil
}

// deepCopy returns a structural copy of n with all nested content cloned.
// Anchor and Alias fields are cleared on the copy: the result is a plain
// concrete subtree, no longer participating in the YAML anchor graph.
// Position information (Line, Column) and Style are preserved so diagnostics
// downstream still point at the original source location, even though the
// node has been duplicated.
func deepCopy(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        n.Kind,
		Tag:         n.Tag,
		Value:       n.Value,
		Style:       n.Style,
		Line:        n.Line,
		Column:      n.Column,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
	}
	if len(n.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			clone.Content[i] = deepCopy(c)
		}
	}
	return clone
}

// foldMergeKeys eliminates `<<` entries from every MappingNode in the tree.
// For each MappingNode, the explicit keys defined on the mapping itself take
// precedence; then, for each merge source in declaration order, any key not
// yet present is appended. A merge value can be a single mapping or a
// sequence of mappings (the YAML 1.1 merge key spec); sequence entries are
// processed in order, with earlier entries winning over later ones — the
// same semantics yaml.Decoder would apply when decoding the unfolded tree
// directly.
//
// Recursion is depth-first so that inner mappings fold their own `<<`
// entries before their parents see them. By this point in the pipeline,
// aliases have already been unfolded, so every merge value is a concrete
// mapping (or sequence of mappings) and no alias indirection remains.
func foldMergeKeys(n *yaml.Node) {
	if n == nil {
		return
	}
	for _, c := range n.Content {
		foldMergeKeys(c)
	}
	if n.Kind != yaml.MappingNode {
		return
	}

	var result []*yaml.Node
	var mergeSources []*yaml.Node
	seen := map[string]bool{}

	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i]
		value := n.Content[i+1]
		if key.Tag == "!!merge" || key.Value == "<<" {
			switch value.Kind {
			case yaml.MappingNode:
				mergeSources = append(mergeSources, value)
			case yaml.SequenceNode:
				for _, item := range value.Content {
					if item != nil && item.Kind == yaml.MappingNode {
						mergeSources = append(mergeSources, item)
					}
				}
			}
			continue
		}
		seen[key.Value] = true
		result = append(result, key, value)
	}

	for _, src := range mergeSources {
		for i := 0; i+1 < len(src.Content); i += 2 {
			key := src.Content[i]
			value := src.Content[i+1]
			if seen[key.Value] {
				continue
			}
			seen[key.Value] = true
			result = append(result, key, value)
		}
	}
	n.Content = result
}
