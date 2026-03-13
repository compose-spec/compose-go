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

package types

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// NodeError is an error that carries yaml source location (file, line, column).
type NodeError struct {
	Line   int
	Column int
	Source string
	Err    error
}

func (e *NodeError) Error() string {
	if e.Source != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.Source, e.Line, e.Column, e.Err)
	}
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Err)
}

func (e *NodeError) Unwrap() error {
	return e.Err
}

// NodeErrorf creates a NodeError from a yaml.Node and a formatted message.
func NodeErrorf(node *yaml.Node, format string, args ...any) error {
	return &NodeError{
		Line:   node.Line,
		Column: node.Column,
		Err:    fmt.Errorf(format, args...),
	}
}

// WrapNodeError wraps an existing error with yaml.Node source location.
func WrapNodeError(node *yaml.Node, err error) error {
	if err == nil {
		return nil
	}
	return &NodeError{
		Line:   node.Line,
		Column: node.Column,
		Err:    err,
	}
}

// WithSource enriches any NodeError instances found in the error chain or
// message with the given source filename. It handles errors wrapped by
// yaml/v4's LoadErrors which break the standard errors.As chain.
func WithSource(err error, source string) error {
	if err == nil {
		return nil
	}
	// Direct match
	if ne, ok := err.(*NodeError); ok {
		return &NodeError{
			Line:   ne.Line,
			Column: ne.Column,
			Source: source,
			Err:    ne.Err,
		}
	}
	// Try to find NodeError via Unwrap
	var ne *NodeError
	if errors.As(err, &ne) {
		enriched := &NodeError{
			Line:   ne.Line,
			Column: ne.Column,
			Source: source,
			Err:    ne.Err,
		}
		return fmt.Errorf("%w", enriched)
	}
	// yaml/v4 LoadErrors wraps errors in a way that breaks errors.As.
	// Check if the error message already has line info from our NodeError.Error().
	// If so, enrich the message with the source file.
	msg := err.Error()
	if strings.Contains(msg, "line ") && strings.Contains(msg, "column ") {
		return fmt.Errorf("%s: %s", source, msg)
	}
	return err
}

// resolveYAMLNode unwraps a DocumentNode wrapper that yaml/v4 passes to
// UnmarshalYAML methods. If the node is a DocumentNode with a single child,
// the child is returned; otherwise the node is returned as-is.
func resolveYAMLNode(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}

// hasKey checks if a MappingNode contains a specific key
func hasKey(node *yaml.Node, key string) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

// findYAMLKey finds a key in a MappingNode and returns (key node, value node).
func findYAMLKey(node *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i], node.Content[i+1]
		}
	}
	return nil, nil
}

// ParseVolumeFunc is a package-level hook for parsing volume short syntax.
// It is set by the loader package to break the circular dependency between
// types and format (format imports types, so types cannot import format).
var ParseVolumeFunc func(string) (ServiceVolumeConfig, error)
