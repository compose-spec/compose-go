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
	"strconv"

	"github.com/compose-spec/compose-go/tree"
	"gopkg.in/yaml.v3"
)

type ResetProcessor struct {
	target interface{}
	paths  []tree.Path
}

// UnmarshalYAML implement yaml.Unmarshaler
func (p *ResetProcessor) UnmarshalYAML(value *yaml.Node) error {
	resolved, err := p.resolveReset(value, tree.NewPath())
	if err != nil {
		return err
	}
	return resolved.Decode(p.target)
}

// resolveReset detects `!reset` tag being set on yaml nodes and record position in the yaml tree
func (p *ResetProcessor) resolveReset(node *yaml.Node, path tree.Path) (*yaml.Node, error) {
	if node.Tag == "!reset" {
		p.paths = append(p.paths, path)
	}
	switch node.Kind {
	case yaml.SequenceNode:
		var err error
		for idx, v := range node.Content {
			next := path.Next(strconv.Itoa(idx))
			node.Content[idx], err = p.resolveReset(v, next)
			if err != nil {
				return nil, err
			}
		}
	case yaml.MappingNode:
		var err error
		var key string
		for idx, v := range node.Content {
			if idx%2 == 0 {
				key = v.Value
			} else {
				node.Content[idx], err = p.resolveReset(v, path.Next(key))
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return node, nil
}

// Apply finds the go attributes matching recorded paths and reset them to zero value
func (p *ResetProcessor) Apply(target any) error {
	return p.applyNullOverrides(target, tree.NewPath())
}

// applyNullOverrides set val to Zero if it matches any of the recorded paths
func (p *ResetProcessor) applyNullOverrides(target any, path tree.Path) error {
	switch v := target.(type) {
	case map[string]any:
	KEYS:
		for k, e := range v {
			next := path.Next(k)
			for _, pattern := range p.paths {
				if next.Matches(pattern) {
					delete(v, k)
					continue KEYS
				}
			}
			err := p.applyNullOverrides(e, next)
			if err != nil {
				return err
			}
		}
	case []any:
	ITER:
		for i, e := range v {
			next := path.Next(fmt.Sprintf("[%d]", i))
			for _, pattern := range p.paths {
				if next.Matches(pattern) {
					continue ITER
					// TODO(ndeloof) support removal from sequence
				}
			}
			err := p.applyNullOverrides(e, next)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
