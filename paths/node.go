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

	// ExcludePaths is the optional list of tree.Path patterns to skip
	// during traversal. Resolvers registered at any of these patterns are
	// not invoked, and the walker continues into the children as if no
	// rule were registered. Used by the include pre-resolution to defer
	// extends.file resolution to the orchestrator extends pass, which
	// needs the original relative reference.
	ExcludePaths []string
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
		"services.*.build":                       r.absBuild,
		"services.*.build.context":               r.absContextScalar,
		"services.*.build.additional_contexts.*": r.absContextScalar,
		"services.*.build.ssh.*":                 r.absSSHEntry,
		"services.*.env_file":                    r.absEnvFileShortForm,
		"services.*.env_file.*":                  r.absEnvFile,
		"services.*.env_file.*.path":             r.absScalar,
		"services.*.label_file":                  r.absScalarMaybeSequence,
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
	for _, ex := range opts.ExcludePaths {
		delete(r.resolvers, tree.NewPath(ex))
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
// Scalars tagged !!null are skipped so post-canonical placeholders (e.g.
// the `default: null` entry produced by ssh canonicalization) keep their
// type instead of being rewritten to a path string.
func (r *nodeResolverState) absScalar(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode || n.Value == "" || n.Tag == "!!null" {
		return nil
	}
	expanded := ExpandUser(n.Value)
	if filepath.IsAbs(expanded) {
		n.Value = expanded
		return nil
	}
	wd := r.opts.WorkingDirFor(n)
	if wd == "" {
		return nil
	}
	n.Value = filepath.Join(wd, expanded)
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
// maybeUnixPath in paths/unix.go. Skips !!null scalars and empty values so
// post-canonical null placeholders are not rewritten.
func (r *nodeResolverState) maybeUnixScalar(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode || n.Value == "" || n.Tag == "!!null" {
		return nil
	}
	expanded := ExpandUser(n.Value)
	if !path.IsAbs(expanded) && !IsWindowsAbs(expanded) {
		if filepath.IsAbs(expanded) {
			n.Value = expanded
			return nil
		}
		wd := r.opts.WorkingDirFor(n)
		if wd == "" {
			return nil
		}
		n.Value = filepath.Join(wd, expanded)
		return nil
	}
	n.Value = expanded
	return nil
}

// absBuild handles services.*.build in both canonical short and long form.
// Short form (a scalar Value is the build context path) is treated as a
// context path and resolved against the layer working directory. Long form
// (a mapping with context / additional_contexts / ssh fields) is recursed
// into by walking the mapping's children — this is needed because the
// generic walker stops at the first matching pattern, so it cannot descend
// past services.*.build to reach services.*.build.context on its own.
//
// Running paths before canonicalization avoids the loss of pointer identity
// that the CanonicalNode bridge would otherwise cause; this handler keeps
// both shapes supported until per-transformer Node ports are in place.
func (r *nodeResolverState) absBuild(n *yaml.Node) error {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		return r.absContextScalar(n)
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		val := n.Content[i+1]
		switch key {
		case "context":
			if err := r.absContextScalar(val); err != nil {
				return err
			}
		case "additional_contexts":
			if val.Kind == yaml.MappingNode {
				for j := 1; j < len(val.Content); j += 2 {
					if err := r.absContextScalar(val.Content[j]); err != nil {
						return err
					}
				}
			}
			if val.Kind == yaml.SequenceNode {
				for _, item := range val.Content {
					if err := r.absContextScalar(item); err != nil {
						return err
					}
				}
			}
		case "ssh":
			switch val.Kind {
			case yaml.SequenceNode:
				for _, item := range val.Content {
					if err := r.absSSHEntry(item); err != nil {
						return err
					}
				}
			case yaml.MappingNode:
				for j := 1; j < len(val.Content); j += 2 {
					if err := r.maybeUnixScalar(val.Content[j]); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// absSSHEntry handles services.*.build.ssh.* in the short form (sequence
// of strings) and the post-canonical mapping form. The short form entries
// are either a bare key (e.g. "default") or "key=path"; only the path
// portion after the `=` is resolved against the working directory. Post-
// canonical, ssh is a mapping whose values are scalar paths and are
// resolved directly.
func (r *nodeResolverState) absSSHEntry(n *yaml.Node) error {
	if n == nil || n.Kind != yaml.ScalarNode {
		return nil
	}
	key, value, hasEq := strings.Cut(n.Value, "=")
	if !hasEq {
		// Bare key (e.g. "default") — nothing to resolve.
		return nil
	}
	tmp := &yaml.Node{Kind: yaml.ScalarNode, Value: value, Line: n.Line, Column: n.Column}
	if err := r.maybeUnixScalar(tmp); err != nil {
		return err
	}
	n.Value = key + "=" + tmp.Value
	return nil
}

// absEnvFileShortForm handles services.*.env_file in every shape it can
// take before Canonical normalizes it: a scalar (env_file: ./foo), a
// sequence of scalars (env_file: [./foo, ./bar]) or a sequence of mappings
// (env_file: [{path: ./foo, required: false}]). The generic walker stops
// at the first matching pattern, so this handler explicitly recurses into
// each sequence shape rather than letting the per-element pattern below
// take over.
func (r *nodeResolverState) absEnvFileShortForm(n *yaml.Node) error {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		return r.absScalar(n)
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if err := r.absEnvFile(item); err != nil {
				return err
			}
		}
	}
	return nil
}

// absEnvFile handles services.*.env_file.* entries. The short form is a
// scalar path; the long form is a mapping with a `path` field. Both are
// resolved against the scalar working directory. For the long form the
// per-field handler ("services.*.env_file.*.path") takes over once the
// walker recurses into the mapping, so this function only acts on the
// short form to avoid double resolution.
func (r *nodeResolverState) absEnvFile(n *yaml.Node) error {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.ScalarNode {
		return r.absScalar(n)
	}
	if n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == "path" {
			if err := r.absScalar(n.Content[i+1]); err != nil {
				return err
			}
		}
	}
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
