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
	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// attachEnvFileContexts walks the merged yaml tree and copies the per-node
// NodeContext into the corresponding EnvFile.Context slot of every decoded
// service. After this pass, WithServicesEnvironmentResolved can resolve the
// env_file paths against the right working directory and interpolate their
// content using the right environment, including variables provided by an
// enclosing include.env_file.
//
// merged is the post-merge yaml tree the project was decoded from.
func (m *ComposeModel) attachEnvFileContexts(merged *yaml.Node, project *types.Project) {
	root := unwrapDocument(merged)
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	_, services := override.FindKey(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		svcNode := services.Content[i+1]
		svc, ok := project.Services[name]
		if !ok {
			continue
		}

		_, efNode := override.FindKey(svcNode, "env_file")
		if efNode == nil {
			continue
		}

		entries := envFileEntryNodes(efNode)
		for idx := range svc.EnvFiles {
			if idx >= len(entries) {
				break
			}
			entry := entries[idx]
			if entry == nil {
				continue
			}
			if ctx, found := m.contexts[entry]; found {
				svc.EnvFiles[idx].Context = ctx
			}
		}
		project.Services[name] = svc
	}
}

// envFileEntryNodes returns one *yaml.Node per env_file entry, matching the
// order produced by decoding into []EnvFile. It accepts the three forms
// allowed by Compose:
//   - env_file: ./single.env             (scalar → one entry)
//   - env_file: [a.env, b.env]           (sequence of scalars)
//   - env_file: [{path: a.env, required: false}, ...] (sequence of mappings)
//
// For mapping items, the path scalar node is returned so its context can be
// looked up.
func envFileEntryNodes(node *yaml.Node) []*yaml.Node {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.ScalarNode:
		return []*yaml.Node{node}
	case yaml.SequenceNode:
		out := make([]*yaml.Node, 0, len(node.Content))
		for _, item := range node.Content {
			switch item.Kind {
			case yaml.ScalarNode:
				out = append(out, item)
			case yaml.MappingNode:
				if _, p := override.FindKey(item, "path"); p != nil {
					out = append(out, p)
				} else {
					out = append(out, item)
				}
			default:
				out = append(out, item)
			}
		}
		return out
	}
	return nil
}
