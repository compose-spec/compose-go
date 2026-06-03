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

// Visit is the callback invoked by Walk at every meaningful position in a
// yaml.Node tree.
//
// path is the dotted tree.Path leading to n; an empty Path denotes the root.
// Sequence elements are represented by the tree.PathMatchList token "[]" to
// stay consistent with the patterns used by override, paths, transform and
// validation throughout the codebase.
//
// Returning a non-nil error aborts the walk; that same error is returned by
// Walk.
type Visit func(path tree.Path, n *yaml.Node) error

// Walk traverses a yaml.Node tree depth-first, invoking fn at every node
// reachable from root that maps to a meaningful Compose path:
//
//   - the root itself, with an empty Path;
//   - every value of every MappingNode, with the path extended by the key;
//   - every element of every SequenceNode, with the path extended by "[]".
//
// DocumentNodes are unwrapped transparently (Walk recurses into Content
// without invoking fn for them). AliasNodes are followed once: their target is
// visited at the alias's path. Cycles between aliases are broken silently to
// avoid infinite recursion; reset / override resolution is responsible for
// reporting them.
//
// Mapping keys themselves are not visited; only their values are. Callers that
// need to inspect a key alongside its value can retrieve the key from
// n.Content[i] when visiting the parent MappingNode in a separate pass.
func Walk(root *yaml.Node, fn Visit) error {
	return walk(root, tree.NewPath(), fn, map[*yaml.Node]struct{}{})
}

func walk(n *yaml.Node, path tree.Path, fn Visit, seen map[*yaml.Node]struct{}) error {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode {
		for _, child := range n.Content {
			if err := walk(child, path, fn, seen); err != nil {
				return err
			}
		}
		return nil
	}
	if n.Kind == yaml.AliasNode {
		target := n.Alias
		if target == nil {
			return nil
		}
		if _, cycle := seen[target]; cycle {
			return nil
		}
		seen[target] = struct{}{}
		defer delete(seen, target)
		return walk(target, path, fn, seen)
	}
	if err := fn(path, n); err != nil {
		return err
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i]
			value := n.Content[i+1]
			if err := walk(value, path.Next(key.Value), fn, seen); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range n.Content {
			if err := walk(child, path.Next(tree.PathMatchList), fn, seen); err != nil {
				return err
			}
		}
	}
	return nil
}
