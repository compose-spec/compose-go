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

	"github.com/compose-spec/compose-go/v2/override"
	"github.com/compose-spec/compose-go/v2/types"
)

// Normalize compose project by moving deprecated attributes to their canonical position and injecting implicit defaults
func Normalize(dict map[string]any, env types.Mapping) (map[string]any, error) {
	normalizeNetworks(dict)

	lookup := func(s string) (string, bool) {
		v, ok := env[s]
		return v, ok
	}

	for _, key := range []string{"services", "jobs"} {
		d, ok := dict[key]
		if !ok {
			continue
		}
		containers := d.(map[string]any)
		for name, s := range containers {
			container := s.(map[string]any)
			if err := normalizeContainer(container, lookup); err != nil {
				return nil, err
			}
			containers[name] = container
		}

		dict[key] = containers
	}
	setNameFromKey(dict)

	return dict, nil
}

// normalizeContainer normalizes a service or job definition in place
func normalizeContainer(container map[string]any, lookup func(string) (string, bool)) error {
	if container["pull_policy"] == types.PullPolicyIfNotPresent {
		container["pull_policy"] = types.PullPolicyMissing
	}

	if b, ok := container["build"]; ok {
		normalizeBuild(b.(map[string]any), lookup)
	}

	if e, ok := container["environment"]; ok {
		container["environment"], _ = resolve(e, lookup, true)
	}

	dependsOn := map[string]any{}
	if d, ok := container["depends_on"]; ok {
		dependsOn = d.(map[string]any)
	}
	addLinksDependencies(container, dependsOn)
	addNamespaceDependencies(container, dependsOn)
	cleanVolumeTargets(container)
	addVolumesFromDependencies(container, dependsOn)
	if len(dependsOn) > 0 {
		container["depends_on"] = dependsOn
	}

	return resolvePreStartHooks(container)
}

// normalizeBuild injects the implicit build context and dockerfile, and
// resolves build args declared without a value from the environment
func normalizeBuild(build map[string]any, lookup func(string) (string, bool)) {
	if build["context"] == nil {
		build["context"] = "."
	}
	if build["dockerfile"] == nil && build["dockerfile_inline"] == nil {
		build["dockerfile"] = "Dockerfile"
	}

	if a, ok := build["args"]; ok {
		build["args"], _ = resolve(a, lookup, false)
	}
}

// implicitDependency builds the depends_on entry for a dependency the user
// did not declare but which the service definition implies
func implicitDependency(restart bool) map[string]any {
	return map[string]any{
		"condition": types.ServiceConditionStarted,
		"restart":   restart,
		"required":  true,
	}
}

// addLinksDependencies makes each linked service a dependency, unless
// declared as such already
func addLinksDependencies(container map[string]any, dependsOn map[string]any) {
	l, ok := container["links"]
	if !ok {
		return
	}
	for _, e := range l.([]any) {
		link := e.(string)
		parts := strings.Split(link, ":")
		if len(parts) == 2 {
			link = parts[0]
		}
		if _, ok := dependsOn[link]; !ok {
			dependsOn[link] = implicitDependency(true)
		}
	}
}

// addNamespaceDependencies makes a service whose network, ipc, pid, uts or
// cgroup namespace is shared with another service depend on it, unless
// declared as such already
func addNamespaceDependencies(container map[string]any, dependsOn map[string]any) {
	for _, namespace := range []string{"network_mode", "ipc", "pid", "uts", "cgroup"} {
		n, ok := container[namespace]
		if !ok {
			continue
		}
		ref := n.(string)
		if !strings.HasPrefix(ref, types.ServicePrefix) {
			continue
		}
		shared := ref[len(types.ServicePrefix):]
		if _, ok := dependsOn[shared]; !ok {
			dependsOn[shared] = implicitDependency(true)
		}
	}
}

// cleanVolumeTargets lexically cleans the container path of volumes
func cleanVolumeTargets(container map[string]any) {
	v, ok := container["volumes"]
	if !ok {
		return
	}
	volumes := v.([]any)
	for i, volume := range volumes {
		vol := volume.(map[string]any)
		target := vol["target"].(string)
		vol["target"] = path.Clean(target)
		volumes[i] = vol
	}
	container["volumes"] = volumes
}

// addVolumesFromDependencies makes a service mounting the volumes of another
// service depend on it (without restarting it on update), unless declared as
// such already. Volumes taken from an external container are not a service
// dependency.
func addVolumesFromDependencies(container map[string]any, dependsOn map[string]any) {
	n, ok := container["volumes_from"]
	if !ok {
		return
	}
	for _, v := range n.([]any) {
		vol := v.(string)
		if strings.HasPrefix(vol, types.ContainerPrefix) {
			continue
		}
		spec := strings.Split(vol, ":")
		if _, ok := dependsOn[spec[0]]; !ok {
			dependsOn[spec[0]] = implicitDependency(false)
		}
	}
}

// resolvePreStartHooks materializes pre_start inheritance at load time: each
// hook's container specification is completed with the service's own
// container-spec attributes, per the compose merge rules — the hook's
// declarations win on conflicts, and accumulated entries strictly identical
// to an inherited one are not duplicated, which keeps this resolution
// idempotent when an already-resolved model (a `config` output) is loaded
// again. This subsumes the image inheritance of compose-spec#647.
//
// Volumes are deliberately not inherited: mounts inherit at runtime through
// volumes_from, the only mechanism able to share the parent's anonymous and
// image volumes; the hook keeps only its own volume declarations.
func resolvePreStartHooks(service map[string]any) error {
	hooks, ok := service["pre_start"].([]any)
	if !ok || len(hooks) == 0 {
		return nil
	}
	base := map[string]any{}
	for k, v := range service {
		if k == "volumes" || !containerSpecKeys[k] {
			continue
		}
		base[k] = v
	}
	for i, h := range hooks {
		hook, ok := h.(map[string]any)
		if !ok {
			continue
		}
		// wrapped as a service entry so the per-attribute merge rules
		// registered under services.* apply to the hook's specification
		merged, err := override.Merge(
			map[string]any{"services": map[string]any{"hook": deepClone(base)}},
			map[string]any{"services": map[string]any{"hook": hook}},
		)
		if err != nil {
			return fmt.Errorf("resolving pre_start hook specification: %w", err)
		}
		hooks[i] = merged["services"].(map[string]any)["hook"]
	}
	return nil
}

// containerSpecKeys indexes types.ContainerSpecAttributes (generated at
// build time from the ContainerSpec type, so the boundary cannot drift from
// the model). The filter matters for the dict form of the model
// (LoadModel/ToModel consumers): schema validation runs before normalization
// and the typed decode drops unknown keys, so nothing else prevents workload
// or service-only attributes (ports, build, depends_on, x-*) from leaking
// into the rendered hooks.
var containerSpecKeys = func() map[string]bool {
	keys := make(map[string]bool, len(types.ContainerSpecAttributes))
	for _, k := range types.ContainerSpecAttributes {
		keys[k] = true
	}
	return keys
}()

func normalizeNetworks(dict map[string]any) {
	var networks map[string]any
	if n, ok := dict["networks"]; ok {
		networks = n.(map[string]any)
	} else {
		networks = map[string]any{}
	}

	// implicit `default` network must be introduced only if actually used by some service
	usesDefaultNetwork := false

	for _, key := range []string{"services", "jobs"} {
		s, ok := dict[key]
		if !ok {
			continue
		}
		containers := s.(map[string]any)
		for name, se := range containers {
			container := se.(map[string]any)
			if _, ok := container["provider"]; ok {
				continue
			}
			if _, ok := container["network_mode"]; ok {
				continue
			}
			if n, ok := container["networks"]; !ok {
				// If none explicitly declared, container is connected to default network
				container["networks"] = map[string]any{"default": nil}
				usesDefaultNetwork = true
			} else {
				net := n.(map[string]any)
				if len(net) == 0 {
					// networks section declared but empty (corner case)
					container["networks"] = map[string]any{"default": nil}
					usesDefaultNetwork = true
				} else if _, ok := net["default"]; ok {
					usesDefaultNetwork = true
				}
			}
			containers[name] = container
		}
		dict[key] = containers
	}

	if _, ok := networks["default"]; !ok && usesDefaultNetwork {
		// If not declared explicitly, Compose model involves an implicit "default" network
		networks["default"] = nil
	}

	if len(networks) > 0 {
		dict["networks"] = networks
	}
}

func resolve(a any, fn func(s string) (string, bool), keepEmpty bool) (any, bool) {
	switch v := a.(type) {
	case []any:
		var resolved []any
		for _, val := range v {
			if r, ok := resolve(val, fn, keepEmpty); ok {
				resolved = append(resolved, r)
			}
		}
		return resolved, true
	case map[string]any:
		resolved := map[string]any{}
		for key, val := range v {
			if val != nil {
				resolved[key] = val
				continue
			}
			if s, ok := fn(key); ok {
				resolved[key] = s
			} else if keepEmpty {
				resolved[key] = nil
			}
		}
		return resolved, true
	case string:
		if !strings.Contains(v, "=") {
			if val, ok := fn(v); ok {
				return fmt.Sprintf("%s=%s", v, val), true
			}
			if keepEmpty {
				return v, true
			}
			return "", false
		}
		return v, true
	default:
		return v, false
	}
}

// Resources with no explicit name are actually named by their key in map
func setNameFromKey(dict map[string]any) {
	for _, r := range []string{"networks", "volumes", "configs", "secrets"} {
		a, ok := dict[r]
		if !ok {
			continue
		}
		toplevel := a.(map[string]any)
		for key, r := range toplevel {
			var resource map[string]any
			if r != nil {
				resource = r.(map[string]any)
			} else {
				resource = map[string]any{}
			}
			if resource["name"] == nil {
				if x, ok := resource["external"]; ok && isTrue(x) {
					resource["name"] = key
				} else {
					resource["name"] = fmt.Sprintf("%s_%s", dict["name"], key)
				}
			}
			toplevel[key] = resource
		}
	}
}

func isTrue(x any) bool {
	parseBool, _ := strconv.ParseBool(fmt.Sprint(x))
	return parseBool
}
