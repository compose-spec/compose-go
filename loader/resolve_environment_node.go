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
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/internal/node"
)

// ResolveEnvironmentNode walks the merged yaml.Node tree and resolves the
// bare-key entries in services.*.environment, secrets.*.environment and
// configs.*.environment by looking each variable up against the scalar
// SourceContext.Environment. When the variable is found, the scalar is
// rewritten in "KEY=value" form; when missing, the scalar is left as-is
// (matching the v2 ResolveEnvironment behavior that distinguishes
// "interpolation produced the empty string" from "value cannot be
// resolved").
//
// The Node-side implementation is the v3 fix for the bare-key lookup
// quirk: the lookup is performed in the SourceContext of the scalar itself,
// not in the project-wide environment, so an env_file declared on an
// include block becomes visible to services defined inside that include
// even though the parent project environment does not carry the variable.
func ResolveEnvironmentNode(root *yaml.Node, origins map[*yaml.Node]*node.SourceContext) {
	if root == nil {
		return
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	if target.Kind != yaml.MappingNode {
		return
	}
	resolveTopLevel := func(topKey, inner string) {
		section := mappingValueByKey(target, topKey)
		if section == nil || section.Kind != yaml.MappingNode {
			return
		}
		for i := 1; i < len(section.Content); i += 2 {
			entry := section.Content[i]
			if entry.Kind != yaml.MappingNode {
				continue
			}
			env := mappingValueByKey(entry, inner)
			if env == nil || env.Kind != yaml.SequenceNode {
				continue
			}
			resolveEnvSequence(env, origins)
		}
	}
	resolveTopLevel("services", "environment")
	resolveTopLevel("secrets", "environment")
	resolveTopLevel("configs", "environment")
}

func resolveEnvSequence(seq *yaml.Node, origins map[*yaml.Node]*node.SourceContext) {
	for _, item := range seq.Content {
		if item.Kind != yaml.ScalarNode {
			continue
		}
		if strings.Contains(item.Value, "=") {
			continue
		}
		ctx := origins[item]
		if ctx == nil {
			continue
		}
		if value, ok := ctx.Environment[item.Value]; ok {
			item.Value = fmt.Sprintf("%s=%s", item.Value, value)
		}
	}
}
