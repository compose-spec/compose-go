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
	"errors"
	"fmt"

	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/schema"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
)

// Resolve orchestrates the v3 loader pipeline on the model's layers and
// returns the merged yaml.Node tree together with its untyped map
// representation. The yaml.Node tree retains per-node loading context, so
// callers can attach those contexts to decoded types (typically EnvFile)
// after decoding.
//
// Steps:
//  1. extends:   each layer's extends directives are resolved in-tree
//  2. include:   include directives expand into extra layers with their own
//     NodeContext (recursive)
//  3. merge:     all layers are merged into a single yaml.Node, preserving
//     leaf pointer identity so per-node contexts remain valid
//  4. cleanup:   !reset / !override post-merge cleanup
//  5. interpolate: per-scalar substitution using the per-node Env
//  6. paths:     relative paths rewritten against the per-node WorkingDir
//     (env_file paths excluded — handled later by
//     WithServicesEnvironmentResolved using EnvFile.Context)
//  7. validate:  Compose JSON Schema validation directly on the yaml tree
//
// Once Resolve returns, the merged tree is also stored on the model and
// available through ComposeModel.Merged() and ComposeModel.Dict().
func (m *ComposeModel) Resolve(ctx context.Context) error {
	if !m.opts.SkipExtends {
		for _, layer := range m.layers {
			if err := m.applyExtendsNode(ctx, layer); err != nil {
				return err
			}
		}
	}

	if !m.opts.SkipInclude {
		if err := m.applyIncludeNodes(ctx); err != nil {
			return err
		}
	}

	// Bare environment references (entries like `- VAR_NAME` without an
	// assignment) are resolved per layer so each one uses its own
	// NodeContext.Env, including variables provided by include.env_file.
	m.resolveBareEnvironmentRefs()

	// Interpolate each layer separately before merging so that variable
	// references declared in an included or extends layer are resolved
	// against that layer's own NodeContext.Env. Doing it after merge would
	// risk losing the per-include environment for nodes whose ancestor was
	// reshaped by a specialized merger (e.g. mergeBuildNode wrapping a
	// scalar in a {context: …} mapping with a freshly created parent).
	if !m.opts.SkipInterpolation {
		for _, layer := range m.layers {
			if err := m.interpolateTree(layer.Root, tree.NewPath(), layer.Context); err != nil {
				return err
			}
		}
	}

	merged, err := m.mergeLayers()
	if err != nil {
		return err
	}

	override.StripResetTags(merged)
	override.EnforceUnicityNode(merged, tree.NewPath())

	// yaml/v4 leaves merge keys (`<<: *anchor`) in the Node tree; the
	// schema would reject them as "additional properties". Inline them
	// before validation.
	if err := resolveMergeKeys(merged); err != nil {
		return err
	}

	// Inject `build.context: .` default for services that still have a
	// build mapping without an explicit context after the merge. The new
	// scalar is anchored to the main NodeContext so path resolution treats
	// it as a main-project relative path (mirrors the legacy behaviour
	// where transform.SetDefaultValues + paths.ResolveRelativePaths anchor
	// the default at the project working directory).
	m.injectMissingBuildContext(merged)

	// Always invoke the path pass. Even when ResolvePaths is false the pass
	// still rewrites paths that came from an included or extends file so they
	// become relative to the main working directory, matching the legacy
	// behaviour where paths.ResolveRelativePaths was force-enabled on
	// included subtrees.
	m.resolvePathsPass(merged)

	if !m.opts.SkipValidation {
		if err := schema.ValidateNode(merged); err != nil {
			source := firstSource(m.layers)
			if source == "" {
				source = "compose model"
			}
			return fmt.Errorf("validating %s: %w", source, err)
		}
	}

	m.merged = merged
	return nil
}

// mergeLayers folds every Layer into a single yaml.Node mapping by applying
// override.MergeNodes in declaration order. Returns an error if no layer is
// available or if the resulting tree is not a mapping.
func (m *ComposeModel) mergeLayers() (*yaml.Node, error) {
	if len(m.layers) == 0 {
		return nil, errors.New("empty compose model")
	}
	var merged *yaml.Node
	for _, layer := range m.layers {
		node := unwrapDocument(layer.Root)
		if merged == nil {
			merged = node
			continue
		}
		next, err := override.MergeNodes(merged, node, tree.NewPath())
		if err != nil {
			return nil, fmt.Errorf("merging %s: %w", layer.Context.Source, err)
		}
		merged = next
	}
	root := unwrapDocument(merged)
	if root == nil {
		return nil, errors.New("empty compose model")
	}
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("top-level object must be a mapping")
	}
	if err := checkNonStringKeys(firstSource(m.layers), root, ""); err != nil {
		return nil, err
	}
	return root, nil
}

// injectMissingBuildContext adds `context: .` to every `build:` mapping
// that has no `context` key in the merged tree. The inserted scalar is
// registered with the main project context so the path resolution pass
// anchors it at configDetails.WorkingDir, matching the legacy semantics
// where transform.SetDefaultValues + paths.ResolveRelativePaths add and
// resolve the default at the project root.
func (m *ComposeModel) injectMissingBuildContext(root *yaml.Node) {
	if len(m.layers) == 0 {
		return
	}
	mainCtx := m.layers[len(m.layers)-1].Context
	doc := unwrapDocument(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return
	}
	_, services := override.FindKey(doc, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svc := services.Content[i+1]
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		_, build := override.FindKey(svc, "build")
		if build == nil || build.Kind != yaml.MappingNode {
			continue
		}
		if _, ctx := override.FindKey(build, "context"); ctx != nil {
			continue
		}
		ctxNode := override.NewScalar(".")
		m.contexts[ctxNode] = mainCtx
		override.SetKey(build, "context", ctxNode)
	}
}

// injectBuildContextDefault adds `context: .` to every `build:` mapping that
// has no `context` key so the path resolution pass can rewrite it against
// the build node's own NodeContext.WorkingDir. Mirrors what
// transform.defaultBuildContext does in the legacy post-merge pipeline.
//
// Used by applyIncludeNodes on a freshly loaded included tree to ensure the
// default context is anchored at the included file's working directory
// (not the main project directory) — matching the legacy behaviour where
// SetDefaultValues runs on the included dict before it is merged into the
// main one.
func injectBuildContextDefault(root *yaml.Node) {
	doc := unwrapDocument(root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return
	}
	_, services := override.FindKey(doc, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svc := services.Content[i+1]
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		_, build := override.FindKey(svc, "build")
		if build == nil || build.Kind != yaml.MappingNode {
			continue
		}
		if _, ctx := override.FindKey(build, "context"); ctx == nil {
			override.SetKey(build, "context", override.NewScalar("."))
		}
	}
}

func firstSource(layers []*Layer) string {
	if len(layers) == 0 {
		return ""
	}
	return layers[0].Context.Source
}

// load builds the ComposeModel and runs the v3 pipeline through Resolve.
// The merged yaml.Node tree and its untyped map projection are accessible
// on the returned model through Merged() and Dict().
func load(ctx context.Context, configDetails *types.ConfigDetails, opts *Options) (*ComposeModel, error) {
	if len(configDetails.ConfigFiles) < 1 {
		return nil, errors.New("no compose file specified")
	}
	if err := projectName(configDetails, opts); err != nil {
		return nil, err
	}

	model := newComposeModel(*configDetails, opts)
	if err := model.parseLayers(*configDetails); err != nil {
		return nil, err
	}
	if len(model.layers) == 0 {
		return nil, errors.New("empty compose file")
	}

	if err := model.Resolve(ctx); err != nil {
		return nil, err
	}
	return model, nil
}
