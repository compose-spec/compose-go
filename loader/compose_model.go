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
	"path/filepath"
	"slices"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/compose-spec/compose-go/v2/format"
	interp "github.com/compose-spec/compose-go/v2/interpolation"
	"github.com/compose-spec/compose-go/v2/override"
	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
)

// NodeContext holds the loading context for a set of yaml nodes.
// Each node parsed from a compose file is associated with a NodeContext
// that captures the environment variables, working directory, and source file
// active at parse time. This allows deferred interpolation and path resolution
// using the correct context for each node, even after nodes from different
// files have been merged.
type NodeContext struct {
	Source     string
	WorkingDir string
	Env        types.Mapping
}

// Layer represents a single compose file parsed as a yaml.Node tree
// together with its loading context.
type Layer struct {
	Node    *yaml.Node
	Context *NodeContext
}

// ComposeModel holds raw yaml layers and resolves them lazily into a types.Project.
// Yaml nodes are kept in their raw (uninterpolated) form as long as possible.
// Interpolation, type casting, and path resolution are deferred until Resolve()
// is called, at which point each node is processed using its own NodeContext.
type ComposeModel struct {
	layers        []*Layer
	configDetails types.ConfigDetails
	opts          *Options
	// nodeContexts maps each yaml.Node to the loading context it was parsed under.
	// This survives merging: original nodes keep their pointer identity, so
	// after MergeNodes the map still resolves each leaf to its source context.
	nodeContexts map[*yaml.Node]*NodeContext
}

func init() {
	// Wire up the volume-parsing hook so that types.ServiceVolumeConfig.UnmarshalYAML
	// can parse short syntax without importing the format package directly.
	types.ParseVolumeFunc = func(s string) (types.ServiceVolumeConfig, error) {
		return format.ParseVolume(s)
	}
}

// LoadLazyModel parses compose files into raw yaml.Node layers without
// performing interpolation or normalization. The resulting ComposeModel
// can later be materialized into a types.Project by calling Resolve().
func LoadLazyModel(ctx context.Context, configDetails types.ConfigDetails, options ...func(*Options)) (*ComposeModel, error) {
	opts := ToOptions(&configDetails, options)

	if len(configDetails.ConfigFiles) < 1 {
		return nil, errors.New("no compose file specified")
	}

	if err := projectName(&configDetails, opts); err != nil {
		return nil, err
	}

	model := &ComposeModel{
		configDetails: configDetails,
		opts:          opts,
		nodeContexts:  make(map[*yaml.Node]*NodeContext),
	}

	for _, file := range configDetails.ConfigFiles {
		node, err := loadYamlFileNode(file)
		if err != nil {
			return nil, err
		}
		if node == nil {
			continue
		}
		nodeCtx := &NodeContext{
			Source:     file.Filename,
			WorkingDir: configDetails.WorkingDir,
			Env:        configDetails.Environment,
		}
		layer := &Layer{Node: node, Context: nodeCtx}
		model.layers = append(model.layers, layer)
		model.registerNodes(node, nodeCtx)
	}

	if len(model.layers) == 0 {
		return nil, errors.New("empty compose file")
	}

	return model, nil
}

// registerNodes associates every node in a tree with the given context.
func (m *ComposeModel) registerNodes(node *yaml.Node, ctx *NodeContext) {
	if node == nil {
		return
	}
	m.nodeContexts[node] = ctx
	for _, child := range node.Content {
		m.registerNodes(child, ctx)
	}
	if node.Alias != nil {
		m.registerNodes(node.Alias, ctx)
	}
}

// loadYamlFileNode parses a ConfigFile into a *yaml.Node tree,
// processing !reset and !override tags via ResetProcessor.
func loadYamlFileNode(file types.ConfigFile) (*yaml.Node, error) {
	content := file.Content
	if content == nil && file.Config == nil {
		var err error
		content, err = os.ReadFile(file.Filename)
		if err != nil {
			return nil, err
		}
	}

	if file.Config != nil {
		// Config is already a map[string]any — marshal back to yaml then parse as node.
		// This path is rare (used in tests) and maintains compatibility.
		b, err := yaml.Marshal(file.Config)
		if err != nil {
			return nil, err
		}
		content = b
	}

	r := bytes.NewReader(content)
	decoder := yaml.NewDecoder(r)

	var result *yaml.Node
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if err != nil && errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file.Filename, err)
		}
		// Process reset/override tags
		resolved, err := processResetTags(&doc)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			result = resolved
		}
	}
	return result, nil
}

// processResetTags walks the yaml.Node tree and handles !reset / !override tags.
func processResetTags(node *yaml.Node) (*yaml.Node, error) {
	return resolveResetNode(node, tree.NewPath())
}

func resolveResetNode(node *yaml.Node, path tree.Path) (*yaml.Node, error) {
	if node == nil {
		return nil, nil
	}

	if node.Kind == yaml.AliasNode {
		return resolveResetNode(node.Alias, path)
	}

	if node.Tag == "!reset" {
		return nil, nil
	}
	if node.Tag == "!override" {
		return node, nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			resolved, err := resolveResetNode(node.Content[0], path)
			if err != nil {
				return nil, err
			}
			if resolved == nil {
				return nil, nil
			}
			node.Content[0] = resolved
		}
		return node, nil

	case yaml.SequenceNode:
		var nodes []*yaml.Node
		for idx, v := range node.Content {
			resolved, err := resolveResetNode(v, path.Next(fmt.Sprintf("%d", idx)))
			if err != nil {
				return nil, err
			}
			if resolved != nil {
				nodes = append(nodes, resolved)
			}
		}
		node.Content = nodes
		return node, nil

	case yaml.MappingNode:
		var key string
		var nodes []*yaml.Node
		for idx, v := range node.Content {
			if idx%2 == 0 {
				key = v.Value
			} else {
				resolved, err := resolveResetNode(v, path.Next(key))
				if err != nil {
					return nil, err
				}
				if resolved != nil {
					nodes = append(nodes, node.Content[idx-1], resolved)
				}
			}
		}
		node.Content = nodes
		return node, nil
	}

	return node, nil
}

// Resolve materializes the lazy model into a types.Project.
// This is the point where all deferred processing happens:
// extends resolution, includes loading, merging, interpolation,
// type casting, and decoding into Go structs.
func (m *ComposeModel) Resolve() (*types.Project, error) {
	// 1. Process extends on raw nodes (before interpolation, same as existing pipeline)
	if !m.opts.SkipExtends {
		for _, layer := range m.layers {
			if err := m.applyExtendsNode(layer); err != nil {
				return nil, fmt.Errorf("%s: %w", layer.Context.Source, err)
			}
		}
	}

	// 2. Process includes — loads referenced files as additional raw layers
	if !m.opts.SkipInclude {
		if err := m.applyIncludeNodes(); err != nil {
			return nil, err
		}
	}

	// 3. Merge all raw layers into a single tree
	var merged *yaml.Node
	mergedSource := ""
	for _, layer := range m.layers {
		node := resolveDocumentNode(layer.Node)
		if merged == nil {
			merged = node
			mergedSource = layer.Context.Source
			continue
		}
		var err error
		merged, err = override.MergeNodes(merged, node, tree.NewPath())
		if err != nil {
			return nil, fmt.Errorf("merging %s: %w", layer.Context.Source, err)
		}
		mergedSource = layer.Context.Source
	}

	if merged == nil {
		return nil, errors.New("empty compose model")
	}

	// 4. Interpolate and type-cast the merged tree.
	// Each node is interpolated using the environment from its original
	// source file (via nodeContexts). Nodes created during merge inherit
	// context from the nearest registered ancestor.
	if !m.opts.SkipInterpolation {
		defaultCtx := m.layers[0].Context
		if err := m.interpolateTree(merged, tree.NewPath(), defaultCtx); err != nil {
			return nil, err
		}
	}

	// 5. Decode into types.Project
	project := &types.Project{
		Name:        m.opts.projectName,
		WorkingDir:  m.configDetails.WorkingDir,
		Environment: m.configDetails.Environment,
	}

	override.DeleteKey(merged, "name")
	override.DeleteKey(merged, "include")
	override.DeleteKey(merged, "version")

	if err := merged.Decode(project); err != nil {
		err = m.enrichError(err, mergedSource)
		return nil, fmt.Errorf("decoding compose model: %w", err)
	}

	// 6. Apply profiles filter
	var err error
	if project, err = project.WithProfiles(m.opts.Profiles); err != nil {
		return nil, err
	}

	// 7. Consistency check
	if !m.opts.SkipConsistencyCheck {
		if err := checkConsistency(project); err != nil {
			return nil, err
		}
	}

	// 8. Resolve environment
	if !m.opts.SkipResolveEnvironment {
		project, err = project.WithServicesEnvironmentResolved(m.opts.discardEnvFiles)
		if err != nil {
			return nil, err
		}
	}

	project, err = project.WithServicesLabelsResolved(m.opts.discardEnvFiles)
	if err != nil {
		return nil, err
	}

	return project, nil
}

// interpolateTree walks the yaml.Node tree and interpolates scalar values
// using per-node context from nodeContexts. Nodes not found in the map
// inherit context from the nearest registered ancestor during the walk.
func (m *ComposeModel) interpolateTree(node *yaml.Node, path tree.Path, inherited *NodeContext) error {
	if node == nil {
		return nil
	}

	// Use this node's own context if registered, otherwise inherit
	ctx := inherited
	if c, ok := m.nodeContexts[node]; ok {
		ctx = c
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := m.interpolateTree(child, path, ctx); err != nil {
				return err
			}
		}

	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]

			// Determine context for this key-value pair.
			// Prefer the value's own context (it came from a specific layer),
			// then the key's context, then the inherited context.
			pairCtx := ctx
			if c, ok := m.nodeContexts[valNode]; ok {
				pairCtx = c
			} else if c, ok := m.nodeContexts[keyNode]; ok {
				pairCtx = c
			}

			next := path.Next(keyNode.Value)
			if err := m.interpolateTree(valNode, next, pairCtx); err != nil {
				return err
			}
		}

	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := m.interpolateTree(child, path.Next(tree.PathMatchList), ctx); err != nil {
				return err
			}
		}

	case yaml.ScalarNode:
		return m.interpolateScalar(node, path, ctx)
	}

	return nil
}

// interpolateScalar substitutes variables and applies type casting on a single scalar node.
func (m *ComposeModel) interpolateScalar(node *yaml.Node, path tree.Path, ctx *NodeContext) error {
	if node.Tag != "!!str" && node.Tag != "" && !strings.Contains(node.Value, "$") {
		return nil
	}

	lookup := func(key string) (string, bool) {
		if ctx == nil {
			return "", false
		}
		v, ok := ctx.Env[key]
		return v, ok
	}

	newValue, err := template.Substitute(node.Value, template.Mapping(lookup))
	if err != nil {
		return err
	}

	// Type casting based on tree path
	var caster interp.Cast
	for pattern, c := range interpolateTypeCastMapping {
		if path.Matches(pattern) {
			caster = c
			break
		}
	}

	if caster == nil {
		node.Value = newValue
		return nil
	}

	casted, err := caster(newValue)
	if err != nil {
		return types.NodeErrorf(node, "failed to cast to expected type: %v", err)
	}

	switch casted.(type) {
	case bool:
		node.Tag = "!!bool"
	case int, int64:
		node.Tag = "!!int"
	case float64:
		node.Tag = "!!float"
	case nil:
		node.Tag = "!!null"
		node.Value = "null"
		return nil
	}
	node.Value = fmt.Sprint(casted)
	return nil
}

// enrichError adds source file information to error messages.
func (m *ComposeModel) enrichError(err error, fallbackSource string) error {
	if err == nil {
		return nil
	}
	source := fallbackSource
	if source == "" && len(m.layers) > 0 {
		source = m.layers[0].Context.Source
	}
	return types.WithSource(err, source)
}

// applyExtendsNode processes "extends" directives within a single layer's node tree.
func (m *ComposeModel) applyExtendsNode(layer *Layer) error {
	node := resolveDocumentNode(layer.Node)
	_, services := override.FindKey(node, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return nil
	}

	resolved := map[string]bool{}
	for i := 0; i+1 < len(services.Content); i += 2 {
		name := services.Content[i].Value
		if err := m.resolveServiceExtends(layer, services, name, resolved, nil); err != nil {
			return err
		}
	}
	return nil
}

func (m *ComposeModel) resolveServiceExtends(layer *Layer, services *yaml.Node, name string, resolved map[string]bool, chain []string) error {
	if resolved[name] {
		return nil
	}

	// cycle detection
	if slices.Contains(chain, name) {
		return fmt.Errorf("circular extends: %s", strings.Join(append(chain, name), " -> "))
	}
	chain = append(chain, name)

	_, svcNode := override.FindKey(services, name)
	if svcNode == nil {
		return nil
	}

	_, extendsNode := override.FindKey(svcNode, "extends")
	if extendsNode == nil {
		resolved[name] = true
		return nil
	}

	var refService string
	var refFile string

	switch extendsNode.Kind {
	case yaml.ScalarNode:
		refService = extendsNode.Value
	case yaml.MappingNode:
		_, sn := override.FindKey(extendsNode, "service")
		if sn == nil {
			return types.NodeErrorf(extendsNode, "extends requires a 'service' key")
		}
		refService = sn.Value
		_, fn := override.FindKey(extendsNode, "file")
		if fn != nil {
			refFile = fn.Value
		}
	default:
		return types.NodeErrorf(extendsNode, "extends must be a string or mapping")
	}

	var baseService *yaml.Node

	if refFile != "" {
		// Load from external file
		filePath := refFile
		if !filepath.IsAbs(filePath) {
			filePath = filepath.Join(layer.Context.WorkingDir, filePath)
		}
		extNode, err := loadYamlFileNode(types.ConfigFile{Filename: filePath})
		if err != nil {
			return types.WrapNodeError(extendsNode, fmt.Errorf("loading extends file %s: %w", refFile, err))
		}
		if extNode == nil {
			return types.NodeErrorf(extendsNode, "extends file %s is empty", refFile)
		}

		// Register nodes from the external file with their own context
		extCtx := &NodeContext{
			Source:     filePath,
			WorkingDir: filepath.Dir(filePath),
			Env:        layer.Context.Env,
		}
		m.registerNodes(extNode, extCtx)

		extRoot := resolveDocumentNode(extNode)
		_, extServices := override.FindKey(extRoot, "services")
		if extServices == nil {
			return types.NodeErrorf(extendsNode, "extends file %s has no services", refFile)
		}
		_, baseService = override.FindKey(extServices, refService)
		if baseService == nil {
			return types.NodeErrorf(extendsNode, "service %q not found in %s", refService, refFile)
		}

		// Recursively resolve extends in the base service's file
		extResolved := map[string]bool{}
		extLayer := &Layer{Node: extNode, Context: extCtx}
		if err := m.resolveServiceExtends(extLayer, extServices, refService, extResolved, nil); err != nil {
			return err
		}
		// Re-fetch after resolution
		_, baseService = override.FindKey(extServices, refService)
	} else {
		// Same file
		if err := m.resolveServiceExtends(layer, services, refService, resolved, chain); err != nil {
			return err
		}
		_, baseService = override.FindKey(services, refService)
		if baseService == nil {
			return types.NodeErrorf(extendsNode, "service %q not found", refService)
		}
	}

	// Deep clone base before merge to avoid mutating it
	baseClone := m.deepCloneNode(baseService)

	// Merge: base extended by current service
	merged, err := override.ExtendServiceNode(baseClone, svcNode)
	if err != nil {
		return types.WrapNodeError(extendsNode, fmt.Errorf("extending service %s: %w", name, err))
	}

	// Remove "extends" from merged result
	override.DeleteKey(merged, "extends")

	// Replace service node in-place
	override.SetKey(services, name, merged)
	resolved[name] = true
	return nil
}

// deepCloneNode creates a deep copy of a yaml.Node tree,
// propagating node contexts from originals to clones.
func (m *ComposeModel) deepCloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := &yaml.Node{
		Kind:        node.Kind,
		Style:       node.Style,
		Tag:         node.Tag,
		Value:       node.Value,
		Anchor:      node.Anchor,
		HeadComment: node.HeadComment,
		LineComment: node.LineComment,
		FootComment: node.FootComment,
		Line:        node.Line,
		Column:      node.Column,
	}
	// Propagate context from original to clone
	if ctx, ok := m.nodeContexts[node]; ok {
		m.nodeContexts[clone] = ctx
	}
	if node.Alias != nil {
		clone.Alias = m.deepCloneNode(node.Alias)
	}
	if len(node.Content) > 0 {
		clone.Content = make([]*yaml.Node, len(node.Content))
		for i, c := range node.Content {
			clone.Content[i] = m.deepCloneNode(c)
		}
	}
	return clone
}

// applyIncludeNodes processes "include" directives from the layers,
// loading referenced files as additional raw layers with their own context.
func (m *ComposeModel) applyIncludeNodes() error {
	for _, layer := range m.layers {
		node := resolveDocumentNode(layer.Node)
		_, includeNode := override.FindKey(node, "include")
		if includeNode == nil {
			continue
		}
		if includeNode.Kind != yaml.SequenceNode {
			return types.NodeErrorf(includeNode, "include must be a sequence")
		}

		for _, entry := range includeNode.Content {
			includeLayers, err := m.loadIncludeEntry(layer, entry)
			if err != nil {
				return err
			}
			m.layers = append(m.layers, includeLayers...)
		}
	}
	return nil
}

func (m *ComposeModel) loadIncludeEntry(parent *Layer, entry *yaml.Node) ([]*Layer, error) {
	var paths []string
	var projectDir string
	var envFiles []string

	switch entry.Kind {
	case yaml.ScalarNode:
		paths = []string{entry.Value}
	case yaml.MappingNode:
		_, pathNode := override.FindKey(entry, "path")
		if pathNode != nil {
			switch pathNode.Kind {
			case yaml.ScalarNode:
				paths = []string{pathNode.Value}
			case yaml.SequenceNode:
				for _, p := range pathNode.Content {
					paths = append(paths, p.Value)
				}
			}
		}
		_, pdNode := override.FindKey(entry, "project_directory")
		if pdNode != nil {
			projectDir = pdNode.Value
		}
		_, efNode := override.FindKey(entry, "env_file")
		if efNode != nil {
			switch efNode.Kind {
			case yaml.ScalarNode:
				envFiles = []string{efNode.Value}
			case yaml.SequenceNode:
				for _, e := range efNode.Content {
					envFiles = append(envFiles, e.Value)
				}
			}
		}
	default:
		return nil, types.NodeErrorf(entry, "include entry must be a string or mapping")
	}

	if len(paths) == 0 {
		return nil, types.NodeErrorf(entry, "include entry has no path")
	}

	// Resolve paths relative to parent working directory
	for i, p := range paths {
		if !filepath.IsAbs(p) {
			paths[i] = filepath.Join(parent.Context.WorkingDir, p)
		}
	}

	// Determine working directory for the included files
	workDir := projectDir
	if workDir == "" {
		workDir = filepath.Dir(paths[0])
	} else if !filepath.IsAbs(workDir) {
		workDir = filepath.Join(parent.Context.WorkingDir, workDir)
	}

	// Resolve environment: parent env + env_file
	env := parent.Context.Env.Clone()
	if len(envFiles) > 0 {
		for i, f := range envFiles {
			if !filepath.IsAbs(f) {
				envFiles[i] = filepath.Join(parent.Context.WorkingDir, f)
			}
		}
		envFromFile, err := dotenv.GetEnvFromFile(env, envFiles)
		if err != nil {
			return nil, types.WrapNodeError(entry, err)
		}
		env = env.Merge(envFromFile)
	}

	// Load each path as a raw layer with the include-specific context
	var layers []*Layer
	for _, p := range paths {
		node, err := loadYamlFileNode(types.ConfigFile{Filename: p})
		if err != nil {
			return nil, types.WrapNodeError(entry, fmt.Errorf("loading include %s: %w", p, err))
		}
		if node == nil {
			continue
		}

		nodeCtx := &NodeContext{
			Source:     p,
			WorkingDir: workDir,
			Env:        env,
		}
		m.registerNodes(node, nodeCtx)

		incLayer := &Layer{Node: node, Context: nodeCtx}
		if !m.opts.SkipExtends {
			if err := m.applyExtendsNode(incLayer); err != nil {
				return nil, fmt.Errorf("%s: %w", p, err)
			}
		}
		layers = append(layers, incLayer)
	}
	return layers, nil
}

// resolveDocumentNode unwraps a DocumentNode to get the actual mapping node.
func resolveDocumentNode(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}
