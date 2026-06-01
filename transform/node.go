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
)

// CanonicalNode rewrites short-form syntax into canonical (long-form) syntax
// on a yaml.Node tree, using the same per-path transformers as Canonical.
//
// First cut: the function bridges through map[string]any — it decodes root,
// runs the existing Canonical, and rebuilds a yaml.Node from the result.
// This keeps the v3 wiring honest end-to-end while reusing the well-tested
// per-transformer rules of the v2 implementation. Subsequent commits will
// port individual transformers (transformPorts, transformVolumeMount,
// transformBuild, ...) to operate on *yaml.Node directly so that source
// positions survive the canonicalization for downstream diagnostics; until
// then the rebuilt tree has Line/Column zero on nodes that the bridge had
// to reconstruct.
//
// CanonicalNode mutates root in place: the inner Content of the document
// node is replaced with the encoded canonical tree. Returns root for
// convenience.
func CanonicalNode(root *yaml.Node, ignoreParseError bool) (*yaml.Node, error) {
	if root == nil {
		return nil, nil
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}

	var data map[string]any
	if err := target.Decode(&data); err != nil {
		return nil, fmt.Errorf("transform: decode for canonical bridge: %w", err)
	}

	canonical, err := Canonical(data, ignoreParseError)
	if err != nil {
		return nil, err
	}

	var rebuilt yaml.Node
	if err := rebuilt.Encode(canonical); err != nil {
		return nil, fmt.Errorf("transform: re-encode after canonical bridge: %w", err)
	}

	// Replace target's contents with the rebuilt mapping while keeping the
	// outer Document wrapper intact so callers that hold a pointer to root
	// keep observing the same value.
	*target = rebuilt
	return root, nil
}
