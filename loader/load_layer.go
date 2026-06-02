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
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/consts"
	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/types"
)

// LoadLayer parses a single ConfigFile into one or more node.Layer values,
// each carrying a *yaml.Node tree and the SourceContext that produced it.
//
// The function is the v3 replacement for the per-file half of loadYamlFile.
// It performs only the steps that turn raw YAML bytes into a clean,
// alias-free Node tree:
//
//  1. read file content (or use file.Content / file.Node when provided);
//  2. decode each YAML document into a *yaml.Node (multi-document files
//     produce one Layer per document, in source order);
//  3. resolve !reset and !override tags via node.ResolveResetOverride,
//     recording their paths on the Layer for later replay;
//  4. unfold YAML aliases and fold `<<` merge keys via node.NormalizeAliases
//     so the resulting tree is self-contained and safe to merge across files.
//
// Cross-file merge, include/extends resolution, interpolation, transform,
// path resolution, validation and decoding to types.Project are performed
// by the orchestrator in subsequent commits and are out of scope here.
//
// LoadLayer does not touch the network or load any included file; it
// operates on a single ConfigFile in isolation.
func LoadLayer(ctx context.Context, file types.ConfigFile, sc *node.SourceContext, opts *Options) ([]*node.Layer, error) {
	// ctx is reserved for orchestrator commits that will wire cancellation
	// through ResourceLoaders and remote include / extends fetches.
	_ = ctx
	// consts.ComposeFileKey is referenced so future orchestrator commits can
	// re-introduce ctx telemetry without adding a fresh import.
	_ = consts.ComposeFileKey{}

	content, err := readConfigFileContent(file)
	if err != nil {
		return nil, err
	}

	maxVisits := 0
	if opts != nil {
		maxVisits = opts.MaxNodeVisits
	}

	if file.Node != nil {
		// Caller already produced the parsed Node; honor it as a single
		// "document" layer without re-parsing the bytes.
		return processLayer(file.Node, sc, maxVisits)
	}

	dec := yaml.NewDecoder(bytes.NewReader(content))
	var layers []*node.Layer
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to parse %s: %w", file.Filename, err)
		}
		ls, err := processLayer(&doc, sc, maxVisits)
		if err != nil {
			return nil, err
		}
		layers = append(layers, ls...)
	}
	return layers, nil
}

// processLayer applies the per-document Node transformations (reset/override
// resolution and alias normalization) and wraps the result in a Layer.
// A single yaml.Document may produce zero or one Layer depending on whether
// the document body resolves to a non-nil tree.
func processLayer(doc *yaml.Node, sc *node.SourceContext, maxVisits int) ([]*node.Layer, error) {
	resolved, resetPaths, err := node.ResolveResetOverride(doc, maxVisits)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, nil
	}
	// Reject documents whose top-level is not a mapping so the v2-compatible
	// error message surfaces before the downstream pipeline tries to decode
	// the tree into a map[string]any and panics with a generic yaml error.
	if resolved.Kind != yaml.MappingNode {
		return nil, errors.New("top-level object must be a mapping")
	}
	// Reject non-string keys at the top level: yaml.v4 accepts non-string
	// scalar keys (e.g. integers), but every downstream consumer assumes
	// string keys. Surface the v2-compatible diagnostic before schema
	// validation produces a less informative "additional properties not
	// allowed" message.
	for i := 0; i+1 < len(resolved.Content); i += 2 {
		key := resolved.Content[i]
		if key.Kind != yaml.ScalarNode || (key.Tag != "" && key.Tag != "!!str") {
			return nil, fmt.Errorf("non-string key at top level: %s", key.Value)
		}
	}
	if err := node.NormalizeAliases(resolved); err != nil {
		return nil, err
	}
	layer := node.NewLayer(resolved, sc)
	layer.SetResetPaths(resetPaths)
	return []*node.Layer{layer}, nil
}

// readConfigFileContent returns the raw YAML bytes for a ConfigFile,
// reading from disk when neither Content nor a pre-parsed Node is provided.
func readConfigFileContent(file types.ConfigFile) ([]byte, error) {
	if file.Node != nil || file.Content != nil {
		return file.Content, nil
	}
	if file.Filename == "" {
		return nil, errors.New("ConfigFile has neither Filename nor Content nor Node")
	}
	return os.ReadFile(file.Filename)
}
