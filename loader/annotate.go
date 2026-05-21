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
	"context"
	"fmt"

	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// LoadAnnotatedYaml runs the v3 yaml.Node pipeline on configDetails and
// returns the merged yaml tree serialized with per-scalar comments pointing
// at the source file:line for every leaf that does not originate from the
// main config file (the first entry of configDetails.ConfigFiles).
//
// The output is intended for diagnostic purposes — it shows the resolved
// model with provenance hints, not the canonical *types.Project produced
// by LoadWithContext.
func LoadAnnotatedYaml(ctx context.Context, configDetails types.ConfigDetails, options ...func(*Options)) ([]byte, error) {
	opts := ToOptions(&configDetails, options)
	model, err := load(ctx, &configDetails, opts)
	if err != nil {
		return nil, err
	}

	var mainSource string
	if len(configDetails.ConfigFiles) > 0 {
		mainSource = configDetails.ConfigFiles[0].Filename
	}
	annotateNonMainScalars(model.Merged(), model.contexts, mainSource, map[*yaml.Node]bool{})

	return yaml.Marshal(model.Merged())
}

// annotateNonMainScalars walks the yaml tree and sets a LineComment on
// every ScalarNode whose registered NodeContext points at a source other
// than mainSource. visited prevents infinite recursion on alias cycles.
func annotateNonMainScalars(node *yaml.Node, contexts map[*yaml.Node]*types.NodeContext, mainSource string, visited map[*yaml.Node]bool) {
	if node == nil || visited[node] {
		return
	}
	visited[node] = true
	if node.Kind == yaml.ScalarNode {
		if ctx, ok := contexts[node]; ok && ctx != nil && ctx.Source != "" && ctx.Source != mainSource {
			node.LineComment = fmt.Sprintf("%s:%d", ctx.Source, node.Line)
		}
		return
	}
	for _, c := range node.Content {
		annotateNonMainScalars(c, contexts, mainSource, visited)
	}
	if node.Alias != nil {
		annotateNonMainScalars(node.Alias, contexts, mainSource, visited)
	}
}
