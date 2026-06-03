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

package validation

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
)

type nodeChecker func(n *yaml.Node, p tree.Path) error

// nodeChecks mirrors `checks` but operates on *yaml.Node so the v3 pipeline
// can validate the merged tree without round-tripping through map[string]any.
// Entries stay in sync with the legacy map; the v2 map disappears when the
// map-based code path is removed.
var nodeChecks = map[tree.Path]nodeChecker{
	"volumes.*":                       checkVolumeNode,
	"configs.*":                       checkFileObjectNode("file", "environment", "content"),
	"secrets.*":                       checkFileObjectNode("file", "environment"),
	"services.*.ports.*":              checkIPAddressNode,
	"services.*.develop.watch.*.path": checkPathNode,
	"services.*.deploy.resources.reservations.devices.*": checkDeviceRequestNode,
	"services.*.gpus.*": checkDeviceRequestNode,
}

// Error carries the offending node and path alongside the underlying
// validation failure so the loader can wrap it with the source file
// from the origins side-table when surfacing the error.
//
// The type name intentionally avoids the "ValidationError" stuttering
// against the package name; consumers should refer to it as
// *validation.Error.
type Error struct {
	Path  tree.Path
	Node  *yaml.Node
	Cause error
}

// Error renders as "path: cause" so the existing test assertions that
// match on the substring keep working when validation is not wrapped
// further upstream.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Path.String() == "" {
		return e.Cause.Error()
	}
	return e.Path.String() + ": " + e.Cause.Error()
}

// Unwrap exposes Cause so errors.Is / errors.As walk through.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// ValidateNode walks root and applies the per-path validation checks. The
// tree is not mutated; only errors are reported. The function returns at the
// first failing check, with the offending tree.Path and *yaml.Node included
// in a *ValidationError so callers can map it back to a source location.
func ValidateNode(root *yaml.Node) error {
	if root == nil {
		return nil
	}
	target := root
	if target.Kind == yaml.DocumentNode && len(target.Content) == 1 {
		target = target.Content[0]
	}
	return checkNode(target, tree.NewPath())
}

func wrapCheckError(err error, node *yaml.Node, p tree.Path) error {
	if err == nil {
		return nil
	}
	var ve *Error
	if errors.As(err, &ve) {
		return ve
	}
	return &Error{Path: p, Node: node, Cause: stripPathPrefix(err, p)}
}

// stripPathPrefix removes the "path: " prefix the per-check helpers
// embed in their error strings so wrapping does not duplicate it.
func stripPathPrefix(err error, p tree.Path) error {
	prefix := p.String() + ": "
	if prefix == ": " {
		return err
	}
	msg := err.Error()
	if len(msg) > len(prefix) && msg[:len(prefix)] == prefix {
		return errString(msg[len(prefix):])
	}
	return err
}

type errString string

func (e errString) Error() string { return string(e) }

func checkNode(n *yaml.Node, p tree.Path) error {
	if n == nil {
		return nil
	}
	for pattern, fn := range nodeChecks {
		if p.Matches(pattern) {
			return wrapCheckError(fn(n, p), n, p)
		}
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if err := checkNode(n.Content[i+1], p.Next(n.Content[i].Value)); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if err := checkNode(c, p.Next(tree.PathMatchList)); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkFileObjectNode(keys ...string) nodeChecker {
	return func(n *yaml.Node, p tree.Path) error {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		count := 0
		for _, k := range keys {
			if mappingFieldNode(n, k) != nil {
				count++
			}
		}
		if count > 1 {
			return fmt.Errorf("%s: %s attributes are mutually exclusive", p, strings.Join(keys, "|"))
		}
		if count == 0 {
			if mappingFieldNode(n, "driver") != nil {
				// Custom driver: may carry its own content channel.
				return nil
			}
			if mappingFieldNode(n, "external") == nil {
				return fmt.Errorf("%s: one of %s must be set", p, strings.Join(keys, "|"))
			}
		}
		return nil
	}
}

func checkPathNode(n *yaml.Node, p tree.Path) error {
	if n == nil || n.Kind != yaml.ScalarNode || n.Value == "" {
		return fmt.Errorf("%s: value can't be blank", p)
	}
	return nil
}

func checkDeviceRequestNode(n *yaml.Node, p tree.Path) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	if mappingFieldNode(n, "count") != nil && mappingFieldNode(n, "device_ids") != nil {
		return fmt.Errorf(`%s: "count" and "device_ids" attributes are exclusive`, p)
	}
	return nil
}

func checkIPAddressNode(n *yaml.Node, p tree.Path) error {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	ip := mappingFieldNode(n, "host_ip")
	if ip == nil || ip.Kind != yaml.ScalarNode {
		return nil
	}
	if net.ParseIP(ip.Value) == nil {
		return fmt.Errorf("%s: invalid ip address: %s", p, ip.Value)
	}
	return nil
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
