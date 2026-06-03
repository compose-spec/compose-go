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

// Normalize injects implicit defaults (default networks, derived
// service dependencies, build defaults, implicit `name`, ...) into a
// map-shaped compose model. The function is a thin wrapper around
// NormalizeNode -- the canonical logic lives on the yaml.Node side
// where source positions are preserved -- and round-trips through
// yaml so callers that hold a map[string]any keep working.
func Normalize(dict map[string]any, env types.Mapping) (map[string]any, error) {
	var n yaml.Node
	if err := n.Encode(dict); err != nil {
		return nil, fmt.Errorf("normalize: encode map: %w", err)
	}
	if _, err := NormalizeNode(&n, env); err != nil {
		return nil, err
	}
	var out map[string]any
	if err := n.Decode(&out); err != nil {
		return nil, fmt.Errorf("normalize: decode after node normalize: %w", err)
	}
	return out, nil
}
