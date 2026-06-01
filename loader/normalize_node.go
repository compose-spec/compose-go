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

	"github.com/compose-spec/compose-go/v3/types"
)

// NormalizeNode injects implicit defaults (default networks, derived service
// dependencies, build defaults, ...) into a parsed yaml.Node tree using the
// same rules as Normalize.
//
// First cut: the function bridges through map[string]any — it decodes root,
// runs Normalize, and rebuilds a yaml.Node from the result. This reuses the
// well-tested per-rule logic of the v2 implementation while keeping the v3
// pipeline honest end-to-end. Subsequent commits will port the individual
// normalization steps (networks, dependencies, builds, ...) to operate on
// *yaml.Node directly so that source positions survive normalization for
// downstream diagnostics; until then the rebuilt subtree has Line / Column
// zero on synthesized nodes (default network, derived depends_on entries).
//
// NormalizeNode mutates root in place: the inner Content of the document
// wrapper is replaced with the encoded normalized tree. Returns root for
// convenience.
func NormalizeNode(root *yaml.Node, env types.Mapping) (*yaml.Node, error) {
	if root == nil {
		return nil, nil
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}

	var data map[string]any
	if err := target.Decode(&data); err != nil {
		return nil, fmt.Errorf("normalize: decode for bridge: %w", err)
	}

	normalized, err := Normalize(data, env)
	if err != nil {
		return nil, err
	}

	var rebuilt yaml.Node
	if err := rebuilt.Encode(normalized); err != nil {
		return nil, fmt.Errorf("normalize: re-encode after bridge: %w", err)
	}

	*target = rebuilt
	return root, nil
}
