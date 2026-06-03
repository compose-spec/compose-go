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

package transform

import (
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
)

// CanonicalNode rewrites short-form syntax into canonical (long-form) syntax
// on a yaml.Node tree.
//
// Walker design: instead of decoding the whole tree into map[string]any and
// re-encoding (which zeroed Line / Column on every fresh node), recurse
// node by node and invoke the per-path transformer only on the matching
// subtree. The decode + encode round-trip is therefore scoped to the
// smallest subtree that needs reshaping, and every parent / sibling node
// keeps the original source position the YAML parser recorded. Downstream
// diagnostics (errdefs.Diagnostic) consume those positions through the
// origins side-table, so the smaller the subtree that loses Line / Column
// the better the user-facing error message.
//
// CanonicalNode mutates root in place and returns root for convenience.
func CanonicalNode(root *yaml.Node, ignoreParseError bool) (*yaml.Node, error) {
	if root == nil {
		return nil, nil
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	if err := canonicalizeNode(target, tree.NewPath(), ignoreParseError); err != nil {
		return nil, err
	}
	return root, nil
}

// canonicalizeNode walks n in place, applying the matching transformer
// to the smallest subtree that matches a registered pattern. Nodes that
// no transformer claims are traversed structurally so their children
// can themselves match -- and untouched scalars keep their original
// Line / Column.
func canonicalizeNode(n *yaml.Node, p tree.Path, ignoreParseError bool) error {
	if n == nil {
		return nil
	}
	for pattern, transformer := range transformers {
		if p.Matches(pattern) {
			return applyTransformer(n, p, transformer, ignoreParseError)
		}
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if err := canonicalizeNode(n.Content[i+1], p.Next(n.Content[i].Value), ignoreParseError); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if err := canonicalizeNode(c, p.Next(tree.PathMatchList), ignoreParseError); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyTransformer runs the legacy map / slice based transformer on
// the scoped subtree at n by going through a minimal decode + encode
// round-trip. Only this subtree loses Line / Column on its fresh
// nodes; every ancestor and sibling keeps the original position.
func applyTransformer(n *yaml.Node, p tree.Path, transformer Func, ignoreParseError bool) error {
	var raw any
	if err := n.Decode(&raw); err != nil {
		return fmt.Errorf("transform %s: decode for canonical: %w", p, err)
	}
	transformed, err := transformer(raw, p, ignoreParseError)
	if err != nil {
		return err
	}
	// Recurse into the transformed value so nested patterns still fire.
	// Example: transformService at "services.*" rewrites the service
	// shape; nested transformers like "services.*.ports" need to run
	// next on the rewritten shape.
	switch v := transformed.(type) {
	case map[string]any:
		if v, err = transformMapping(v, p, ignoreParseError); err != nil {
			return err
		}
		transformed = v
	case []any:
		if v, err = transformSequence(v, p, ignoreParseError); err != nil {
			return err
		}
		transformed = v
	}
	var rebuilt yaml.Node
	if err := rebuilt.Encode(transformed); err != nil {
		return fmt.Errorf("transform %s: re-encode after canonical: %w", p, err)
	}
	// Replace n's content with the rebuilt subtree while keeping the
	// outer node pointer intact so callers that walked into this node
	// still observe the canonical shape.
	*n = rebuilt
	return nil
}
