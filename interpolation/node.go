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

package interpolation

import (
	"errors"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/template"
	"github.com/compose-spec/compose-go/v3/tree"
)

// NodeOptions configures InterpolateNode.
type NodeOptions struct {
	// LookupValueFor returns the variable lookup function to consult for a
	// given scalar node. It is invoked once per scalar visited, which is
	// what enables lazy per-Layer interpolation: callers can hand back a
	// different lookup based on the SourceContext attached to each scalar
	// after a cross-file merge.
	//
	// When nil, every scalar uses LookupValue.
	LookupValueFor func(*yaml.Node) LookupValue

	// LookupValue is the fall-back single lookup used when LookupValueFor is
	// nil. Provided for parity with v2 callers that have only one environment
	// for the entire document.
	LookupValue LookupValue

	// Substitute is the template substitution function; defaults to
	// template.Substitute when nil. The shape matches v2.
	Substitute func(string, template.Mapping) (string, error)

	// Tags maps tree.Path patterns to a YAML tag ("!!int", "!!bool",
	// "!!float", ...). After substitution, scalars whose path matches a
	// pattern have their Tag updated so the eventual (*yaml.Node).Decode
	// produces the right Go type — replacing the legacy mapstructure cast
	// hook with native YAML decoding semantics.
	Tags map[tree.Path]string
}

// InterpolateNode walks a yaml.Node tree and substitutes ${VAR} references
// in every scalar value, using a LookupValue that may be picked per-node.
// This is the v3 interpolation phase: it runs after the cross-file merge so
// each scalar can be interpolated in the SourceContext of its layer of
// origin — the bug fix that motivates the whole refactor.
//
// Mapping keys are not interpolated (matching v2 behavior). When a scalar's
// path matches an entry in opts.Tags, its Tag is rewritten so that yaml.v4
// converts the value to the expected target type at decode time.
//
// The tree is mutated in place. An error from the substitution function or
// from the template parser short-circuits the walk; the returned error is
// wrapped with the source path for diagnostics.
func InterpolateNode(root *yaml.Node, opts NodeOptions) error {
	if opts.Substitute == nil {
		opts.Substitute = template.Substitute
	}
	if opts.LookupValueFor == nil {
		if opts.LookupValue == nil {
			return errors.New("interpolation: LookupValueFor or LookupValue must be set")
		}
		lookup := opts.LookupValue
		opts.LookupValueFor = func(*yaml.Node) LookupValue { return lookup }
	}
	return node.Walk(root, func(p tree.Path, n *yaml.Node) error {
		if n == nil || n.Kind != yaml.ScalarNode {
			return nil
		}
		// !!null scalars carry no substitutable content.
		if n.Tag == "!!null" {
			return nil
		}
		lookup := opts.LookupValueFor(n)
		substituted, err := opts.Substitute(n.Value, template.Mapping(lookup))
		if err != nil {
			return &Error{Path: p, Node: n, Cause: newPathError(p, err)}
		}
		n.Value = substituted
		if tag, ok := tagFor(p, opts.Tags); ok {
			n.Tag = tag
		}
		return nil
	})
}

// Error is returned by InterpolateNode when substitution fails on a
// scalar. It carries the offending *yaml.Node and tree.Path so the
// loader can wrap it with the source file from the origins side-table
// and surface an errdefs.Diagnostic.
type Error struct {
	Path  tree.Path
	Node  *yaml.Node
	Cause error
}

func (e *Error) Error() string {
	if e == nil || e.Cause == nil {
		return ""
	}
	return e.Cause.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func tagFor(p tree.Path, tags map[tree.Path]string) (string, bool) {
	for pattern, tag := range tags {
		if p.Matches(pattern) {
			return tag, true
		}
	}
	return "", false
}
