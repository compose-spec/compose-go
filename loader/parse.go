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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// parseLayers reads every ConfigFile in details and turns it into a Layer
// attached to a per-file NodeContext (Source = filename, WorkingDir = the
// project working dir, Env = the project environment). Each node of each
// parsed tree is registered in the model's contexts map.
func (m *ComposeModel) parseLayers(details types.ConfigDetails) error {
	for _, file := range details.ConfigFiles {
		node, err := loadYamlFileNode(file)
		if err != nil {
			return err
		}
		if node == nil {
			continue
		}
		ctx := &types.NodeContext{
			Source:     file.Filename,
			WorkingDir: details.WorkingDir,
			Env:        details.Environment,
		}
		m.layers = append(m.layers, &Layer{Root: node, Context: ctx})
		m.registerNodes(node, ctx)
		if file.Filename != "" {
			m.loadedFiles = append(m.loadedFiles, file.Filename)
		}
	}
	return nil
}

// loadYamlFileNode reads a ConfigFile and parses it into a *yaml.Node tree.
// The returned node is a DocumentNode wrapping the top-level mapping. Returns
// (nil, nil) for an empty file.
//
// !reset and !override tags are preserved in the tree; they are handled by
// the merger and the post-merge cleanup pass.
func loadYamlFileNode(file types.ConfigFile) (*yaml.Node, error) {
	content := file.Content
	if content == nil && file.Config == nil {
		if file.Filename == "" {
			return nil, errors.New("config file has no filename and no content")
		}
		data, err := os.ReadFile(file.Filename)
		if err != nil {
			return nil, err
		}
		content = data
	}
	if file.Config != nil {
		b, err := yaml.Marshal(file.Config)
		if err != nil {
			return nil, fmt.Errorf("marshaling pre-parsed config: %w", err)
		}
		content = b
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var result *yaml.Node
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse %s: %w", file.Filename, err)
		}
		result = &doc
	}
	if result == nil {
		return nil, nil
	}
	if err := checkDuplicateKeys(result); err != nil {
		return nil, fmt.Errorf("%s: %w", file.Filename, err)
	}
	return result, nil
}

// checkDuplicateKeys recursively walks a yaml tree and returns an error if
// any MappingNode contains duplicate keys. yaml/v4 is permissive about this
// (last value wins) but Compose forbids it.
func checkDuplicateKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := checkDuplicateKeys(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if line, dup := seen[key.Value]; dup {
				return fmt.Errorf("line %d: mapping key %q already defined at line %d", key.Line, key.Value, line)
			}
			seen[key.Value] = key.Line
			if err := checkDuplicateKeys(node.Content[i+1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkNonStringKeys walks a yaml tree and returns an error if any mapping
// key is not a string (e.g. an integer key like `123: value`). prefix is used
// to build a human-readable location in error messages; pass "" at the root.
func checkNonStringKeys(node *yaml.Node, prefix string) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := checkNonStringKeys(child, prefix); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Tag != "" && key.Tag != "!!str" && key.Tag != "!!merge" {
				location := "at top level"
				if prefix != "" {
					location = fmt.Sprintf("in %s", prefix)
				}
				return fmt.Errorf("non-string key %s: %s", location, key.Value)
			}
			childPrefix := key.Value
			if prefix != "" {
				childPrefix = prefix + "." + key.Value
			}
			if err := checkNonStringKeys(val, childPrefix); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			childPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			if err := checkNonStringKeys(item, childPrefix); err != nil {
				return err
			}
		}
	}
	return nil
}
