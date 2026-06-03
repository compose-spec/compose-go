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

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/tree"
)

// ResetProcessor adapts node.ResolveResetOverride to the yaml.Decoder API.
// It collects the !reset/!override paths during YAML decoding and replays
// them as map deletions via Apply, after the v2 pipeline has merged the
// decoded documents into the running map[string]any.
//
// The yaml.Node-side logic lives in internal/node so the upcoming merge
// phase can reuse it without going through the legacy map[string]any path.
type ResetProcessor struct {
	target        any
	paths         []tree.Path
	maxNodeVisits int
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (p *ResetProcessor) UnmarshalYAML(value *yaml.Node) error {
	resolved, paths, err := node.ResolveResetOverride(value, p.maxNodeVisits)
	if err != nil {
		return err
	}
	p.paths = paths
	return resolved.Decode(p.target)
}

// Apply walks target (a map[string]any tree decoded from YAML) and removes
// every entry whose path matches one of the recorded !reset/!override paths.
// This is the v2 post-merge cleanup; replaced by a direct Node-tree
// rewrite during merge.
func (p *ResetProcessor) Apply(target any) error {
	return p.applyNullOverrides(target, tree.NewPath())
}

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
			if err := p.applyNullOverrides(e, next); err != nil {
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
					// TODO(ndeloof) support removal from sequence — tracked.
				}
			}
			if err := p.applyNullOverrides(e, next); err != nil {
				return err
			}
		}
	}
	return nil
}
