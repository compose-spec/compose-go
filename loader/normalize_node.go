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
	"path"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/types"
)

// NormalizeNode injects implicit defaults (default networks, derived
// service dependencies, build defaults, implicit `name`, ...) into the
// merged yaml.Node tree.
//
// The walker keeps the root mapping pointer stable and operates section
// by section so untouched siblings (and untouched scalars inside
// touched sections) keep the Line / Column the YAML parser recorded.
// Downstream diagnostics still hit the right source location after
// normalize runs.
func NormalizeNode(root *yaml.Node, env types.Mapping) (*yaml.Node, error) {
	if root == nil {
		return nil, nil
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	if target.Kind != yaml.MappingNode {
		return root, nil
	}

	normalizeNetworksNode(target)
	if err := normalizeServicesNode(target, env); err != nil {
		return nil, err
	}
	setNameFromKeyNode(target)
	return root, nil
}

// normalizeNetworksNode injects the implicit `default` network when any
// service does not opt out (network_mode, provider, explicit networks)
// and ensures the top-level `networks` mapping carries the entry.
func normalizeNetworksNode(root *yaml.Node) {
	networks := mappingValueByKey(root, "networks")
	if networks == nil {
		networks = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}

	usesDefault := false
	services := mappingValueByKey(root, "services")
	if services != nil && services.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(services.Content); i += 2 {
			svc := services.Content[i+1]
			if svc == nil || svc.Kind != yaml.MappingNode {
				continue
			}
			if mappingValueByKey(svc, "provider") != nil {
				continue
			}
			if mappingValueByKey(svc, "network_mode") != nil {
				continue
			}
			netsKey := mappingFieldNode(svc, "networks")
			if netsKey == nil {
				setMappingValue(svc, "networks", defaultNetworkOnly())
				usesDefault = true
				continue
			}
			if netsKey.Kind != yaml.MappingNode || len(netsKey.Content) == 0 {
				setMappingValue(svc, "networks", defaultNetworkOnly())
				usesDefault = true
				continue
			}
			if mappingValueByKey(netsKey, "default") != nil {
				usesDefault = true
			}
		}
	}

	if usesDefault && mappingValueByKey(networks, "default") == nil {
		setMappingValue(networks, "default", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null"})
	}
	if networks.Kind == yaml.MappingNode && len(networks.Content) > 0 {
		setMappingValue(root, "networks", networks)
	}
}

// defaultNetworkOnly returns the canonical `{default: null}` mapping
// used as the `networks` value on services that did not declare any.
func defaultNetworkOnly() *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "default"},
			{Kind: yaml.ScalarNode, Tag: "!!null"},
		},
	}
}

// normalizeServicesNode walks each service and applies the per-service
// normalizations (pull_policy alias, build defaults, environment
// resolution, derived depends_on, volume target cleanup).
func normalizeServicesNode(root *yaml.Node, env types.Mapping) error {
	services := mappingValueByKey(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svc := services.Content[i+1]
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		normalizePullPolicy(svc)
		if err := normalizeBuild(svc, env); err != nil {
			return err
		}
		normalizeServiceEnvironment(svc, env)
		normalizeServiceDependsOn(svc)
		normalizeVolumeTargets(svc)
	}
	return nil
}

func normalizePullPolicy(svc *yaml.Node) {
	pp := mappingValueByKey(svc, "pull_policy")
	if pp == nil || pp.Kind != yaml.ScalarNode {
		return
	}
	if pp.Value == types.PullPolicyIfNotPresent {
		pp.Value = types.PullPolicyMissing
	}
}

func normalizeBuild(svc *yaml.Node, env types.Mapping) error {
	build := mappingValueByKey(svc, "build")
	if build == nil || build.Kind != yaml.MappingNode {
		return nil
	}
	if mappingValueByKey(build, "context") == nil {
		setMappingValue(build, "context", &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: ".",
		})
	}
	if mappingValueByKey(build, "dockerfile") == nil && mappingValueByKey(build, "dockerfile_inline") == nil {
		setMappingValue(build, "dockerfile", &yaml.Node{
			Kind: yaml.ScalarNode, Tag: "!!str", Value: "Dockerfile",
		})
	}
	if args := mappingValueByKey(build, "args"); args != nil {
		resolveSequenceOrMapping(args, env, false)
	}
	return nil
}

func normalizeServiceEnvironment(svc *yaml.Node, env types.Mapping) {
	e := mappingValueByKey(svc, "environment")
	if e == nil {
		return
	}
	resolveSequenceOrMapping(e, env, true)
}

// resolveSequenceOrMapping rewrites bare `KEY` entries to `KEY=value`
// when the variable is set in env. Operates on both sequence form (list
// of strings) and mapping form (with null values). When keepEmpty is
// false, unset entries in mapping form are dropped.
func resolveSequenceOrMapping(n *yaml.Node, env types.Mapping, keepEmpty bool) {
	switch n.Kind {
	case yaml.SequenceNode:
		filtered := n.Content[:0]
		for _, item := range n.Content {
			if item == nil || item.Kind != yaml.ScalarNode {
				filtered = append(filtered, item)
				continue
			}
			if strings.Contains(item.Value, "=") {
				filtered = append(filtered, item)
				continue
			}
			if v, ok := env[item.Value]; ok {
				item.Value = fmt.Sprintf("%s=%s", item.Value, v)
				filtered = append(filtered, item)
				continue
			}
			if keepEmpty {
				filtered = append(filtered, item)
			}
		}
		n.Content = filtered
	case yaml.MappingNode:
		filtered := n.Content[:0]
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if v != nil && v.Kind == yaml.ScalarNode && v.Tag != "!!null" {
				filtered = append(filtered, k, v)
				continue
			}
			if val, ok := env[k.Value]; ok {
				filtered = append(filtered, k, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val})
				continue
			}
			if keepEmpty {
				filtered = append(filtered, k, v)
			}
		}
		n.Content = filtered
	}
}

// normalizeServiceDependsOn derives implicit depends_on entries from
// links, namespace references (network_mode/ipc/pid/uts/cgroup with the
// "service:" prefix) and volumes_from. The existing depends_on mapping
// is mutated in place; an empty section is left untouched.
func normalizeServiceDependsOn(svc *yaml.Node) {
	dependsOn := mappingValueByKey(svc, "depends_on")
	created := false
	if dependsOn == nil || dependsOn.Kind != yaml.MappingNode {
		dependsOn = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		created = true
	}

	addDep := func(name string, restart bool) {
		if name == "" {
			return
		}
		if mappingValueByKey(dependsOn, name) != nil {
			return
		}
		setMappingValue(dependsOn, name, dependsOnEntry(restart))
	}

	if links := mappingValueByKey(svc, "links"); links != nil && links.Kind == yaml.SequenceNode {
		for _, item := range links.Content {
			if item == nil || item.Kind != yaml.ScalarNode {
				continue
			}
			link := item.Value
			parts := strings.Split(link, ":")
			if len(parts) == 2 {
				link = parts[0]
			}
			addDep(link, true)
		}
	}

	for _, namespace := range []string{"network_mode", "ipc", "pid", "uts", "cgroup"} {
		ref := mappingValueByKey(svc, namespace)
		if ref == nil || ref.Kind != yaml.ScalarNode {
			continue
		}
		if !strings.HasPrefix(ref.Value, types.ServicePrefix) {
			continue
		}
		addDep(ref.Value[len(types.ServicePrefix):], true)
	}

	if vf := mappingValueByKey(svc, "volumes_from"); vf != nil && vf.Kind == yaml.SequenceNode {
		for _, item := range vf.Content {
			if item == nil || item.Kind != yaml.ScalarNode {
				continue
			}
			vol := item.Value
			if strings.HasPrefix(vol, types.ContainerPrefix) {
				continue
			}
			spec := strings.Split(vol, ":")
			addDep(spec[0], false)
		}
	}

	if len(dependsOn.Content) == 0 {
		return
	}
	if created {
		setMappingValue(svc, "depends_on", dependsOn)
	}
}

func dependsOnEntry(restart bool) *yaml.Node {
	restartScalar := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	if restart {
		restartScalar.Value = "true"
	}
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "condition"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: types.ServiceConditionStarted},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "restart"},
			restartScalar,
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "required"},
			{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"},
		},
	}
}

// normalizeVolumeTargets cleans every `services.*.volumes.*.target`
// path with path.Clean so the canonical form matches what v2 produced.
func normalizeVolumeTargets(svc *yaml.Node) {
	volumes := mappingValueByKey(svc, "volumes")
	if volumes == nil || volumes.Kind != yaml.SequenceNode {
		return
	}
	for _, item := range volumes.Content {
		if item == nil || item.Kind != yaml.MappingNode {
			continue
		}
		target := mappingValueByKey(item, "target")
		if target == nil || target.Kind != yaml.ScalarNode || target.Value == "" {
			continue
		}
		target.Value = path.Clean(target.Value)
	}
}

// setNameFromKeyNode assigns the implicit `<project>_<key>` name (or
// the bare key for `external: true` entries) to networks / volumes /
// configs / secrets entries that did not declare one explicitly.
func setNameFromKeyNode(root *yaml.Node) {
	projectName := scalarValueByKey(root, "name")
	for _, section := range []string{"networks", "volumes", "configs", "secrets"} {
		topLevel := mappingValueByKey(root, section)
		if topLevel == nil || topLevel.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(topLevel.Content); i += 2 {
			key := topLevel.Content[i]
			resource := topLevel.Content[i+1]
			if resource == nil || resource.Kind != yaml.MappingNode {
				resource = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				topLevel.Content[i+1] = resource
			}
			if mappingValueByKey(resource, "name") != nil {
				continue
			}
			ext := mappingValueByKey(resource, "external")
			if ext != nil && isTrueNode(ext) {
				setMappingValue(resource, "name", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key.Value})
				continue
			}
			setMappingValue(resource, "name", &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: fmt.Sprintf("%s_%s", projectName, key.Value),
			})
		}
	}
}

func scalarValueByKey(n *yaml.Node, key string) string {
	v := mappingValueByKey(n, key)
	if v == nil || v.Kind != yaml.ScalarNode {
		return ""
	}
	return v.Value
}

func isTrueNode(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.MappingNode {
		// `external: { name: ... }` shorthand is treated as truthy.
		return true
	}
	if n.Kind != yaml.ScalarNode {
		return false
	}
	parsed, _ := strconv.ParseBool(n.Value)
	return parsed
}

// mappingFieldNode returns the value node for key in n, or nil. Unlike
// mappingValueByKey, the function returns nil even for null values so
// callers can distinguish "key absent" from "key present with null".
func mappingFieldNode(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}
