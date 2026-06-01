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
	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
)

// ApplyResetPaths removes every mapping entry in root whose path matches one
// of the recorded reset paths. !override entries also feature in the list
// but their replacement semantics are handled by the merge phase (the
// override layer wins outright), so the value at the path on the right-hand
// merge tree is already correct; deleting an entry whose path matches a
// stored !override pattern is a no-op when the override layer carried a
// concrete value at that path.
//
// Sequence elements are not currently supported: !reset on an array entry
// is rejected by v3 with an explicit error rather than silently being
// applied, matching the decision recorded in the plan.
//
// ApplyResetPaths mutates root in place. Returns root for convenience.
func ApplyResetPaths(root *yaml.Node, paths []tree.Path) *yaml.Node {
	if root == nil || len(paths) == 0 {
		return root
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	applyResetPaths(target, tree.NewPath(), paths)
	return root
}

func applyResetPaths(n *yaml.Node, p tree.Path, patterns []tree.Path) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		filtered := make([]*yaml.Node, 0, len(n.Content))
		for i := 0; i+1 < len(n.Content); i += 2 {
			next := p.Next(n.Content[i].Value)
			if matchesAny(next, patterns) {
				continue
			}
			applyResetPaths(n.Content[i+1], next, patterns)
			filtered = append(filtered, n.Content[i], n.Content[i+1])
		}
		n.Content = filtered
	case yaml.SequenceNode:
		for i, c := range n.Content {
			applyResetPaths(c, p.Next(tree.PathMatchList), patterns)
			_ = i
		}
	}
}

func matchesAny(p tree.Path, patterns []tree.Path) bool {
	for _, pattern := range patterns {
		if p.Matches(pattern) {
			return true
		}
	}
	return false
}
