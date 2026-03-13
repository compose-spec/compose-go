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
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/sirupsen/logrus"
	"github.com/compose-spec/compose-go/v2/format"
	interp "github.com/compose-spec/compose-go/v2/interpolation"
	"github.com/compose-spec/compose-go/v2/override"
	"github.com/compose-spec/compose-go/v2/schema"
	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/compose-spec/compose-go/v2/validation"
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
	// serviceWorkDirs maps service names to the working directory of the file
	// they were defined in. Used for includes where paths are relative to
	// the included file's directory.
	serviceWorkDirs map[string]string
	// loadedFiles tracks files that have been loaded, for cycle detection.
	loadedFiles []string
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
		serviceWorkDirs: make(map[string]string),
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
		model.loadedFiles = append(model.loadedFiles, file.Filename)
	}

	if len(model.layers) == 0 {
		return nil, errors.New("empty compose file")
	}

	return model, nil
}

// registerNodes associates every node in a tree with the given context.
func (m *ComposeModel) registerNodes(node *yaml.Node, ctx *NodeContext) {
	m.registerNodesVisited(node, ctx, make(map[*yaml.Node]bool))
}

func (m *ComposeModel) registerNodesVisited(node *yaml.Node, ctx *NodeContext, visited map[*yaml.Node]bool) {
	if node == nil || visited[node] {
		return
	}
	visited[node] = true
	m.nodeContexts[node] = ctx
	for _, child := range node.Content {
		m.registerNodesVisited(child, ctx, visited)
	}
	if node.Alias != nil {
		m.registerNodesVisited(node.Alias, ctx, visited)
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
		// Keep !reset and !override tags in the tree — they are handled during merge
		result = &doc
	}
	if result != nil {
		if err := checkDuplicateKeys(result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// checkDuplicateKeys recursively walks a yaml.Node tree and returns an error
// if any MappingNode contains duplicate keys.
func checkDuplicateKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := checkDuplicateKeys(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		keys := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			key := keyNode.Value
			if line, seen := keys[key]; seen {
				return fmt.Errorf("line %d: mapping key %#v already defined at line %d", keyNode.Line, key, line)
			}
			keys[key] = keyNode.Line
			if err := checkDuplicateKeys(node.Content[i+1]); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := checkDuplicateKeys(child); err != nil {
				return err
			}
		}
	}
	return nil
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
				return nil, err
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

	// 3a. Check top-level node is a mapping
	if merged.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("top-level object must be a mapping")
	}

	// 3b. Check for non-string keys
	if err := checkNonStringKeysNode(merged, ""); err != nil {
		return nil, err
	}

	// 3c. Validate project name
	if !m.opts.SkipValidation && m.opts.projectName == "" {
		return nil, errors.New("project name must not be empty")
	}

	// 3d. Strip remaining !reset tags and enforce sequence unicity
	override.StripResetTags(merged)
	override.EnforceUnicityNode(merged, tree.NewPath())

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

	// 4b. Transform deprecated external.name syntax to external: true + name
	if err := transformExternalNodes(merged); err != nil {
		return nil, err
	}

	// 5. Schema validation (on map[string]any representation, for backward compat)
	if !m.opts.SkipValidation {
		var dict map[string]any
		if err := merged.Decode(&dict); err == nil {
			if err := schema.Validate(dict); err != nil {
				return nil, fmt.Errorf("validating %s: %w", mergedSource, err)
			}
			if _, ok := dict["version"]; ok {
				m.opts.warnObsoleteVersion(mergedSource)
			}
		}
		if err := validation.Validate(dict); err != nil {
			return nil, err
		}
	}

	// 6. Decode into types.Project
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

	// 6a. Process known extensions — convert raw extension values to registered Go types
	if len(m.opts.KnownExtensions) > 0 {
		if err := processProjectExtensions(project, m.opts.KnownExtensions); err != nil {
			return nil, err
		}
	}

	// 6b. Set service names from map keys (always, even when SkipNormalization)
	for name, svc := range project.Services {
		svc.Name = name
		project.Services[name] = svc
	}

	// 6c. Validate environment variable whitespace (with path context)
	for name, svc := range project.Services {
		for k := range svc.Environment {
			if k != "" && k[len(k)-1] == ' ' {
				return nil, fmt.Errorf("'services[%s].environment' environment variable %s is declared with a trailing space", name, k)
			}
		}
	}

	// 7. Normalization (default network, resource names, build defaults, etc.)
	if !m.opts.SkipNormalization {
		if err := normalizeProject(project, m.opts); err != nil {
			return nil, err
		}
	}

	// 7a. Always resolve environment references in secrets/configs
	resolveSecretConfigEnvironment(project)

	// 8. Path resolution
	if m.opts.ResolvePaths {
		if err := resolveProjectPaths(project, m.opts); err != nil {
			return nil, err
		}
	}

	// 9. Windows path conversion
	if m.opts.ConvertWindowsPaths {
		for name, svc := range project.Services {
			for j, vol := range svc.Volumes {
				svc.Volumes[j] = convertVolumePath(vol)
			}
			project.Services[name] = svc
		}
	}

	// 10. Apply profiles filter
	var err error
	if project, err = project.WithProfiles(m.opts.Profiles); err != nil {
		return nil, err
	}

	// 11. Consistency check
	if !m.opts.SkipConsistencyCheck {
		if err := checkConsistency(project); err != nil {
			return nil, err
		}
	}

	// 12. Resolve environment
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
// It tries to find the correct source file by matching line numbers
// from the error against nodes tracked in nodeContexts.
func (m *ComposeModel) enrichError(err error, fallbackSource string) error {
	if err == nil {
		return nil
	}
	source := m.findSourceForError(err)
	if source == "" {
		source = fallbackSource
		if source == "" && len(m.layers) > 0 {
			source = m.layers[0].Context.Source
		}
	}
	return types.WithSource(err, source)
}

// findSourceForError extracts line/column numbers from yaml error messages and
// looks up which source file contains a node at that position.
func (m *ComposeModel) findSourceForError(err error) string {
	msg := err.Error()
	// yaml/v4 errors look like: "line N, column M: ..."
	// Try matching with both line and column first for precision
	re := regexp.MustCompile(`line (\d+), column (\d+)`)
	matches := re.FindStringSubmatch(msg)
	if len(matches) >= 3 {
		lineNum, _ := strconv.Atoi(matches[1])
		colNum, _ := strconv.Atoi(matches[2])
		for node, ctx := range m.nodeContexts {
			if node.Line == lineNum && node.Column == colNum {
				return ctx.Source
			}
		}
	}
	// Fallback: match on line only
	reLineOnly := regexp.MustCompile(`line (\d+)`)
	lineMatches := reLineOnly.FindStringSubmatch(msg)
	if len(lineMatches) < 2 {
		return ""
	}
	lineNum, convErr := strconv.Atoi(lineMatches[1])
	if convErr != nil {
		return ""
	}
	for node, ctx := range m.nodeContexts {
		if node.Line == lineNum {
			return ctx.Source
		}
	}
	return ""
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

	// cycle detection using file:service identifiers
	chainID := layer.Context.Source + ":" + name
	if slices.Contains(chain, chainID) {
		return fmt.Errorf("Circular reference with extends")
	}
	chain = append(chain, chainID)

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
		m.opts.ProcessEvent("extends", map[string]any{"service": refService})
	case yaml.MappingNode:
		_, sn := override.FindKey(extendsNode, "service")
		if sn == nil {
			return fmt.Errorf("extends.%s.service is required", name)
		}
		refService = sn.Value
		_, fn := override.FindKey(extendsNode, "file")
		if fn != nil {
			refFile = fn.Value
		}
		metadata := map[string]any{"service": refService}
		if refFile != "" {
			metadata["file"] = refFile
		}
		m.opts.ProcessEvent("extends", metadata)
	default:
		return types.NodeErrorf(extendsNode, "extends must be a string or mapping")
	}

	var baseService *yaml.Node

	if refFile != "" {
		// Load from external file, checking remote resource loaders first
		filePath := refFile
		for _, loader := range m.opts.RemoteResourceLoaders() {
			if loader.Accept(refFile) {
				resolved, loadErr := loader.Load(context.TODO(), refFile)
				if loadErr != nil {
					return loadErr
				}
				filePath = resolved
				break
			}
		}
		if filePath == refFile && !filepath.IsAbs(filePath) {
			filePath = filepath.Join(layer.Context.WorkingDir, filePath)
		}
		extNode, err := loadYamlFileNode(types.ConfigFile{Filename: filePath})
		if err != nil {
			return types.WrapNodeError(extendsNode, fmt.Errorf("loading extends file %s: %w", refFile, err))
		}
		if extNode == nil {
			return types.NodeErrorf(extendsNode, "extends file %s is empty", refFile)
		}

		extDir := filepath.Dir(filePath)

		// Register nodes from the external file with their own context
		extCtx := &NodeContext{
			Source:     filePath,
			WorkingDir: extDir,
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
		if err := m.resolveServiceExtends(extLayer, extServices, refService, extResolved, chain); err != nil {
			return err
		}
		// Re-fetch after resolution
		_, baseService = override.FindKey(extServices, refService)

		// Resolve relative paths in the base service using the extends file's
		// relative directory, so paths are expressed relative to the main project dir.
		relWorkDir, err := filepath.Rel(layer.Context.WorkingDir, extDir)
		if err != nil {
			relWorkDir = extDir
		}
		resolveServiceNodePaths(baseService, relWorkDir)
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
// Includes are inserted BEFORE their parent layer so the parent takes
// precedence during merge (matching the old pipeline's behavior).
func (m *ComposeModel) applyIncludeNodes() error {
	var newLayers []*Layer
	for _, layer := range m.layers {
		node := resolveDocumentNode(layer.Node)
		_, includeNode := override.FindKey(node, "include")
		if includeNode == nil {
			newLayers = append(newLayers, layer)
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
			// Prepend includes before parent so parent overrides them
			newLayers = append(newLayers, includeLayers...)
		}
		newLayers = append(newLayers, layer)
	}
	m.layers = newLayers
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

	// Resolve paths: check remote resource loaders first, then resolve relative to parent
	for i, p := range paths {
		resolved := false
		for _, loader := range m.opts.RemoteResourceLoaders() {
			if loader.Accept(p) {
				absPath, loadErr := loader.Load(context.TODO(), p)
				if loadErr != nil {
					return nil, types.WrapNodeError(entry, fmt.Errorf("loading include %s: %w", p, loadErr))
				}
				paths[i] = absPath
				resolved = true
				break
			}
		}
		if !resolved && !filepath.IsAbs(p) {
			paths[i] = filepath.Join(parent.Context.WorkingDir, p)
		}
	}

	// Cycle detection
	for _, p := range paths {
		for _, loaded := range m.loadedFiles {
			if loaded == p {
				m.loadedFiles = append(m.loadedFiles, p)
				return nil, fmt.Errorf("include cycle detected:\n%s\n include %s",
					m.loadedFiles[0], strings.Join(m.loadedFiles[1:], "\n include "))
			}
		}
	}

	// Determine working directory for the included files (absolute)
	workDir := projectDir
	if workDir == "" {
		workDir = filepath.Dir(paths[0])
	} else if !filepath.IsAbs(workDir) {
		workDir = filepath.Join(parent.Context.WorkingDir, workDir)
	}

	// Compute relative working dir from main project dir to include dir.
	// This is used to resolve paths in the included file so they are
	// expressed relative to the main project directory.
	mainWorkDir := m.configDetails.WorkingDir
	relWorkDir, err := filepath.Rel(mainWorkDir, workDir)
	if err != nil {
		relWorkDir = workDir
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
		m.loadedFiles = append(m.loadedFiles, p)
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

		// Resolve relative paths in the included layer using the relative
		// working directory, so paths are expressed relative to the main
		// project directory (matching the old pipeline's behavior).
		resolveLayerNodePaths(node, relWorkDir)

		// Resolve bare environment variables (e.g., "VAR_NAME" without "=")
		// using the include's environment, so they survive merge correctly.
		resolveLayerEnvironment(node, env)

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

// resolveLayerEnvironment resolves bare environment variable references
// (entries like "VAR_NAME" without "=") in all services within a layer,
// using the given environment mapping. This must be done before merge so that
// include-specific environment variables are correctly resolved.
func resolveLayerEnvironment(node *yaml.Node, env types.Mapping) {
	root := resolveDocumentNode(node)
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	_, services := override.FindKey(root, "services")
	if services == nil || services.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(services.Content); i += 2 {
		svc := services.Content[i+1]
		if svc == nil || svc.Kind != yaml.MappingNode {
			continue
		}
		_, envNode := override.FindKey(svc, "environment")
		if envNode == nil || envNode.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range envNode.Content {
			if item.Kind != yaml.ScalarNode {
				continue
			}
			// Only process bare variable names (no "=" sign)
			if !strings.Contains(item.Value, "=") {
				if val, ok := env[item.Value]; ok {
					item.Value = fmt.Sprintf("%s=%s", item.Value, val)
				}
			}
		}
	}
}

// addBuildContextDefault ensures a build mapping has a "context" key.
// If the build is a mapping without a context, "." is added as default.
// This must be called before path resolution for includes, so the default
// context gets resolved relative to the include's working directory.
func addBuildContextDefault(svc *yaml.Node) {
	if svc == nil || svc.Kind != yaml.MappingNode {
		return
	}
	_, build := override.FindKey(svc, "build")
	if build == nil || build.Kind != yaml.MappingNode {
		return
	}
	_, ctx := override.FindKey(build, "context")
	if ctx == nil {
		override.SetKey(build, "context", override.NewScalar("."))
	}
}

// resolveServiceNodePaths adjusts relative paths in a service yaml.Node to be
// expressed relative to workDir. This is used for extends from external files
// to make paths relative to the main project directory.
func resolveServiceNodePaths(svc *yaml.Node, workDir string) {
	if svc == nil || svc.Kind != yaml.MappingNode || workDir == "." {
		return
	}

	absNodePath := func(p string) string {
		if filepath.IsAbs(p) || p == "" {
			return p
		}
		return filepath.Join(workDir, p)
	}

	// build.context
	_, build := override.FindKey(svc, "build")
	if build != nil {
		if build.Kind == yaml.ScalarNode {
			// short syntax: build: ./path
			if !strings.Contains(build.Value, "://") {
				build.Value = absNodePath(build.Value)
			}
		} else if build.Kind == yaml.MappingNode {
			_, ctx := override.FindKey(build, "context")
			if ctx != nil && ctx.Kind == yaml.ScalarNode && !strings.Contains(ctx.Value, "://") {
				ctx.Value = absNodePath(ctx.Value)
			}
			_, addCtx := override.FindKey(build, "additional_contexts")
			if addCtx != nil && addCtx.Kind == yaml.MappingNode {
				for i := 0; i+1 < len(addCtx.Content); i += 2 {
					v := addCtx.Content[i+1]
					if v.Kind == yaml.ScalarNode && !strings.Contains(v.Value, "://") {
						v.Value = absNodePath(v.Value)
					}
				}
			}
		}
	}

	// env_file
	_, envFile := override.FindKey(svc, "env_file")
	if envFile != nil {
		resolveEnvFileNodePaths(envFile, absNodePath)
	}

	// label_file
	_, labelFile := override.FindKey(svc, "label_file")
	if labelFile != nil && labelFile.Kind == yaml.SequenceNode {
		for _, item := range labelFile.Content {
			if item.Kind == yaml.ScalarNode {
				item.Value = absNodePath(item.Value)
			}
		}
	}

	// volumes (only for bind mount sources that are relative paths)
	_, volumes := override.FindKey(svc, "volumes")
	if volumes != nil && volumes.Kind == yaml.SequenceNode {
		for i, item := range volumes.Content {
			if item.Kind == yaml.MappingNode {
				_, vtype := override.FindKey(item, "type")
				if vtype != nil && vtype.Value == "bind" {
					_, src := override.FindKey(item, "source")
					if src != nil && src.Kind == yaml.ScalarNode {
						src.Value = absNodePath(src.Value)
					}
				}
			} else if item.Kind == yaml.ScalarNode {
				// Short syntax: parse, resolve source, convert to long syntax mapping
				vol, err := format.ParseVolume(item.Value)
				if err == nil && vol.Type == types.VolumeTypeBind && vol.Source != "" && !filepath.IsAbs(vol.Source) {
					vol.Source = absNodePath(vol.Source)
					// Convert to long syntax mapping node to preserve bind type
					trueNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
					bindNode := override.NewMapping(override.KeyValue{
						Key: "create_host_path", Value: trueNode,
					})
					pairs := []override.KeyValue{
						{Key: "type", Value: override.NewScalar("bind")},
						{Key: "source", Value: override.NewScalar(vol.Source)},
						{Key: "target", Value: override.NewScalar(vol.Target)},
						{Key: "bind", Value: bindNode},
					}
					if vol.ReadOnly {
						pairs = append(pairs, override.KeyValue{Key: "read_only", Value: override.NewScalar("true")})
					}
					volumes.Content[i] = override.NewMapping(pairs...)
				}
			}
		}
	}

	// extends.file
	_, extends := override.FindKey(svc, "extends")
	if extends != nil && extends.Kind == yaml.MappingNode {
		_, file := override.FindKey(extends, "file")
		if file != nil && file.Kind == yaml.ScalarNode {
			file.Value = absNodePath(file.Value)
		}
	}

	// develop.watch[].path
	_, develop := override.FindKey(svc, "develop")
	if develop != nil && develop.Kind == yaml.MappingNode {
		_, watch := override.FindKey(develop, "watch")
		if watch != nil && watch.Kind == yaml.SequenceNode {
			for _, item := range watch.Content {
				if item.Kind == yaml.MappingNode {
					_, p := override.FindKey(item, "path")
					if p != nil && p.Kind == yaml.ScalarNode {
						p.Value = absNodePath(p.Value)
					}
				}
			}
		}
	}
}

// resolveEnvFileNodePaths adjusts env_file paths using the given abs function.
func resolveEnvFileNodePaths(node *yaml.Node, absPath func(string) string) {
	switch node.Kind {
	case yaml.ScalarNode:
		node.Value = absPath(node.Value)
	case yaml.SequenceNode:
		for _, item := range node.Content {
			switch item.Kind {
			case yaml.ScalarNode:
				item.Value = absPath(item.Value)
			case yaml.MappingNode:
				_, p := override.FindKey(item, "path")
				if p != nil && p.Kind == yaml.ScalarNode {
					p.Value = absPath(p.Value)
				}
			}
		}
	}
}

// transformExternalNodes processes deprecated external.name syntax in volumes,
// networks, secrets, and configs. Converts `external: {name: foo}` to
// `external: true` and sets the name key on the parent resource.
func transformExternalNodes(node *yaml.Node) error {
	for _, section := range []string{"volumes", "networks", "secrets", "configs"} {
		_, sectionNode := override.FindKey(node, section)
		if sectionNode == nil || sectionNode.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(sectionNode.Content); i += 2 {
			resourceKey := sectionNode.Content[i].Value
			resource := sectionNode.Content[i+1]
			if resource == nil || resource.Kind != yaml.MappingNode {
				continue
			}
			_, extNode := override.FindKey(resource, "external")
			if extNode == nil || extNode.Kind != yaml.MappingNode {
				continue
			}
			// external is a mapping — deprecated syntax
			_, extNameNode := override.FindKey(extNode, "name")
			if extNameNode != nil && extNameNode.Kind == yaml.ScalarNode {
				extName := extNameNode.Value
				_, nameNode := override.FindKey(resource, "name")
				p := tree.NewPath(section, resourceKey)
				logrus.Warnf("%s: external.name is deprecated. Please set name and external: true", p)
				if nameNode != nil && nameNode.Kind == yaml.ScalarNode && nameNode.Value != extName {
					return fmt.Errorf("%s: name and external.name conflict; only use name", p)
				}
				if nameNode == nil {
					override.SetKey(resource, "name", override.NewScalar(extName))
				}
			}
			// Replace external mapping with scalar true
			extNode.Kind = yaml.ScalarNode
			extNode.Tag = "!!bool"
			extNode.Value = "true"
			extNode.Content = nil
		}
	}
	return nil
}

// checkNonStringKeysNode walks a yaml.Node tree and returns an error if any
// mapping key is not a string (e.g., an integer key like `123: value`).
func checkNonStringKeysNode(node *yaml.Node, keyPrefix string) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			if keyNode.Tag != "" && keyNode.Tag != "!!str" && keyNode.Tag != "!!merge" {
				var location string
				if keyPrefix == "" {
					location = "at top level"
				} else {
					location = fmt.Sprintf("in %s", keyPrefix)
				}
				return fmt.Errorf("non-string key %s: %s", location, keyNode.Value)
			}
			var childPrefix string
			if keyPrefix == "" {
				childPrefix = keyNode.Value
			} else {
				childPrefix = fmt.Sprintf("%s.%s", keyPrefix, keyNode.Value)
			}
			if err := checkNonStringKeysNode(valNode, childPrefix); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for idx, item := range node.Content {
			childPrefix := fmt.Sprintf("%s[%d]", keyPrefix, idx)
			if err := checkNonStringKeysNode(item, childPrefix); err != nil {
				return err
			}
		}
	}
	return nil
}

// resolveLayerNodePaths resolves all relative paths in a layer's node tree
// using the given working directory. This makes paths absolute so they survive
// merging into the main project with a different working directory.
func resolveLayerNodePaths(node *yaml.Node, workDir string) {
	root := resolveDocumentNode(node)
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}

	absPath := func(p string) string {
		if filepath.IsAbs(p) || p == "" {
			return p
		}
		return filepath.Join(workDir, p)
	}

	_, services := override.FindKey(root, "services")
	if services != nil && services.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(services.Content); i += 2 {
			svc := services.Content[i+1]
			// Add build.context default before path resolution so it gets
			// resolved relative to the include's working directory.
			addBuildContextDefault(svc)
			resolveServiceNodePaths(svc, workDir)
		}
	}

	// configs.*.file
	_, configs := override.FindKey(root, "configs")
	if configs != nil && configs.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(configs.Content); i += 2 {
			cfg := configs.Content[i+1]
			if cfg.Kind == yaml.MappingNode {
				_, file := override.FindKey(cfg, "file")
				if file != nil && file.Kind == yaml.ScalarNode {
					file.Value = absPath(file.Value)
				}
			}
		}
	}

	// secrets.*.file
	_, secrets := override.FindKey(root, "secrets")
	if secrets != nil && secrets.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(secrets.Content); i += 2 {
			sec := secrets.Content[i+1]
			if sec.Kind == yaml.MappingNode {
				_, file := override.FindKey(sec, "file")
				if file != nil && file.Kind == yaml.ScalarNode {
					file.Value = absPath(file.Value)
				}
			}
		}
	}
}

// processProjectExtensions converts raw extension values to registered Go types.
// This mirrors the old processExtensions that operated on map[string]any.
func processProjectExtensions(project *types.Project, known map[string]any) error {
	convertExtensions := func(ext types.Extensions) error {
		for name, val := range ext {
			typ, ok := known[name]
			if !ok {
				continue
			}
			target := reflect.New(reflect.TypeOf(typ)).Interface()
			if err := Transform(val, target); err != nil {
				return fmt.Errorf("converting extension %s: %w", name, err)
			}
			ext[name] = reflect.ValueOf(target).Elem().Interface()
		}
		return nil
	}

	if err := convertExtensions(project.Extensions); err != nil {
		return err
	}
	for name, svc := range project.Services {
		if err := convertExtensions(svc.Extensions); err != nil {
			return err
		}
		project.Services[name] = svc
	}
	for name, net := range project.Networks {
		if err := convertExtensions(net.Extensions); err != nil {
			return err
		}
		project.Networks[name] = net
	}
	for name, vol := range project.Volumes {
		if err := convertExtensions(vol.Extensions); err != nil {
			return err
		}
		project.Volumes[name] = vol
	}
	return nil
}
