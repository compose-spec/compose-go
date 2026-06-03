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
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
)

// MergeNode merges two yaml.Node trees using the same per-path override /
// append rules as MergeYaml, but without ever round-tripping through
// map[string]any. The left tree (override) is folded into the right tree
// (base): for mappings, the left keys overwrite or recurse into the matching
// right keys; for sequences, the default behavior is append; for scalars,
// left wins. Paths declared in mergeSpecialsNode override the default
// behavior with a per-key strategy (append-merged, deduplicated, etc.).
//
// The returned node is the right tree, mutated in place. Callers must not
// rely on a particular ordering of keys in mappings; insertion order is the
// existing keys from the right tree followed by any new keys from the left.
//
// MergeNode expects both inputs to have had their aliases unfolded
// beforehand (see node.NormalizeAliases). It does not follow AliasNode
// values.
func MergeNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	right = unwrapDocumentNode(right)
	left = unwrapDocumentNode(left)
	if left == nil {
		return right, nil
	}
	if right == nil {
		return left, nil
	}

	for pattern, merger := range mergeSpecialsNode {
		if p.Matches(pattern) {
			return merger(right, left, p)
		}
	}

	switch right.Kind {
	case yaml.MappingNode:
		if left.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("cannot override %s", p)
		}
		return mergeMappingsNode(right, left, p)
	case yaml.SequenceNode:
		if left.Kind != yaml.SequenceNode {
			return nil, fmt.Errorf("cannot override %s", p)
		}
		right.Content = append(right.Content, left.Content...)
		return right, nil
	default:
		return left, nil
	}
}

type nodeMerger func(*yaml.Node, *yaml.Node, tree.Path) (*yaml.Node, error)

// mergeSpecialsNode mirrors mergeSpecials but operates on *yaml.Node. The
// entries are kept in sync between the two maps; the v2 map disappears when
// the legacy map[string]any path is removed.
var mergeSpecialsNode = map[tree.Path]nodeMerger{}

func init() {
	mergeSpecialsNode["networks.*.ipam.config"] = mergeIPAMConfigNode
	mergeSpecialsNode["networks.*.labels"] = mergeToSequenceNode
	mergeSpecialsNode["volumes.*.labels"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.annotations"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.build"] = mergeBuildNode
	mergeSpecialsNode["services.*.build.args"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.build.additional_contexts"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.build.extra_hosts"] = mergeExtraHostsNode
	mergeSpecialsNode["services.*.build.labels"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.command"] = overrideNode
	mergeSpecialsNode["services.*.depends_on"] = mergeDependsOnNode
	mergeSpecialsNode["services.*.deploy.labels"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.dns"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.dns_opt"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.dns_search"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.entrypoint"] = overrideNode
	mergeSpecialsNode["services.*.env_file"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.label_file"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.environment"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.extra_hosts"] = mergeExtraHostsNode
	mergeSpecialsNode["services.*.healthcheck.test"] = overrideNode
	mergeSpecialsNode["services.*.labels"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.volumes.*.volume.labels"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.logging"] = mergeLoggingNode
	mergeSpecialsNode["services.*.models"] = mergeModelsNode
	mergeSpecialsNode["services.*.networks"] = mergeNetworksNode
	mergeSpecialsNode["services.*.sysctls"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.tmpfs"] = mergeToSequenceNode
	mergeSpecialsNode["services.*.ulimits.*"] = mergeUlimitNode
}

// mergeMappingsNode folds the left mapping into the right mapping. For each
// (key, value) of left:
//   - if right has no entry for key, the (key, value) pair is appended;
//   - otherwise, MergeNode is invoked recursively at the next path,
//     and the result replaces right's value for that key.
//
// The order of right's existing keys is preserved; new keys from left are
// appended at the end.
func mergeMappingsNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	keyIdx := mappingKeyIndex(right)
	for i := 0; i+1 < len(left.Content); i += 2 {
		key := left.Content[i]
		value := left.Content[i+1]
		if idx, ok := keyIdx[key.Value]; ok {
			merged, err := MergeNode(right.Content[idx+1], value, p.Next(key.Value))
			if err != nil {
				return nil, err
			}
			right.Content[idx+1] = merged
			continue
		}
		right.Content = append(right.Content, key, value)
		keyIdx[key.Value] = len(right.Content) - 2
	}
	return right, nil
}

// mappingKeyIndex returns a map from each key's Value to the index of the key
// node within n.Content. Index i means the value node is at Content[i+1].
func mappingKeyIndex(n *yaml.Node) map[string]int {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	idx := make(map[string]int, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		idx[n.Content[i].Value] = i
	}
	return idx
}

// nodeMapGet returns the value Node for key in a MappingNode, or nil when
// the key is absent.
func nodeMapGet(n *yaml.Node, key string) *yaml.Node {
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

// unwrapDocumentNode peels off a single DocumentNode wrapper, returning the
// inner content. Useful when a Node was produced by yaml.Unmarshal directly
// rather than by a sub-decode.
func unwrapDocumentNode(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		return n.Content[0]
	}
	return n
}

// overrideNode is the merger for paths where the left value replaces the
// right value wholesale (services.*.command, .entrypoint, .healthcheck.test).
func overrideNode(_, left *yaml.Node, _ tree.Path) (*yaml.Node, error) {
	return left, nil
}

// mergeLoggingNode merges logging blocks only when both files declare the
// same driver (or one of them omits it). When the drivers differ, the left
// block replaces the right block entirely — option keys are driver-specific
// and merging them would be meaningless.
func mergeLoggingNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	rDriver := scalarValue(nodeMapGet(right, "driver"))
	lDriver := scalarValue(nodeMapGet(left, "driver"))
	rHas := nodeMapGet(right, "driver") != nil
	lHas := nodeMapGet(left, "driver") != nil
	if rDriver == lDriver || !rHas || !lHas {
		return mergeMappingsNode(right, left, p)
	}
	return left, nil
}

func scalarValue(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// mergeBuildNode promotes the short form (a single scalar = context path)
// into the canonical mapping {context: <scalar>} before merging.
func mergeBuildNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	return mergeMappingsNode(promoteBuildNode(right), promoteBuildNode(left), p)
}

func promoteBuildNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		return &yaml.Node{
			Kind:   yaml.MappingNode,
			Tag:    "!!map",
			Line:   n.Line,
			Column: n.Column,
			Content: []*yaml.Node{
				stringScalarAt("context", n.Line, n.Column),
				n,
			},
		}
	}
	return n
}

// mergeDependsOnNode normalizes both inputs into the canonical mapping form
// before merging. The short form (list of service names) is expanded to
// {<name>: {condition: service_started, required: true}}.
func mergeDependsOnNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	defaults := func() *yaml.Node {
		return &yaml.Node{
			Kind: yaml.MappingNode,
			Tag:  "!!map",
			Content: []*yaml.Node{
				stringScalar("condition"), stringScalar("service_started"),
				stringScalar("required"), boolScalar(true),
			},
		}
	}
	return mergeMappingsNode(
		convertIntoMappingNode(right, defaults),
		convertIntoMappingNode(left, defaults),
		p,
	)
}

// mergeModelsNode normalizes both inputs into the canonical mapping form
// before merging. The short form (list of model names) maps each name to a
// nil value.
func mergeModelsNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	return mergeMappingsNode(
		convertIntoMappingNode(right, nil),
		convertIntoMappingNode(left, nil),
		p,
	)
}

// mergeNetworksNode mirrors mergeModelsNode: short-form lists are expanded
// into name->nil mappings before merging.
func mergeNetworksNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	return mergeMappingsNode(
		convertIntoMappingNode(right, nil),
		convertIntoMappingNode(left, nil),
		p,
	)
}

// mergeExtraHostsNode appends left into right while filtering out entries
// already present in right, regardless of declaration order. Each entry is
// compared by its serialized form (`hostname=ip` for mapping inputs,
// raw string for sequence inputs).
func mergeExtraHostsNode(right, left *yaml.Node, _ tree.Path) (*yaml.Node, error) {
	r := convertIntoSequenceNode(right)
	l := convertIntoSequenceNode(left)
	seen := map[string]bool{}
	for _, item := range r.Content {
		seen[scalarValue(item)] = true
	}
	for _, item := range l.Content {
		if seen[scalarValue(item)] {
			continue
		}
		seen[scalarValue(item)] = true
		r.Content = append(r.Content, item)
	}
	return r, nil
}

// mergeToSequenceNode is the simple append rule used by env_file, labels,
// volumes, ports, dns, etc. Both sides are normalized to sequence form and
// concatenated; no deduplication is performed at this stage. Unicity is
// enforced later by EnforceUnicityNode where required.
func mergeToSequenceNode(right, left *yaml.Node, _ tree.Path) (*yaml.Node, error) {
	r := convertIntoSequenceNode(right)
	l := convertIntoSequenceNode(left)
	r.Content = append(r.Content, l.Content...)
	return r, nil
}

// mergeUlimitNode merges two ulimit entries: when both are mappings (soft /
// hard form), keys are merged; otherwise the left value replaces the right.
func mergeUlimitNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	if right != nil && right.Kind == yaml.MappingNode && left != nil && left.Kind == yaml.MappingNode {
		return mergeMappingsNode(right, left, p)
	}
	return left, nil
}

// mergeIPAMConfigNode merges two networks.*.ipam.config sequences. Each entry
// is a mapping that may include a `subnet`. Entries with a matching subnet
// are merged together; entries with a unique subnet from either side are
// preserved as-is. The result preserves left's order of newly-introduced
// entries.
func mergeIPAMConfigNode(right, left *yaml.Node, p tree.Path) (*yaml.Node, error) {
	if right.Kind != yaml.SequenceNode || left.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s: unexpected non-sequence value", p)
	}
	result := &yaml.Node{
		Kind:   yaml.SequenceNode,
		Tag:    right.Tag,
		Style:  right.Style,
		Line:   right.Line,
		Column: right.Column,
	}
	for _, original := range right.Content {
		base := convertIntoMappingNode(original, nil)
		matched := false
		for _, override := range left.Content {
			over := convertIntoMappingNode(override, nil)
			if scalarValue(nodeMapGet(over, "subnet")) != scalarValue(nodeMapGet(base, "subnet")) {
				continue
			}
			matched = true
			merged, err := mergeMappingsNode(base, over, p)
			if err != nil {
				return nil, err
			}
			result.Content = append(result.Content, merged)
		}
		if !matched {
			result.Content = append(result.Content, base)
		}
	}
	// Append left-only entries (subnets present in left but absent in right).
	knownSubnets := map[string]bool{}
	for _, entry := range result.Content {
		knownSubnets[scalarValue(nodeMapGet(entry, "subnet"))] = true
	}
	for _, override := range left.Content {
		over := convertIntoMappingNode(override, nil)
		subnet := scalarValue(nodeMapGet(over, "subnet"))
		if knownSubnets[subnet] {
			continue
		}
		result.Content = append(result.Content, over)
	}
	return result, nil
}

// convertIntoMappingNode promotes a sequence of strings into a mapping where
// each string becomes a key. If defaults is non-nil, every new key gets a
// deep copy of the value returned by defaults() (a function so each entry
// gets a distinct copy). If the input is already a mapping, it is returned
// unchanged.
func convertIntoMappingNode(n *yaml.Node, defaults func() *yaml.Node) *yaml.Node {
	if n == nil {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	}
	if n.Kind == yaml.MappingNode {
		return n
	}
	if n.Kind == yaml.SequenceNode {
		m := &yaml.Node{
			Kind:   yaml.MappingNode,
			Tag:    "!!map",
			Line:   n.Line,
			Column: n.Column,
		}
		for _, item := range n.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			key := &yaml.Node{
				Kind:   yaml.ScalarNode,
				Tag:    "!!str",
				Value:  item.Value,
				Line:   item.Line,
				Column: item.Column,
			}
			var value *yaml.Node
			if defaults != nil {
				value = defaults()
			} else {
				value = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Line: item.Line, Column: item.Column}
			}
			m.Content = append(m.Content, key, value)
		}
		return m
	}
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

// convertIntoSequenceNode promotes mappings and scalars into a sequence of
// scalar items. A mapping {key: value} becomes a sequence of "key=value"
// strings (sorted lexicographically to keep merge results deterministic); a
// mapping value that is itself a sequence yields one "key=item" entry per
// item. A bare scalar becomes a one-element sequence.
func convertIntoSequenceNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	}
	switch n.Kind {
	case yaml.SequenceNode:
		return n
	case yaml.MappingNode:
		var values []string
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			value := n.Content[i+1]
			if value == nil || (value.Kind == yaml.ScalarNode && value.Tag == "!!null") {
				values = append(values, key)
				continue
			}
			if value.Kind == yaml.SequenceNode {
				for _, item := range value.Content {
					values = append(values, fmt.Sprintf("%s=%s", key, scalarOrInline(item)))
				}
				continue
			}
			values = append(values, fmt.Sprintf("%s=%s", key, scalarOrInline(value)))
		}
		slices.SortFunc(values, cmp.Compare[string])
		seq := &yaml.Node{
			Kind:   yaml.SequenceNode,
			Tag:    "!!seq",
			Line:   n.Line,
			Column: n.Column,
		}
		for _, v := range values {
			seq.Content = append(seq.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: v,
				Line:  n.Line,
			})
		}
		return seq
	case yaml.ScalarNode:
		return &yaml.Node{
			Kind:    yaml.SequenceNode,
			Tag:     "!!seq",
			Line:    n.Line,
			Column:  n.Column,
			Content: []*yaml.Node{n},
		}
	}
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
}

// scalarOrInline formats a non-scalar value into a single-line string for
// use as part of a "key=value" entry built by convertIntoSequenceNode.
// Scalars are returned verbatim; sequences and mappings are flattened with
// their fields concatenated, which mirrors the v2 behavior of relying on
// fmt.Sprintf("%v", ...) over the decoded interface{}.
func scalarOrInline(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yaml.ScalarNode {
		return n.Value
	}
	var parts []string
	for _, c := range n.Content {
		parts = append(parts, scalarOrInline(c))
	}
	return strings.Join(parts, " ")
}

func stringScalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func stringScalarAt(value string, line, col int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, Line: line, Column: col}
}

func boolScalar(b bool) *yaml.Node {
	v := "false"
	if b {
		v = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}
