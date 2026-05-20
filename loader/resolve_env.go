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

	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// resolveBareEnvironmentRefs walks each layer and turns bare environment
// references (an entry written as just the variable name, e.g.
// `environment: - VAR_NAME`) into a fully qualified `VAR_NAME=value` entry
// using the layer's NodeContext.Env mapping.
//
// This must run before the merge pass because each layer's Env is specific
// to its own loading context (included files may carry variables from
// include.env_file that the main project does not see).
func (m *ComposeModel) resolveBareEnvironmentRefs() {
	for _, layer := range m.layers {
		resolveBareEnvLayer(layer.Root, layer.Context)
		resolveSecretsConfigsEnvLayer(layer.Root, layer.Context)
	}
}

// resolveSecretsConfigsEnvLayer mirrors loader.resolveSecretsEnvironment and
// resolveConfigsEnvironment but operates on a yaml.Node tree, per layer,
// so the lookup uses the layer's NodeContext.Env. Without this, a secret
// or config declared in an included file with `environment: VAR` and a
// VAR provided by include.env_file would not be resolved (the main
// configDetails.Environment knows nothing about VAR).
//
// For each matching entry, the layer's value is recorded under the
// loader-private key types.SecretConfigXValue ("x-#value"), which is the
// transport the legacy ResolveEnvironment uses to forward the value to
// the runtime.
func resolveSecretsConfigsEnvLayer(root *yaml.Node, ctx *types.NodeContext) {
	if ctx == nil || len(ctx.Env) == 0 {
		return
	}
	doc := unwrapDocument(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return
	}
	for _, section := range []string{"secrets", "configs"} {
		_, sectionNode := override.FindKey(doc, section)
		if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(sectionNode.Content); i += 2 {
			entry := sectionNode.Content[i+1]
			if entry == nil || entry.Kind != yaml.MappingNode {
				continue
			}
			_, envKey := override.FindKey(entry, "environment")
			if envKey == nil || envKey.Kind != yaml.ScalarNode {
				continue
			}
			if v, ok := ctx.Env[envKey.Value]; ok {
				override.SetKey(entry, types.SecretConfigXValue, override.NewScalar(v))
			}
		}
	}
}

func resolveBareEnvLayer(root *yaml.Node, ctx *types.NodeContext) {
	if ctx == nil || len(ctx.Env) == 0 {
		return
	}
	doc := unwrapDocument(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return
	}
	_, services := override.FindKey(doc, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svc := services.Content[i+1]
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		_, env := override.FindKey(svc, "environment")
		if env == nil {
			continue
		}
		resolveBareEnvNode(env, ctx.Env)
	}
}

// resolveBareEnvNode handles both the sequence form (a list of "KEY" or
// "KEY=value" scalars) and the mapping form (where a key with a null value
// is the bare reference).
func resolveBareEnvNode(node *yaml.Node, env types.Mapping) {
	switch node.Kind {
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			if strings.Contains(item.Value, "=") {
				continue
			}
			if v, ok := env[item.Value]; ok {
				item.Value = fmt.Sprintf("%s=%s", item.Value, v)
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Kind != yaml.ScalarNode || val.Tag != "!!null" {
				continue
			}
			if v, ok := env[key.Value]; ok {
				val.Value = v
				val.Tag = "!!str"
			}
		}
	}
}
