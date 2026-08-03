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
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/compose-spec/compose-go/v2/tree"
)

// Merge applies overrides to a config model
func Merge(right, left map[string]any) (map[string]any, error) {
	return MergeWithPositionalPaths(right, left, nil)
}

// MergeWithPositionalPaths is like Merge but performs a positional (element by
// element) merge for the sequences located at the given paths, instead of the
// default append. These paths are collected by the loader from sequences tagged
// with `!merge` in an override file. See mergePositional for the semantics.
func MergeWithPositionalPaths(right, left map[string]any, positional []tree.Path) (map[string]any, error) {
	merged, err := mergeYaml(right, left, tree.NewPath(), positional)
	if err != nil {
		return nil, err
	}
	return merged.(map[string]any), nil
}

type merger func(any, any, tree.Path) (any, error)

// mergeSpecials defines the custom rules applied by compose when merging yaml trees
var mergeSpecials = map[tree.Path]merger{}

func init() {
	mergeSpecials["networks.*.ipam.config"] = mergeIPAMConfig
	mergeSpecials["networks.*.labels"] = mergeToSequence
	mergeSpecials["volumes.*.labels"] = mergeToSequence
	mergeSpecials["services.*.annotations"] = mergeToSequence
	mergeSpecials["services.*.build"] = mergeBuild
	mergeSpecials["services.*.build.args"] = mergeToSequence
	mergeSpecials["services.*.build.additional_contexts"] = mergeToSequence
	mergeSpecials["services.*.build.extra_hosts"] = mergeExtraHosts
	mergeSpecials["services.*.build.labels"] = mergeToSequence
	mergeSpecials["services.*.command"] = override
	mergeSpecials["services.*.depends_on"] = mergeDependsOn
	mergeSpecials["services.*.deploy.labels"] = mergeToSequence
	mergeSpecials["services.*.dns"] = mergeToSequence
	mergeSpecials["services.*.dns_opt"] = mergeToSequence
	mergeSpecials["services.*.dns_search"] = mergeToSequence
	mergeSpecials["services.*.entrypoint"] = override
	mergeSpecials["services.*.env_file"] = mergeToSequence
	mergeSpecials["services.*.label_file"] = mergeToSequence
	mergeSpecials["services.*.environment"] = mergeToSequence
	mergeSpecials["services.*.extra_hosts"] = mergeExtraHosts
	mergeSpecials["services.*.healthcheck.test"] = override
	mergeSpecials["services.*.labels"] = mergeToSequence
	mergeSpecials["services.*.volumes.*.volume.labels"] = mergeToSequence
	mergeSpecials["services.*.logging"] = mergeLogging
	mergeSpecials["services.*.models"] = mergeModels
	mergeSpecials["services.*.networks"] = mergeNetworks
	mergeSpecials["services.*.sysctls"] = mergeToSequence
	mergeSpecials["services.*.tmpfs"] = mergeToSequence
	mergeSpecials["services.*.ulimits.*"] = mergeUlimit
}

// MergeYaml merges map[string]any yaml trees handling special rules
func MergeYaml(e any, o any, p tree.Path) (any, error) {
	return mergeYaml(e, o, p, nil)
}

// mergeYaml is MergeYaml with the set of positional-merge paths threaded through
// the recursion. A sequence whose path matches one of them is merged element by
// element (see mergePositional) — this takes precedence over both the default
// append and the mergeSpecials rules, since it is an explicit user request.
func mergeYaml(e any, o any, p tree.Path, positional []tree.Path) (any, error) {
	for _, mp := range positional {
		if p.Matches(mp) {
			return mergePositional(e, o, p, positional)
		}
	}
	for pattern, merger := range mergeSpecials {
		if p.Matches(pattern) {
			merged, err := merger(e, o, p)
			if err != nil {
				return nil, err
			}
			return merged, nil
		}
	}
	if o == nil {
		return e, nil
	}
	switch value := e.(type) {
	case map[string]any:
		other, ok := o.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot override %s", p)
		}
		return mergeMappings(value, other, p, positional)
	case []any:
		other, ok := o.([]any)
		if !ok {
			return nil, fmt.Errorf("cannot override %s", p)
		}
		return append(value, other...), nil
	default:
		return o, nil
	}
}

// mergePositional merges two sequences element by element, by index, instead of
// appending. It backs the `!merge` tag: an override can align its sequence with
// the base and touch a single element while leaving the others untouched with a
// no-op entry (`- {}` or `- null`). Semantics per index i:
//
//   - override shorter than base at i: keep base[i]
//   - base shorter than override at i: take override[i] (the sequence is extended)
//   - override[i] is a no-op (nil / empty map / empty seq): keep base[i]
//   - otherwise: recursively merge base[i] with override[i] (deep-merge for
//     mappings, replace for scalars)
func mergePositional(e any, o any, p tree.Path, positional []tree.Path) (any, error) {
	base, ok1 := e.([]any)
	over, ok2 := o.([]any)
	if !ok1 || !ok2 {
		// The loader only records the tag for sequences, so this is defensive:
		// fall back to a plain override rather than failing.
		return o, nil
	}
	n := max(len(base), len(over))
	out := make([]any, 0, n)
	for i := range n {
		switch {
		case i >= len(over):
			out = append(out, base[i])
		case i >= len(base):
			out = append(out, over[i])
		case isNoOp(over[i]):
			out = append(out, base[i])
		default:
			merged, err := mergeYaml(base[i], over[i], p.Next(strconv.Itoa(i)), positional)
			if err != nil {
				return nil, err
			}
			out = append(out, merged)
		}
	}
	return out, nil
}

// isNoOp reports whether a positional override element must leave the base
// element untouched: an explicit null or an empty container (`- {}`).
func isNoOp(x any) bool {
	switch v := x.(type) {
	case nil:
		return true
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	}
	return false
}

func mergeMappings(mapping map[string]any, other map[string]any, p tree.Path, positional []tree.Path) (map[string]any, error) {
	for k, v := range other {
		e, ok := mapping[k]
		if !ok {
			mapping[k] = v
			continue
		}
		next := p.Next(k)
		merged, err := mergeYaml(e, v, next, positional)
		if err != nil {
			return nil, err
		}
		mapping[k] = merged
	}
	return mapping, nil
}

// logging driver options are merged only when both compose file define the same driver
func mergeLogging(c any, o any, p tree.Path) (any, error) {
	config := c.(map[string]any)
	other := o.(map[string]any)
	// we override logging config if source and override have the same driver set, or none
	d, ok1 := other["driver"]
	o, ok2 := config["driver"]
	if d == o || !ok1 || !ok2 {
		return mergeMappings(config, other, p, nil)
	}
	return other, nil
}

func mergeBuild(c any, o any, path tree.Path) (any, error) {
	toBuild := func(c any) map[string]any {
		switch v := c.(type) {
		case string:
			return map[string]any{
				"context": v,
			}
		case map[string]any:
			return v
		}
		return nil
	}
	return mergeMappings(toBuild(c), toBuild(o), path, nil)
}

func mergeDependsOn(c any, o any, path tree.Path) (any, error) {
	right := convertIntoMapping(c, map[string]any{
		"condition": "service_started",
		"required":  true,
	})
	left := convertIntoMapping(o, map[string]any{
		"condition": "service_started",
		"required":  true,
	})
	return mergeMappings(right, left, path, nil)
}

func mergeModels(c any, o any, path tree.Path) (any, error) {
	right := convertIntoMapping(c, nil)
	left := convertIntoMapping(o, nil)
	return mergeMappings(right, left, path, nil)
}

func mergeNetworks(c any, o any, path tree.Path) (any, error) {
	right := convertIntoMapping(c, nil)
	left := convertIntoMapping(o, nil)
	return mergeMappings(right, left, path, nil)
}

func mergeExtraHosts(c any, o any, _ tree.Path) (any, error) {
	right := convertIntoSequence(c)
	left := convertIntoSequence(o)
	// Rewrite content of left slice to remove duplicate elements
	i := 0
	for _, v := range left {
		if !slices.Contains(right, v) {
			left[i] = v
			i++
		}
	}
	// keep only not duplicated elements from left slice
	left = left[:i]
	return append(right, left...), nil
}

func mergeToSequence(c any, o any, _ tree.Path) (any, error) {
	right := convertIntoSequence(c)
	left := convertIntoSequence(o)
	return append(right, left...), nil
}

func convertIntoSequence(value any) []any {
	switch v := value.(type) {
	case map[string]any:
		var seq []any
		for k, val := range v {
			if val == nil {
				seq = append(seq, k)
			} else {
				switch vl := val.(type) {
				// if val is an array we need to add the key with each value one by one
				case []any:
					for _, vlv := range vl {
						seq = append(seq, fmt.Sprintf("%s=%v", k, vlv))
					}
				default:
					seq = append(seq, fmt.Sprintf("%s=%v", k, val))
				}
			}
		}
		slices.SortFunc(seq, func(a, b any) int {
			return cmp.Compare(a.(string), b.(string))
		})
		return seq
	case []any:
		return v
	case string:
		return []any{v}
	}
	return nil
}

func mergeUlimit(_ any, o any, p tree.Path) (any, error) {
	over, ismapping := o.(map[string]any)
	if base, ok := o.(map[string]any); ok && ismapping {
		return mergeMappings(base, over, p, nil)
	}
	return o, nil
}

func mergeIPAMConfig(c any, o any, path tree.Path) (any, error) {
	var ipamConfigs []any
	configs, ok := c.([]any)
	if !ok {
		return o, fmt.Errorf("%s: unexpected type %T", path, c)
	}
	overrides, ok := o.([]any)
	if !ok {
		return o, fmt.Errorf("%s: unexpected type %T", path, c)
	}
	for _, original := range configs {
		right := convertIntoMapping(original, nil)
		for _, override := range overrides {
			left := convertIntoMapping(override, nil)
			if left["subnet"] != right["subnet"] {
				// check if left is already in ipamConfigs, add it if not and continue with the next config
				if !slices.ContainsFunc(ipamConfigs, func(a any) bool {
					return a.(map[string]any)["subnet"] == left["subnet"]
				}) {
					ipamConfigs = append(ipamConfigs, left)
					continue
				}
			}
			merged, err := mergeMappings(right, left, path, nil)
			if err != nil {
				return nil, err
			}
			// find index of potential previous config with the same subnet in ipamConfigs
			indexIfExist := slices.IndexFunc(ipamConfigs, func(a any) bool {
				return a.(map[string]any)["subnet"] == merged["subnet"]
			})
			// if a previous config is already in ipamConfigs, replace it
			if indexIfExist >= 0 {
				ipamConfigs[indexIfExist] = merged
			} else {
				// or add the new config to ipamConfigs
				ipamConfigs = append(ipamConfigs, merged)
			}
		}
	}
	return ipamConfigs, nil
}

func convertIntoMapping(a any, defaultValue map[string]any) map[string]any {
	switch v := a.(type) {
	case map[string]any:
		return v
	case []any:
		converted := map[string]any{}
		for _, s := range v {
			if defaultValue == nil {
				converted[s.(string)] = nil
			} else {
				// Create a new map for each key
				converted[s.(string)] = copyMap(defaultValue)
			}
		}
		return converted
	}
	return nil
}

func copyMap(m map[string]any) map[string]any {
	c := make(map[string]any)
	for k, v := range m {
		c[k] = v
	}
	return c
}

func override(_ any, other any, _ tree.Path) (any, error) {
	return other, nil
}
