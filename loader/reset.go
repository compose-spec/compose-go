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
	"strings"

	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/goccy/go-yaml/ast"
)

type ResetProcessor struct {
	paths []tree.Path
}

func NewResetProcessor(doc *ast.DocumentNode) PostProcessor {
	r := &ResetProcessor{}
	r.parse(doc.Body)
	return r
}

func (p *ResetProcessor) parse(n ast.Node) bool {
	switch n.Type() {
	case ast.TagType:
		t := n.(*ast.TagNode)
		tag := t.Start.Value
		if tag == "!reset" {
			p.paths = append(p.paths, tree.Path(strings.TrimPrefix(n.GetPath(), "$.")))
			return true
		}
		if tag == "!override" {
			p.paths = append(p.paths, tree.Path(strings.TrimPrefix(n.GetPath(), "$.")))
			return false
		}
	case ast.MappingType:
		node := n.(*ast.MappingNode)
		for _, value := range node.Values {
			if p.parse(value.Value) {
				node.Values = removeMapping(node.Values, value.Key.String())
			}
		}
	case ast.SequenceType:
		for _, value := range n.(*ast.SequenceNode).Values {
			p.parse(value)
		}
	}

	return false
}

func removeMapping(nodes []*ast.MappingValueNode, key string) []*ast.MappingValueNode {
	for i, node := range nodes {
		if node.Key.String() == key {
			return append(nodes[:i], nodes[i+1:]...)
		}
	}
	return nodes
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
