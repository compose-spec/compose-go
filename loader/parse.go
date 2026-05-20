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

// parseLayers reads every ConfigFile in details and turns it into one or
// more Layers attached to a per-file NodeContext (Source = filename,
// WorkingDir = the project working dir, Env = the project environment).
// A single ConfigFile that contains multiple yaml documents (separated by
// `---`) is expanded into one Layer per document, in declaration order, so
// each later document overrides the previous one through the merge pass.
// Each node of each parsed tree is registered in the model's contexts map.
func (m *ComposeModel) parseLayers(details types.ConfigDetails) error {
	for _, file := range details.ConfigFiles {
		docs, err := loadYamlFileNodes(file)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			continue
		}
		for _, node := range docs {
			ctx := &types.NodeContext{
				Source:     file.Filename,
				WorkingDir: details.WorkingDir,
				Env:        details.Environment,
			}
			m.layers = append(m.layers, &Layer{Root: node, Context: ctx})
			m.registerNodes(node, ctx)
		}
		if file.Filename != "" {
			m.loadedFiles = append(m.loadedFiles, file.Filename)
		}
	}
	return nil
}

// loadYamlFileNode reads a ConfigFile and parses it into a *yaml.Node tree.
// The returned node is a DocumentNode wrapping the top-level mapping. For
// multi-document streams this helper returns only the last document; use
// loadYamlFileNodes when every document must be processed.
//
// !reset and !override tags are preserved in the tree; they are handled by
// the merger and the post-merge cleanup pass.
func loadYamlFileNode(file types.ConfigFile) (*yaml.Node, error) {
	docs, err := loadYamlFileNodes(file)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, nil
	}
	return docs[len(docs)-1], nil
}

// loadYamlFileNodes reads a ConfigFile and parses every yaml document it
// contains. A multi-document stream (typical of compose files produced by
// `compose publish` to an OCI artifact, which packs the main file and every
// override into a single multi-document layer) yields one *yaml.Node per
// document. Returns nil/empty for an empty file.
func loadYamlFileNodes(file types.ConfigFile) ([]*yaml.Node, error) {
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
	var docs []*yaml.Node
	for {
		var doc yaml.Node
		if err := decoder.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse %s: %w", file.Filename, err)
		}
		// Skip empty documents (e.g. trailing `---` yields a DocumentNode
		// whose only child is a null scalar).
		if isEmptyDocument(&doc) {
			continue
		}
		clone := doc
		if err := checkDuplicateKeys(file.Filename, &clone); err != nil {
			return nil, err
		}
		docs = append(docs, &clone)
	}
	return docs, nil
}

// isEmptyDocument reports whether a parsed DocumentNode has no usable
// content, either because it has no children or because its sole child is
// a null scalar. yaml/v4 emits such a document for a stream containing
// only `---` separators with no payload between them.
func isEmptyDocument(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.DocumentNode {
		return false
	}
	if len(node.Content) == 0 {
		return true
	}
	for _, c := range node.Content {
		if c == nil {
			continue
		}
		if c.Kind == yaml.ScalarNode && (c.Tag == "!!null" || c.Value == "") {
			continue
		}
		return false
	}
	return true
}

// checkDuplicateKeys recursively walks a yaml tree and returns an error if
// any MappingNode contains duplicate keys. yaml/v4 is permissive about this
// (last value wins) but Compose forbids it. The source filename is included
// in error messages as "<source>:<line>:<column>: …" so users can jump to
// the offending location.
func checkDuplicateKeys(source string, node *yaml.Node) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := checkDuplicateKeys(source, child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if line, dup := seen[key.Value]; dup {
				origin := (&types.NodeContext{Source: source}).OriginAt(key)
				return fmt.Errorf("%s: mapping key %q already defined at line %d", origin, key.Value, line)
			}
			seen[key.Value] = key.Line
			if err := checkDuplicateKeys(source, node.Content[i+1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkNonStringKeys walks a yaml tree and returns an error if any mapping
// key is not a string (e.g. an integer key like `123: value`). source is the
// yaml file path used to anchor errors to "<source>:<line>:<column>".
// prefix is used to build a human-readable location in error messages; pass
// "" at the root.
func checkNonStringKeys(source string, node *yaml.Node, prefix string) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := checkNonStringKeys(source, child, prefix); err != nil {
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
				origin := (&types.NodeContext{Source: source}).OriginAt(key)
				return fmt.Errorf("%s: non-string key %s: %s", origin, location, key.Value)
			}
			childPrefix := key.Value
			if prefix != "" {
				childPrefix = prefix + "." + key.Value
			}
			if err := checkNonStringKeys(source, val, childPrefix); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			childPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			if err := checkNonStringKeys(source, item, childPrefix); err != nil {
				return err
			}
		}
	}
	return nil
}
