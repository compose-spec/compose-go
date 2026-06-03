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
// The Node-side implementation is the fix for the bare-key lookup
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

// CaptureSecretConfigContent walks the merged tree and, for each
// `secrets.NAME.environment` / `configs.NAME.environment` scalar,
// resolves the variable against the SourceContext.Environment of the
// layer that DECLARED that scalar. Returns two `secrets-name -> resolved
// value` and `configs-name -> resolved value` maps so the resolution can
// later survive a CanonicalNode round-trip that re-encodes subtrees and
// invalidates the *yaml.Node pointers backing `origins`.
//
// The lookup-at-origin behavior fixes a v2 limitation where the
// project-wide environment was the only scope: a secret declared in an
// included compose file whose env_file introduced the variable could
// not see it. The secret/config now resolves in the same scope its
// declaration would resolve `${VAR}` interpolation in -- the layer's
// own environment.
func CaptureSecretConfigContent(root *yaml.Node, origins map[*yaml.Node]*node.SourceContext) (map[string]string, map[string]string) {
	secrets := map[string]string{}
	configs := map[string]string{}
	if root == nil {
		return secrets, configs
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	if target.Kind != yaml.MappingNode {
		return secrets, configs
	}
	collect := func(section *yaml.Node, into map[string]string) {
		if section == nil || section.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(section.Content); i += 2 {
			name := section.Content[i].Value
			entry := section.Content[i+1]
			if entry.Kind != yaml.MappingNode {
				continue
			}
			env := mappingValueByKey(entry, "environment")
			if env == nil || env.Kind != yaml.ScalarNode || env.Value == "" {
				continue
			}
			ctx := origins[env]
			if ctx == nil {
				continue
			}
			if value, ok := ctx.Environment[env.Value]; ok {
				into[name] = value
			}
		}
	}
	collect(mappingValueByKey(target, "secrets"), secrets)
	collect(mappingValueByKey(target, "configs"), configs)
	return secrets, configs
}

// ApplySecretConfigContent injects each captured `name -> value` pair as
// a `content` scalar inside the corresponding entry of the post-canonical
// tree. Runs after the compose-rule validator so the mutual-exclusivity
// check between content and environment does not flag the synthesized
// value.
func ApplySecretConfigContent(root *yaml.Node, secrets, configs map[string]string) {
	if root == nil || (len(secrets) == 0 && len(configs) == 0) {
		return
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	if target.Kind != yaml.MappingNode {
		return
	}
	apply := func(section *yaml.Node, values map[string]string) {
		if section == nil || section.Kind != yaml.MappingNode || len(values) == 0 {
			return
		}
		for i := 0; i+1 < len(section.Content); i += 2 {
			name := section.Content[i].Value
			value, ok := values[name]
			if !ok {
				continue
			}
			entry := section.Content[i+1]
			if entry.Kind != yaml.MappingNode {
				continue
			}
			setMappingValue(entry, "content", &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: value,
			})
		}
	}
	apply(mappingValueByKey(target, "secrets"), secrets)
	apply(mappingValueByKey(target, "configs"), configs)
}
