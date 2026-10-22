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
	"maps"
	"slices"

	"github.com/compose-spec/compose-go/v2/internal/warn"
	"github.com/compose-spec/compose-go/v2/logging"
	"github.com/compose-spec/compose-go/v2/tree"
)

// externalResources are the top-level sections of resources that can be
// declared external.
var externalResources = []string{"volumes", "networks", "secrets", "configs"}

// warnDeprecatedExternalName warns, for each top-level resource declaring
// `external.name`, that the attribute is deprecated. It runs on the model
// before it is made canonical, which turns `external` into a boolean: the
// warnings are therefore raised even when the canonicalization fails later.
// Resources are visited in the order of their names.
func warnDeprecatedExternalName(yaml map[string]any, l logging.Logger) {
	for _, section := range externalResources {
		resources, ok := yaml[section].(map[string]any)
		if !ok {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(resources)) {
			m, ok := resources[name].(map[string]any)
			if !ok {
				continue
			}
			external, ok := m["external"].(map[string]any)
			if !ok {
				continue
			}
			if _, ok := external["name"]; ok {
				warn.Warnf(l, "%s: external.name is deprecated. Please set name and external: true", tree.NewPath(section, name))
			}
		}
	}
}

func transformMaybeExternal(data any, p tree.Path, ignoreParseError bool) (any, error) {
	if data == nil {
		return nil, nil
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: must be a mapping", p)
	}
	resource, err := transformMapping(m, p, ignoreParseError)
	if err != nil {
		return nil, err
	}

	if ext, ok := resource["external"]; ok {
		name, named := resource["name"]
		if external, ok := ext.(map[string]any); ok {
			resource["external"] = true
			if extname, extNamed := external["name"]; extNamed {
				if named && extname != name {
					return nil, fmt.Errorf("%s: name and external.name conflict; only use name", p)
				}
				if !named {
					// adopt (deprecated) external.name if set
					resource["name"] = extname
					return resource, nil
				}
			}
		}
	}

	return resource, nil
}
