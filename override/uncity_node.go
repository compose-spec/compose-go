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

package override

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/format"
	"github.com/compose-spec/compose-go/v3/tree"
)

// nodeIndexer extracts a stable identity key from a sequence entry. Entries
// sharing the same key are de-duplicated by EnforceUnicityNode, with the
// last occurrence winning — matching the v2 semantics where the override
// file takes precedence over the base.
type nodeIndexer func(*yaml.Node, tree.Path) (string, error)

// uniqueNode mirrors override.unique but holds the Node-typed indexers.
// Entries are kept in sync between the two maps; the legacy map disappears
// when the v2 map[string]any path is removed.
var uniqueNode = map[tree.Path]nodeIndexer{}

func init() {
	uniqueNode["networks.*.labels"] = keyValueIndexerNode
	uniqueNode["networks.*.ipam.options"] = keyValueIndexerNode
	uniqueNode["services.*.annotations"] = keyValueIndexerNode
	uniqueNode["services.*.build.args"] = keyValueIndexerNode
	uniqueNode["services.*.build.additional_contexts"] = keyValueIndexerNode
	uniqueNode["services.*.build.platform"] = keyValueIndexerNode
	uniqueNode["services.*.build.tags"] = keyValueIndexerNode
	uniqueNode["services.*.build.labels"] = keyValueIndexerNode
	uniqueNode["services.*.cap_add"] = keyValueIndexerNode
	uniqueNode["services.*.cap_drop"] = keyValueIndexerNode
	uniqueNode["services.*.configs"] = mountIndexerNode("")
	uniqueNode["services.*.deploy.labels"] = keyValueIndexerNode
	uniqueNode["services.*.dns"] = keyValueIndexerNode
	uniqueNode["services.*.dns_opt"] = keyValueIndexerNode
	uniqueNode["services.*.dns_search"] = keyValueIndexerNode
	uniqueNode["services.*.environment"] = keyValueIndexerNode
	uniqueNode["services.*.env_file"] = envFileIndexerNode
	uniqueNode["services.*.expose"] = exposeIndexerNode
	uniqueNode["services.*.labels"] = keyValueIndexerNode
	uniqueNode["services.*.links"] = keyValueIndexerNode
	uniqueNode["services.*.networks.*.aliases"] = keyValueIndexerNode
	uniqueNode["services.*.networks.*.link_local_ips"] = keyValueIndexerNode
	uniqueNode["services.*.ports"] = portIndexerNode
	uniqueNode["services.*.profiles"] = keyValueIndexerNode
	uniqueNode["services.*.secrets"] = mountIndexerNode("/run/secrets")
	uniqueNode["services.*.sysctls"] = keyValueIndexerNode
	uniqueNode["services.*.tmpfs"] = keyValueIndexerNode
	uniqueNode["services.*.volumes"] = volumeIndexerNode
	uniqueNode["services.*.devices"] = deviceMappingIndexerNode
}

// EnforceUnicityNode removes duplicated entries in any sequence whose path
// matches uniqueNode. Inside each affected sequence, entries are indexed by
// the configured nodeIndexer; later occurrences replace earlier ones at the
// same key. Mappings outside the configured paths are recursed into but
// untouched.
//
// The function mutates root in place and returns it for convenience.
func EnforceUnicityNode(root *yaml.Node) (*yaml.Node, error) {
	root = unwrapDocumentNode(root)
	if err := enforceUnicityNode(root, tree.NewPath()); err != nil {
		return nil, err
	}
	return root, nil
}

func enforceUnicityNode(n *yaml.Node, p tree.Path) error {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if err := enforceUnicityNode(n.Content[i+1], p.Next(key)); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for pattern, indexer := range uniqueNode {
			if !p.Matches(pattern) {
				continue
			}
			result := make([]*yaml.Node, 0, len(n.Content))
			keys := map[string]int{}
			for i, entry := range n.Content {
				key, err := indexer(entry, p.Next(fmt.Sprintf("[%d]", i)))
				if err != nil {
					return err
				}
				if j, ok := keys[key]; ok {
					result[j] = entry
					continue
				}
				result = append(result, entry)
				keys[key] = len(result) - 1
			}
			n.Content = result
			return nil
		}
		// Recurse into nested containers when the sequence itself is not a
		// unicity target (the entries may themselves contain mappings whose
		// children are unicity-enforced).
		for i, entry := range n.Content {
			if err := enforceUnicityNode(entry, p.Next(fmt.Sprintf("[%d]", i))); err != nil {
				return err
			}
		}
	}
	return nil
}

func keyValueIndexerNode(n *yaml.Node, p tree.Path) (string, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s: unexpected non-scalar entry", p)
	}
	key, _, found := strings.Cut(n.Value, "=")
	if found {
		return key, nil
	}
	return n.Value, nil
}

func volumeIndexerNode(n *yaml.Node, p tree.Path) (string, error) {
	if n == nil {
		return "", nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		target := nodeMapGet(n, "target")
		if target == nil || target.Kind != yaml.ScalarNode {
			return "", fmt.Errorf("service volume %s is missing a mount target", p)
		}
		return target.Value, nil
	case yaml.ScalarNode:
		volume, err := format.ParseVolume(n.Value)
		if err != nil {
			return "", err
		}
		return volume.Target, nil
	}
	return "", nil
}

func deviceMappingIndexerNode(n *yaml.Node, p tree.Path) (string, error) {
	if n == nil {
		return "", nil
	}
	switch n.Kind {
	case yaml.MappingNode:
		target := nodeMapGet(n, "target")
		if target == nil || target.Kind != yaml.ScalarNode {
			return "", fmt.Errorf("service device %s is missing a mount target", p)
		}
		return target.Value, nil
	case yaml.ScalarNode:
		parts := strings.Split(n.Value, ":")
		if len(parts) == 1 {
			return parts[0], nil
		}
		return parts[1], nil
	}
	return "", nil
}

func exposeIndexerNode(n *yaml.Node, p tree.Path) (string, error) {
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s: unsupported expose value", p)
	}
	return n.Value, nil
}

func mountIndexerNode(defaultPath string) nodeIndexer {
	return func(n *yaml.Node, p tree.Path) (string, error) {
		if n == nil {
			return "", nil
		}
		switch n.Kind {
		case yaml.ScalarNode:
			return fmt.Sprintf("%s/%s", defaultPath, n.Value), nil
		case yaml.MappingNode:
			if target := nodeMapGet(n, "target"); target != nil && target.Kind == yaml.ScalarNode {
				return target.Value, nil
			}
			source := nodeMapGet(n, "source")
			if source != nil && source.Kind == yaml.ScalarNode {
				return fmt.Sprintf("%s/%s", defaultPath, source.Value), nil
			}
			return "", fmt.Errorf("%s: missing target or source", p)
		}
		return "", fmt.Errorf("%s: unsupported mount value", p)
	}
}

func portIndexerNode(n *yaml.Node, p tree.Path) (string, error) {
	if n == nil {
		return "", nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		// Could be a bare port number (int-tagged or untagged scalar) or a
		// "host:container/proto" short-form string. Use the literal Value as
		// the indexer key in both cases — different surface syntaxes that
		// describe the same port end up de-duplicated by EnforceUnicity at a
		// later stage if Canonical has normalized them by then.
		return n.Value, nil
	case yaml.MappingNode:
		target := nodeMapGet(n, "target")
		if target == nil {
			return "", fmt.Errorf("service ports %s is missing a target port", p)
		}
		published := nodeMapGet(n, "published")
		publishedStr := ""
		if published != nil {
			publishedStr = published.Value
		}
		host := scalarValueOrDefault(nodeMapGet(n, "host_ip"), "0.0.0.0")
		protocol := scalarValueOrDefault(nodeMapGet(n, "protocol"), "tcp")
		return fmt.Sprintf("%s:%s:%s/%s", host, publishedStr, target.Value, protocol), nil
	}
	return "", nil
}

func envFileIndexerNode(n *yaml.Node, p tree.Path) (string, error) {
	if n == nil {
		return "", nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return n.Value, nil
	case yaml.MappingNode:
		if pathValue := nodeMapGet(n, "path"); pathValue != nil && pathValue.Kind == yaml.ScalarNode {
			return pathValue.Value, nil
		}
		return "", fmt.Errorf("environment path attribute %s is missing", p)
	}
	return "", nil
}

func scalarValueOrDefault(n *yaml.Node, fallback string) string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Value == "" {
		return fallback
	}
	return n.Value
}
