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

package node

import (
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
)

// DefaultMaxNodeVisits caps the total number of node visits performed by
// ResolveResetOverride per document. The value is sized to accommodate large
// real-world compose files while still rejecting documents that would cause
// unbounded traversal (alias bombs). Callers can override it by passing a
// non-zero limit to ResolveResetOverride.
const DefaultMaxNodeVisits = 100_000

// nodeCache stores a resolved node and the relative sub-paths within its
// subtree that carried !reset/!override tags, so cache hits at different call
// sites can replay them rather than re-traversing the shared subtree.
type nodeCache struct {
	node          *yaml.Node
	relativePaths []tree.Path
}

// ResolveResetOverride detects !reset and !override tags inside a yaml.Node
// tree and produces a cleaned tree together with the list of paths where one
// of those tags was found.
//
//   - Nodes tagged !reset are removed from cleaned (their value contributes
//     nothing to this layer) but their path is recorded so the merge phase
//     can also drop any value contributed at the same path by a base layer.
//   - Nodes tagged !override are kept in cleaned with their value; their path
//     is recorded so the merge phase replaces (rather than merges with) any
//     value from a base layer at that path.
//
// maxNodeVisits caps the total number of recursive resolution calls; pass 0
// to use DefaultMaxNodeVisits. Exceeding the cap returns an error rather than
// silently truncating, which is the v2 alias-bomb defense.
//
// Aliases are followed once per call site through an internal cache, so a
// shared anchor used at multiple sites is traversed only once and the
// recorded !reset/!override paths are replayed at each subsequent site.
func ResolveResetOverride(root *yaml.Node, maxNodeVisits int) (*yaml.Node, []tree.Path, error) {
	if maxNodeVisits <= 0 {
		maxNodeVisits = DefaultMaxNodeVisits
	}
	r := &resolver{
		visitedNodes:  make(map[*yaml.Node][]string),
		resolvedNodes: make(map[*yaml.Node]nodeCache),
		maxNodeVisits: maxNodeVisits,
	}
	// A DocumentNode is a transparent wrapper around the actual root; unwrap
	// it so callers that pass the result of yaml.Unmarshal directly get the
	// same behavior as the v2 path, where yaml.Decoder hands the inner node
	// to UnmarshalYAML.
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	resolved, err := r.resolveReset(target, tree.NewPath())
	if err != nil {
		return nil, nil, err
	}
	return resolved, r.paths, nil
}

type resolver struct {
	paths         []tree.Path
	visitedNodes  map[*yaml.Node][]string
	resolvedNodes map[*yaml.Node]nodeCache
	visitCount    int
	maxNodeVisits int
}

func (r *resolver) resolveReset(node *yaml.Node, path tree.Path) (*yaml.Node, error) {
	r.visitCount++
	if r.visitCount > r.maxNodeVisits {
		return nil, fmt.Errorf("compose file exceeds maximum node visit limit (%d)", r.maxNodeVisits)
	}

	pathStr := path.String()
	// A merge key (`<<`) appears as a synthetic ".<<" segment in the path; the
	// recorded path must elide it so downstream consumers can match it against
	// the user-visible structure.
	if strings.Contains(pathStr, ".<<") {
		path = tree.NewPath(strings.Replace(pathStr, ".<<", "", 1))
	}

	if node.Tag == "!reset" {
		r.paths = append(r.paths, path)
		return nil, nil
	}
	if node.Tag == "!override" {
		r.paths = append(r.paths, path)
		return node, nil
	}

	if node.Kind == yaml.AliasNode {
		if err := r.checkForCycle(node.Alias, path); err != nil {
			return nil, err
		}
		target := node.Alias
		if target.Tag == "!reset" {
			r.paths = append(r.paths, path)
			return nil, nil
		}
		if target.Tag == "!override" {
			r.paths = append(r.paths, path)
			return target, nil
		}
		return r.cachedResolve(target, path)
	}

	if node.Kind == yaml.SequenceNode || node.Kind == yaml.MappingNode {
		return r.cachedResolve(node, path)
	}

	return node, nil
}

// cachedResolve resolves a container node (Sequence or Mapping), serving from
// cache on repeat visits so a shared anchor is only traversed once. The cache
// is keyed by the original node pointer; on a cache hit, the recorded
// relative paths are replayed under the current base path.
func (r *resolver) cachedResolve(node *yaml.Node, path tree.Path) (*yaml.Node, error) {
	if cached, ok := r.resolvedNodes[node]; ok {
		for _, rel := range cached.relativePaths {
			r.paths = append(r.paths, joinPath(path, rel))
		}
		return cached.node, nil
	}

	startIdx := len(r.paths)
	resolved, err := r.resolveContainer(node, path)
	if err != nil {
		return nil, err
	}

	var relPaths []tree.Path
	for _, addedPath := range r.paths[startIdx:] {
		rel, err := subPath(addedPath, path)
		if err != nil {
			return nil, err
		}
		relPaths = append(relPaths, rel)
	}
	r.resolvedNodes[node] = nodeCache{node: resolved, relativePaths: relPaths}
	return resolved, nil
}

// resolveContainer recurses into a Sequence or Mapping node's children.
// AliasNodes are preserved as-is in the output Content so the YAML library
// can resolve them at decode time; only the resolved value is consulted for
// tag inspection. Mapping keys are checked for duplicates and the error
// carries the offending line numbers for diagnostics.
func (r *resolver) resolveContainer(node *yaml.Node, path tree.Path) (*yaml.Node, error) {
	switch node.Kind {
	case yaml.SequenceNode:
		var nodes []*yaml.Node
		for idx, v := range node.Content {
			next := path.Next(strconv.Itoa(idx))
			resolved, err := r.resolveReset(v, next)
			if err != nil {
				return nil, err
			}
			if resolved == nil {
				continue
			}
			if v.Kind == yaml.AliasNode {
				nodes = append(nodes, v)
			} else {
				nodes = append(nodes, resolved)
			}
		}
		node.Content = nodes
	case yaml.MappingNode:
		keys := map[string]int{}
		var key string
		var nodes []*yaml.Node
		for idx, v := range node.Content {
			if idx%2 == 0 {
				key = v.Value
				if line, seen := keys[key]; seen {
					return nil, fmt.Errorf("line %d: mapping key %#v already defined at line %d", v.Line, key, line)
				}
				keys[key] = v.Line
			} else {
				resolved, err := r.resolveReset(v, path.Next(key))
				if err != nil {
					return nil, err
				}
				if resolved == nil {
					continue
				}
				if v.Kind == yaml.AliasNode {
					nodes = append(nodes, node.Content[idx-1], v)
				} else {
					nodes = append(nodes, node.Content[idx-1], resolved)
				}
			}
		}
		node.Content = nodes
	}
	return node, nil
}

func (r *resolver) checkForCycle(node *yaml.Node, path tree.Path) error {
	paths := r.visitedNodes[node]
	pathStr := path.String()

	for _, prevPath := range paths {
		if pathStr == prevPath {
			continue
		}
		// Merge keys (`<<`) are legitimate YAML merging, not a cycle.
		if strings.Contains(prevPath, "<<") || strings.Contains(pathStr, "<<") {
			continue
		}
		// Only consider it a cycle if one path is contained within the other
		// and they're not in different service definitions.
		if (strings.HasPrefix(pathStr, prevPath+".") ||
			strings.HasPrefix(prevPath, pathStr+".")) &&
			!areInDifferentServices(pathStr, prevPath) {
			return fmt.Errorf("cycle detected: node at path %s references node at path %s", pathStr, prevPath)
		}
	}

	r.visitedNodes[node] = append(paths, pathStr)
	return nil
}

// areInDifferentServices returns true when both paths traverse the `services`
// top-level key but land on different service names. A shared anchor used by
// two different services is not a cycle, even if both paths share a common
// prefix below the service name.
func areInDifferentServices(path1, path2 string) bool {
	parts1 := strings.Split(path1, ".")
	parts2 := strings.Split(path2, ".")
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] == "services" && i+1 < len(parts1) &&
			parts2[i] == "services" && i+1 < len(parts2) {
			return parts1[i+1] != parts2[i+1]
		}
	}
	return false
}

// subPath strips base from full to produce a relative path stored in the
// cache. Returns "" when full == base (the !reset/!override tag is on the
// node root itself). Returns an error when full is not rooted at base, which
// would indicate a logic error in resolveReset/cachedResolve.
func subPath(full, base tree.Path) (tree.Path, error) {
	if base == "" {
		return full, nil
	}
	fullStr := string(full)
	baseStr := string(base)
	if fullStr == baseStr {
		return "", nil
	}
	prefix := baseStr + "."
	if strings.HasPrefix(fullStr, prefix) {
		return tree.Path(fullStr[len(prefix):]), nil
	}
	return "", fmt.Errorf("internal error: path %q is not a sub-path of %q", fullStr, baseStr)
}

// joinPath reconstructs an absolute path from a call-site base and a cached
// relative path. A relative path of "" means the tag was on the node root, so
// base is returned unchanged.
func joinPath(base, rel tree.Path) tree.Path {
	if rel == "" {
		return base
	}
	if base == "" {
		return rel
	}
	return tree.Path(string(base) + "." + string(rel))
}
