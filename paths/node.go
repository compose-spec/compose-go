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

package paths

import (
	"errors"
	"path"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"github.com/compose-spec/compose-go/v3/utils"
)

// NodeResolverOptions configures ResolveRelativePathsNode.
type NodeResolverOptions struct {
	// WorkingDirFor returns the working directory against which a relative
	// path attached to n should be resolved. Letting the caller pick a
	// per-node WorkingDir is what enables the v3 fix where a relative path
	// declared inside an included file is resolved against the include's
	// project_directory rather than the project root.
	//
	// When nil, every resolution uses WorkingDir.
	WorkingDirFor func(n *yaml.Node) string

	// WorkingDir is the fall-back single working directory used when
	// WorkingDirFor is nil. Provided for v2-style callers that have a
	// single project root.
	WorkingDir string

	// Remotes is a list of predicates that flag a path string as a remote
	// URL, exempting it from absolute-path conversion. Mirrors the v2
	// behavior of the same name.
	Remotes []RemoteResource
}

// ResolveRelativePathsNode walks root and converts relative path scalars to
// absolute paths in place, using a per-scalar WorkingDir chosen by
// WorkingDirFor.
//
// The set of recognized paths mirrors the v2 ResolveRelativePaths registry:
// services.*.build.context, services.*.build.additional_contexts.*,
// services.*.build.ssh.*, services.*.env_file.*.path, services.*.label_file.*,
// services.*.extends.file, services.*.develop.watch.*.path, services.*.volumes.*,
// configs.*.file, secrets.*.file, include.path, include.project_directory,
// include.env_file, and volumes.*. The Node version reads Tag information
// directly, so callers do not need to canonicalize beforehand only to
// distinguish short- from long-form values (for example services.*.volumes.*
// inspects the entry's Kind rather than its decoded Go type).
func ResolveRelativePathsNode(root *yaml.Node, opts NodeResolverOptions) error {
	if opts.WorkingDirFor == nil {
		wd := opts.WorkingDir
		opts.WorkingDirFor = func(*yaml.Node) string { return wd }
	}
	r := &nodeResolverState{opts: opts}
	r.resolvers = map[tree.Path]func(*yaml.Node) error{
		"services.*.build.context":               r.absContextScalar,
		"services.*.build.additional_contexts.*": r.absContextScalar,
		"services.*.build.ssh.*":                 r.maybeUnixScalar,
		"services.*.env_file.*.path":             r.absScalar,
		"services.*.label_file.*":                r.absScalar,
		"services.*.extends.file":                r.absExtendsScalar,
		"services.*.develop.watch.*.path":        r.absSymbolicLinkScalar,
		"services.*.volumes.*":                   r.absVolumeMount,
		"configs.*.file":                         r.maybeUnixScalar,
		"secrets.*.file":                         r.maybeUnixScalar,
		"include.path":                           r.absScalarMaybeSequence,
		"include.project_directory":              r.absScalar,
		"include.env_file":                       r.absScalarMaybeSequence,
		"volumes.*":                              r.volumeDriverOpts,
	}
	return r.walk(root, tree.NewPath())
}

type nodeResolverState struct {
	opts      NodeResolverOptions
	resolvers map[tree.Path]func(*yaml.Node) error
}

func (r *nodeResolverState) walk(n *yaml.Node, p tree.Path) error {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.DocumentNode {
		for _, c := range n.Content {
			if err := r.walk(c, p); err != nil {
				return err
			}
		}
		return nil
	}
	for pattern, fn := range r.resolvers {
		if p.Matches(pattern) {
			return fn(n)
		}
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if err := r.walk(n.Content[i+1], p.Next(key)); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if err := r.walk(c, p.Next(tree.PathMatchList)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *nodeResolverState) isRemoteResource(p string) bool {
	for _, remote := range r.opts.Remotes {
		if remote(p) {
			return true
		}
	}
	return false
}

// absScalar resolves a single ScalarNode to an absolute path. A nil / empty
// scalar is left untouched; a non-scalar node is also left untouched (a
// caller that targets a path expecting a scalar but receives a sequence has
// pre-canonicalization shape and is handled by absScalarMaybeSequence).
func (r *nodeResolverState) absScalar(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode || n.Value == "" {
		return nil
	}
	expanded := ExpandUser(n.Value)
	if filepath.IsAbs(expanded) {
		n.Value = expanded
		return nil
	}
	n.Value = filepath.Join(r.opts.WorkingDirFor(n), expanded)
	return nil
}

// absScalarMaybeSequence accepts either a single ScalarNode or a SequenceNode
// of scalars and resolves each. Used for include.path (which may be a single
// path or a list) and include.env_file (same).
func (r *nodeResolverState) absScalarMaybeSequence(n *yaml.Node) error {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.SequenceNode {
		for _, c := range n.Content {
			if err := r.absScalar(c); err != nil {
				return err
			}
		}
		return nil
	}
	return r.absScalar(n)
}

// maybeUnixScalar resolves a path scalar against the working directory,
// unless the value is already an absolute Unix or Windows path. Mirrors
// maybeUnixPath in paths/unix.go.
func (r *nodeResolverState) maybeUnixScalar(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode {
		return nil
	}
	expanded := ExpandUser(n.Value)
	if !path.IsAbs(expanded) && !IsWindowsAbs(expanded) {
		if filepath.IsAbs(expanded) {
			n.Value = expanded
			return nil
		}
		n.Value = filepath.Join(r.opts.WorkingDirFor(n), expanded)
		return nil
	}
	n.Value = expanded
	return nil
}

// absContextScalar handles services.*.build.context: skip URL-like values
// (https://, git://, ssh://, github.com/, git@, custom builder schemes),
// skip ServicePrefix entries, otherwise treat as a path.
func (r *nodeResolverState) absContextScalar(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode {
		return nil
	}
	v := n.Value
	if strings.Contains(v, "://") {
		return nil
	}
	if strings.HasPrefix(v, types.ServicePrefix) {
		return nil
	}
	if isRemoteContext(v) {
		return nil
	}
	return r.absScalar(n)
}

// absExtendsScalar resolves a services.*.extends.file scalar unless it
// matches a registered remote loader.
func (r *nodeResolverState) absExtendsScalar(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode {
		return nil
	}
	if r.isRemoteResource(n.Value) {
		return nil
	}
	return r.absScalar(n)
}

// absSymbolicLinkScalar resolves a path then dereferences it through
// utils.ResolveSymbolicLink. Used by services.*.develop.watch.*.path.
func (r *nodeResolverState) absSymbolicLinkScalar(n *yaml.Node) error {
	if err := r.absScalar(n); err != nil {
		return err
	}
	if n == nil || n.Kind != yaml.ScalarNode {
		return nil
	}
	resolved, err := utils.ResolveSymbolicLink(n.Value)
	if err != nil {
		return err
	}
	n.Value = resolved
	return nil
}

// absVolumeMount handles services.*.volumes.*: when the entry is the
// canonical long form (a mapping with type: bind), resolve the source
// path against the working directory of the scalar. Short-form string
// entries are left untouched and handled by EnforceUnicity later in the
// pipeline.
func (r *nodeResolverState) absVolumeMount(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	mountType := mappingFieldValue(n, "type")
	if mountType != types.VolumeTypeBind {
		return nil
	}
	source := mappingFieldNode(n, "source")
	if source == nil {
		return errors.New(`invalid mount config for type "bind": field Source must not be empty`)
	}
	return r.maybeUnixScalar(source)
}

// volumeDriverOpts handles volumes.*: when the local driver is in use with
// "o: bind", resolve the device path against the working directory. Mirrors
// the v2 relativePathsResolver.volumeDriverOpts.
func (r *nodeResolverState) volumeDriverOpts(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	if mappingFieldValue(n, "driver") != "local" {
		return nil
	}
	opts := mappingFieldNode(n, "driver_opts")
	if opts == nil || opts.Kind != yaml.MappingNode {
		return nil
	}
	if mappingFieldValue(opts, "o") != "bind" {
		return nil
	}
	device := mappingFieldNode(opts, "device")
	if device == nil {
		return nil
	}
	return r.maybeUnixScalar(device)
}

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

func mappingFieldValue(n *yaml.Node, key string) string {
	v := mappingFieldNode(n, key)
	if v == nil || v.Kind != yaml.ScalarNode {
		return ""
	}
	return v.Value
}
