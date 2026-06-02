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

	"github.com/compose-spec/compose-go/v3/consts"
	"github.com/compose-spec/compose-go/v3/errdefs"
	interp "github.com/compose-spec/compose-go/v3/interpolation"
	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/paths"
	"github.com/compose-spec/compose-go/v3/schema"
	"github.com/compose-spec/compose-go/v3/template"
	"github.com/compose-spec/compose-go/v3/transform"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
	"github.com/compose-spec/compose-go/v3/validation"
	"github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v4"
)

// Options supported by Load
type Options struct {
	// Skip schema validation
	SkipValidation bool
	// Skip interpolation
	SkipInterpolation bool
	// Skip normalization
	SkipNormalization bool
	// Resolve path
	ResolvePaths bool
	// Convert Windows path
	ConvertWindowsPaths bool
	// Skip consistency check
	SkipConsistencyCheck bool
	// Skip extends
	SkipExtends bool
	// SkipInclude will ignore `include` and only load model from file(s) set by ConfigDetails
	SkipInclude bool
	// SkipResolveEnvironment will ignore computing `environment` for services
	SkipResolveEnvironment bool
	// SkipDefaultValues will ignore missing required attributes
	SkipDefaultValues bool
	// Interpolation options
	Interpolate *interp.Options
	// Discard 'env_file' entries after resolving to 'environment' section
	discardEnvFiles bool
	// Set project projectName
	projectName string
	// Indicates when the projectName was imperatively set or guessed from path
	projectNameImperativelySet bool
	// Profiles set profiles to enable
	Profiles []string
	// SelectedServices restricts the project model to these services (and their dependencies)
	// after parsing. An empty slice means "all services". When set, services not in the list
	// are dropped from the project before environment resolution, so their env_file / label_file
	// entries are not loaded.
	SelectedServices []string
	// PruneUnnecessaryResources drops networks/volumes/secrets/configs/models that are not
	// referenced by active services after service selection.
	PruneUnnecessaryResources bool
	// ResourceLoaders manages support for remote resources
	ResourceLoaders []ResourceLoader
	// KnownExtensions manages x-* attribute we know and the corresponding go structs
	KnownExtensions map[string]any
	// Metada for telemetry
	Listeners []Listener
	// MaxNodeVisits caps total YAML node visits during reset/override resolution.
	// Zero means use the default. Useful for very large compose files that exceed the default cap.
	MaxNodeVisits int

	// envFileScopes captures, during v3 LoadV3, the layer Environment in
	// effect when each env_file entry was declared. The map is keyed by
	// the resolved absolute env_file path and consumed by ModelToProject
	// to populate EnvFile.Env, which WithServicesEnvironmentResolved
	// then uses as the preferred interpolation scope.
	envFileScopes map[string]types.Mapping
}

var versionWarning []string

func (o *Options) warnObsoleteVersion(file string) {
	if !slices.Contains(versionWarning, file) {
		logrus.Warning(fmt.Sprintf("%s: the attribute `version` is obsolete, it will be ignored, please remove it to avoid potential confusion", file))
	}
	versionWarning = append(versionWarning, file)
}

type Listener = func(event string, metadata map[string]any)

// Invoke all listeners for an event
func (o *Options) ProcessEvent(event string, metadata map[string]any) {
	for _, l := range o.Listeners {
		l(event, metadata)
	}
}

// ResourceLoader is a plugable remote resource resolver
type ResourceLoader interface {
	// Accept returns `true` is the resource reference matches ResourceLoader supported protocol(s)
	Accept(path string) bool
	// Load returns the path to a local copy of remote resource identified by `path`.
	Load(ctx context.Context, path string) (string, error)
	// Dir computes path to resource"s parent folder, made relative if possible
	Dir(path string) string
}

// RemoteResourceLoaders excludes localResourceLoader from ResourceLoaders
func (o Options) RemoteResourceLoaders() []ResourceLoader {
	var loaders []ResourceLoader
	for i, loader := range o.ResourceLoaders {
		if _, ok := loader.(localResourceLoader); ok {
			if i != len(o.ResourceLoaders)-1 {
				logrus.Warning("misconfiguration of ResourceLoaders: localResourceLoader should be last")
			}
			continue
		}
		loaders = append(loaders, loader)
	}
	return loaders
}

type localResourceLoader struct {
	WorkingDir string
}

func (l localResourceLoader) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(l.WorkingDir, p)
}

func (l localResourceLoader) Accept(_ string) bool {
	// LocalResourceLoader is the last loader tested so it always should accept the config and try to get the content.
	return true
}

func (l localResourceLoader) Load(_ context.Context, p string) (string, error) {
	return l.abs(p), nil
}

func (l localResourceLoader) Dir(originalPath string) string {
	path := l.abs(originalPath)
	if !l.isDir(path) {
		path = l.abs(filepath.Dir(originalPath))
	}
	rel, err := filepath.Rel(l.WorkingDir, path)
	if err != nil {
		return path
	}
	return rel
}

func (l localResourceLoader) isDir(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

func (o *Options) clone() *Options {
	return &Options{
		SkipValidation:             o.SkipValidation,
		SkipInterpolation:          o.SkipInterpolation,
		SkipNormalization:          o.SkipNormalization,
		ResolvePaths:               o.ResolvePaths,
		ConvertWindowsPaths:        o.ConvertWindowsPaths,
		SkipConsistencyCheck:       o.SkipConsistencyCheck,
		SkipExtends:                o.SkipExtends,
		SkipInclude:                o.SkipInclude,
		Interpolate:                o.Interpolate,
		discardEnvFiles:            o.discardEnvFiles,
		projectName:                o.projectName,
		projectNameImperativelySet: o.projectNameImperativelySet,
		Profiles:                   o.Profiles,
		SelectedServices:           o.SelectedServices,
		PruneUnnecessaryResources:  o.PruneUnnecessaryResources,
		ResourceLoaders:            o.ResourceLoaders,
		KnownExtensions:            o.KnownExtensions,
		Listeners:                  o.Listeners,
	}
}

func (o *Options) SetProjectName(name string, imperativelySet bool) {
	o.projectName = name
	o.projectNameImperativelySet = imperativelySet
}

func (o Options) GetProjectName() (string, bool) {
	return o.projectName, o.projectNameImperativelySet
}

// serviceRef identifies a reference to a service. It's used to detect cyclic
// references in "extends".
type serviceRef struct {
	filename string
	service  string
}

type cycleTracker struct {
	loaded []serviceRef
}

func (ct *cycleTracker) Add(filename, service string) (*cycleTracker, error) {
	toAdd := serviceRef{filename: filename, service: service}
	for _, loaded := range ct.loaded {
		if toAdd == loaded {
			// Create an error message of the form:
			// Circular reference:
			//   service-a in docker-compose.yml
			//   extends service-b in docker-compose.yml
			//   extends service-a in docker-compose.yml
			errLines := []string{
				"Circular reference:",
				fmt.Sprintf("  %s in %s", ct.loaded[0].service, ct.loaded[0].filename),
			}
			for _, service := range append(ct.loaded[1:], toAdd) {
				errLines = append(errLines, fmt.Sprintf("  extends %s in %s", service.service, service.filename))
			}

			return nil, errors.New(strings.Join(errLines, "\n"))
		}
	}

	var branch []serviceRef
	branch = append(branch, ct.loaded...)
	branch = append(branch, toAdd)
	return &cycleTracker{
		loaded: branch,
	}, nil
}

// WithDiscardEnvFiles sets the Options to discard the `env_file` section after resolving to
// the `environment` section
func WithDiscardEnvFiles(opts *Options) {
	opts.discardEnvFiles = true
}

// WithSkipValidation sets the Options to skip validation when loading sections
func WithSkipValidation(opts *Options) {
	opts.SkipValidation = true
}

// WithProfiles sets profiles to be activated
func WithProfiles(profiles []string) func(*Options) {
	return func(opts *Options) {
		opts.Profiles = profiles
	}
}

// WithSelectedServices restricts the loaded project to the given services and their
// dependencies. An empty slice means "all services". When set, services not in the
// list are dropped from the project before environment resolution: their `env_file`
// and `label_file` entries will not be loaded from disk.
func WithSelectedServices(services []string) func(*Options) {
	return func(opts *Options) {
		opts.SelectedServices = services
	}
}

// WithoutUnnecessaryResources drops networks/volumes/secrets/configs/models that
// are not referenced by services remaining after selection.
func WithoutUnnecessaryResources(opts *Options) {
	opts.PruneUnnecessaryResources = true
}

// PostProcessor is used to tweak compose model based on metadata extracted during yaml Unmarshal phase
// that hardly can be implemented using go-yaml and mapstructure
type PostProcessor interface {
	// Apply changes to compose model based on recorder metadata
	Apply(interface{}) error
}

type NoopPostProcessor struct{}

func (NoopPostProcessor) Apply(interface{}) error { return nil }

// LoadConfigFiles ingests config files with ResourceLoader and returns config details with paths to local copies
func LoadConfigFiles(ctx context.Context, configFiles []string, workingDir string, options ...func(*Options)) (*types.ConfigDetails, error) {
	if len(configFiles) < 1 {
		return &types.ConfigDetails{}, fmt.Errorf("no configuration file provided: %w", errdefs.ErrNotFound)
	}

	opts := &Options{}
	config := &types.ConfigDetails{
		ConfigFiles: make([]types.ConfigFile, len(configFiles)),
	}

	for _, op := range options {
		op(opts)
	}
	opts.ResourceLoaders = append(opts.ResourceLoaders, localResourceLoader{})

	for i, p := range configFiles {
		if p == "-" {
			config.ConfigFiles[i] = types.ConfigFile{
				Filename: p,
			}
			continue
		}

		for _, loader := range opts.ResourceLoaders {
			_, isLocalResourceLoader := loader.(localResourceLoader)
			if !loader.Accept(p) {
				continue
			}
			local, err := loader.Load(ctx, p)
			if err != nil {
				return nil, err
			}
			if config.WorkingDir == "" && !isLocalResourceLoader {
				config.WorkingDir = filepath.Dir(local)
			}
			abs, err := filepath.Abs(local)
			if err != nil {
				abs = local
			}
			config.ConfigFiles[i] = types.ConfigFile{
				Filename: abs,
			}
			break
		}
	}
	if config.WorkingDir == "" {
		config.WorkingDir = workingDir
	}
	return config, nil
}

// LoadWithContext reads a ConfigDetails and returns a fully loaded configuration as a compose-go Project
func LoadWithContext(ctx context.Context, configDetails types.ConfigDetails, options ...func(*Options)) (*types.Project, error) {
	opts := ToOptions(&configDetails, options)
	if len(configDetails.ConfigFiles) < 1 {
		return nil, errors.New("no compose file specified")
	}
	root, err := LoadV3(ctx, configDetails, opts)
	if err != nil {
		return nil, err
	}
	return nodeToProject(root, opts, configDetails)
}

// LoadModelWithContext reads a ConfigDetails and returns a fully loaded configuration as a yaml dictionary
func LoadModelWithContext(ctx context.Context, configDetails types.ConfigDetails, options ...func(*Options)) (map[string]any, error) {
	opts := ToOptions(&configDetails, options)
	if len(configDetails.ConfigFiles) < 1 {
		return nil, errors.New("no compose file specified")
	}
	root, err := LoadV3(ctx, configDetails, opts)
	if err != nil {
		return nil, err
	}
	return nodeToModel(root)
}

func ToOptions(configDetails *types.ConfigDetails, options []func(*Options)) *Options {
	opts := &Options{
		Interpolate: &interp.Options{
			Substitute:      template.Substitute,
			LookupValue:     configDetails.LookupEnv,
			TypeCastMapping: interpolateTypeCastMapping,
		},
		ResolvePaths: true,
	}

	for _, op := range options {
		op(opts)
	}
	opts.ResourceLoaders = append(opts.ResourceLoaders, localResourceLoader{configDetails.WorkingDir})
	return opts
}

func loadYamlModel(ctx context.Context, config types.ConfigDetails, opts *Options, ct *cycleTracker, included []string) (map[string]interface{}, error) {
	var (
		dict = map[string]interface{}{}
		err  error
	)
	workingDir, environment := config.WorkingDir, config.Environment

	for _, file := range config.ConfigFiles {
		dict, _, err = loadYamlFile(ctx, file, opts, workingDir, environment, ct, dict, included)
		if err != nil {
			return nil, err
		}
	}

	if !opts.SkipDefaultValues {
		dict, err = transform.SetDefaultValues(dict)
		if err != nil {
			return nil, err
		}
	}

	if !opts.SkipValidation {
		if err := validation.Validate(dict); err != nil {
			return nil, err
		}
	}

	if opts.ResolvePaths {
		var remotes []paths.RemoteResource
		for _, loader := range opts.RemoteResourceLoaders() {
			remotes = append(remotes, loader.Accept)
		}
		err = paths.ResolveRelativePaths(dict, config.WorkingDir, remotes)
		if err != nil {
			return nil, err
		}
	}
	ResolveEnvironment(dict, config.Environment)

	return dict, nil
}

func loadYamlFile(ctx context.Context,
	file types.ConfigFile,
	opts *Options,
	workingDir string,
	environment types.Mapping,
	ct *cycleTracker,
	dict map[string]interface{},
	included []string,
) (map[string]interface{}, PostProcessor, error) {
	ctx = context.WithValue(ctx, consts.ComposeFileKey{}, file.Filename)
	if file.Content == nil && file.Config == nil {
		content, err := os.ReadFile(file.Filename)
		if err != nil {
			return nil, nil, err
		}
		file.Content = content
	}

	processRawYaml := func(raw interface{}, processor PostProcessor) error {
		converted, err := convertToStringKeysRecursive(raw, "")
		if err != nil {
			return err
		}
		cfg, ok := converted.(map[string]interface{})
		if !ok {
			return errors.New("top-level object must be a mapping")
		}

		if opts.Interpolate != nil && !opts.SkipInterpolation {
			cfg, err = interp.Interpolate(cfg, *opts.Interpolate)
			if err != nil {
				return err
			}
		}

		fixEmptyNotNull(cfg)

		// Process includes first so that extended services have all merged attributes
		if !opts.SkipInclude {
			included = append(included, file.Filename)
			err = ApplyInclude(ctx, workingDir, environment, cfg, opts, included, processor)
			if err != nil {
				return err
			}
		}

		if err := processor.Apply(dict); err != nil {
			return err
		}

		// Process extends after includes so base services are fully merged
		if !opts.SkipExtends {
			err = ApplyExtends(ctx, cfg, opts, ct, processor)
			if err != nil {
				return err
			}

		}

		dict, err = override.Merge(dict, cfg)
		if err != nil {
			return err
		}

		dict, err = override.EnforceUnicity(dict)
		if err != nil {
			return err
		}

		if !opts.SkipValidation {
			if err := schema.Validate(dict); err != nil {
				return fmt.Errorf("validating %s: %w", file.Filename, err)
			}
			if _, ok := dict["version"]; ok {
				opts.warnObsoleteVersion(file.Filename)
				delete(dict, "version")
			}
		}

		dict, err = transform.Canonical(dict, opts.SkipInterpolation)
		if err != nil {
			return err
		}

		dict = OmitEmpty(dict)

		// Canonical transformation can reveal duplicates, typically as ports can be a range and conflict with an override
		dict, err = override.EnforceUnicity(dict)
		return err
	}

	var processor PostProcessor
	if file.Config == nil {
		r := bytes.NewReader(file.Content)
		decoder := yaml.NewDecoder(r)
		for {
			var raw interface{}
			reset := &ResetProcessor{target: &raw, maxNodeVisits: opts.MaxNodeVisits}
			err := decoder.Decode(reset)
			if err != nil && errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse %s: %w", file.Filename, err)
			}
			processor = reset
			if err := processRawYaml(raw, processor); err != nil {
				return nil, nil, err
			}
		}
	} else {
		if err := processRawYaml(file.Config, NoopPostProcessor{}); err != nil {
			return nil, nil, err
		}
	}
	return dict, processor, nil
}

// load is the v2 map-based pipeline. Kept available behind the LoadV3
// cutover so individual tests that still need v2 semantics can opt in
// during the v3 transition window.
//
//nolint:unused
func load(ctx context.Context, configDetails types.ConfigDetails, opts *Options, loaded []string) (map[string]interface{}, error) {
	mainFile := configDetails.ConfigFiles[0].Filename
	for _, f := range loaded {
		if f == mainFile {
			loaded = append(loaded, mainFile)
			return nil, fmt.Errorf("include cycle detected:\n%s\n include %s", loaded[0], strings.Join(loaded[1:], "\n include "))
		}
	}

	dict, err := loadYamlModel(ctx, configDetails, opts, &cycleTracker{}, nil)
	if err != nil {
		return nil, err
	}

	if len(dict) == 0 {
		return nil, errors.New("empty compose file")
	}

	if !opts.SkipValidation && opts.projectName == "" {
		return nil, errors.New("project name must not be empty")
	}

	if !opts.SkipNormalization {
		dict["name"] = opts.projectName
		dict, err = Normalize(dict, configDetails.Environment)
		if err != nil {
			return nil, err
		}
	}

	return dict, nil
}

// nodeToProject decodes the canonical merged yaml.Node directly into a
// *types.Project (no intermediate map[string]any) and applies the
// project-level post-decode passes: env_file declaring-scope side-table
// for lazy interpolation, Windows path conversion, profile / service
// selection, services environment + label resolution. Runs the
// equivalent of v2 ModelToProject without the map -> mapstructure
// detour.
func nodeToProject(root *yaml.Node, opts *Options, configDetails types.ConfigDetails) (*types.Project, error) {
	project := &types.Project{
		Name:        opts.projectName,
		WorkingDir:  configDetails.WorkingDir,
		Environment: configDetails.Environment,
	}

	// The project name comes from opts.projectName (set by projectName()
	// during the LoadV3 prologue with the first ConfigFile's `name:`
	// folded in). Strip any `name` scalar from the tree before decode so
	// it does not silently overwrite the value the loader has already
	// canonicalized.
	deleteMappingKey(root, "name")

	if err := root.Decode(project); err != nil {
		return nil, fmt.Errorf("decode project: %w", err)
	}

	// Decode KnownExtensions into their declared target types. The yaml
	// inline tag has parked them as map[string]any under Extensions; this
	// pass swaps each known x-* entry for the typed value the caller
	// registered.
	if err := decodeKnownExtensions(project, opts.KnownExtensions); err != nil {
		return nil, err
	}

	for path, env := range opts.envFileScopes {
		project.SetEnvFileScope(path, env)
	}

	if opts.ConvertWindowsPaths {
		for i, service := range project.Services {
			for j, volume := range service.Volumes {
				service.Volumes[j] = convertVolumePath(volume)
			}
			project.Services[i] = service
		}
	}

	var err error
	if project, err = project.WithProfiles(opts.Profiles); err != nil {
		return nil, err
	}

	if !opts.SkipConsistencyCheck {
		if err := checkConsistency(project); err != nil {
			return nil, err
		}
	}

	if len(opts.SelectedServices) > 0 {
		project, err = project.WithServicesEnabled(opts.SelectedServices...)
		if err != nil {
			return nil, err
		}
		project, err = project.WithSelectedServices(opts.SelectedServices)
		if err != nil {
			return nil, err
		}
	}

	if opts.PruneUnnecessaryResources {
		project = project.WithoutUnnecessaryResources()
	}

	if !opts.SkipResolveEnvironment {
		project, err = project.WithServicesEnvironmentResolved(opts.discardEnvFiles)
		if err != nil {
			return nil, err
		}
	}

	project, err = project.WithServicesLabelsResolved(opts.discardEnvFiles)
	if err != nil {
		return nil, err
	}

	return project, nil
}

// decodeKnownExtensions walks Project.Extensions and every typed
// container's Extensions map looking for keys the caller registered via
// Options.KnownExtensions. Each match has its raw map[string]any value
// re-decoded into the declared target type via a yaml round-trip so the
// caller gets the strongly-typed value back at p.Extensions[name].
func decodeKnownExtensions(project *types.Project, known map[string]any) error {
	if len(known) == 0 {
		return nil
	}
	maps := []types.Extensions{project.Extensions}
	for _, s := range project.Services {
		maps = append(maps, s.Extensions)
	}
	for _, n := range project.Networks {
		maps = append(maps, n.Extensions)
	}
	for _, v := range project.Volumes {
		maps = append(maps, v.Extensions)
	}
	for _, c := range project.Configs {
		maps = append(maps, c.Extensions)
	}
	for _, s := range project.Secrets {
		maps = append(maps, s.Extensions)
	}
	for _, m := range maps {
		for name, typ := range known {
			raw, ok := m[name]
			if !ok {
				continue
			}
			target := reflect.New(reflect.TypeOf(typ)).Interface()
			buf, err := yaml.Marshal(raw)
			if err != nil {
				return err
			}
			if err := yaml.Unmarshal(buf, target); err != nil {
				return err
			}
			m[name] = reflect.ValueOf(target).Elem().Interface()
		}
	}
	return nil
}

func InvalidProjectNameErr(v string) error {
	return fmt.Errorf(
		"invalid project name %q: must consist only of lowercase alphanumeric characters, hyphens, and underscores as well as start with a letter or number",
		v,
	)
}

// projectName determines the canonical name to use for the project considering
// the loader Options as well as `name` fields in Compose YAML fields (which
// also support interpolation).
func projectName(details *types.ConfigDetails, opts *Options) error {
	defer func() {
		if details.Environment == nil {
			details.Environment = map[string]string{}
		}
		details.Environment[consts.ComposeProjectName] = opts.projectName
	}()

	if opts.projectNameImperativelySet {
		if NormalizeProjectName(opts.projectName) != opts.projectName {
			return InvalidProjectNameErr(opts.projectName)
		}
		return nil
	}

	type named struct {
		Name string `yaml:"name"`
	}

	// if user did NOT provide a name explicitly, then see if one is defined
	// in any of the config files
	var pjNameFromConfigFile string
	for _, configFile := range details.ConfigFiles {
		content := configFile.Content
		if content == nil {
			// This can be hit when Filename is set but Content is not. One
			// example is when using ToConfigFiles().
			d, err := os.ReadFile(configFile.Filename)
			if err != nil {
				return fmt.Errorf("failed to read file %q: %w", configFile.Filename, err)
			}
			content = d
			configFile.Content = d
		}
		var n named
		r := bytes.NewReader(content)
		decoder := yaml.NewDecoder(r)
		for {
			err := decoder.Decode(&n)
			if err != nil && errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				// HACK: the way that loading is currently structured, this is
				// a duplicative parse just for the `name`. if it fails, we
				// give up but don't return the error, knowing that it'll get
				// caught downstream for us
				break
			}
			if n.Name != "" {
				pjNameFromConfigFile = n.Name
			}
		}
	}
	if !opts.SkipInterpolation {
		interpolated, err := interp.Interpolate(
			map[string]interface{}{"name": pjNameFromConfigFile},
			*opts.Interpolate,
		)
		if err != nil {
			return err
		}
		pjNameFromConfigFile = interpolated["name"].(string)
	}

	if !opts.SkipNormalization {
		pjNameFromConfigFile = NormalizeProjectName(pjNameFromConfigFile)
	}
	if pjNameFromConfigFile != "" {
		opts.projectName = pjNameFromConfigFile
	}
	return nil
}

func NormalizeProjectName(s string) string {
	r := regexp.MustCompile("[a-z0-9_-]")
	s = strings.ToLower(s)
	s = strings.Join(r.FindAllString(s, -1), "")
	return strings.TrimLeft(s, "_-")
}

//nolint:unused
var userDefinedKeys = []tree.Path{
	"services",
	"services.*.depends_on",
	"volumes",
	"networks",
	"secrets",
	"configs",
}

//nolint:unused
func processExtensions(dict map[string]any, p tree.Path, extensions map[string]any) (map[string]interface{}, error) {
	extras := map[string]any{}
	var err error
	for key, value := range dict {
		skip := false
		for _, uk := range userDefinedKeys {
			if p.Matches(uk) {
				skip = true
				break
			}
		}
		if !skip && strings.HasPrefix(key, "x-") {
			extras[key] = value
			delete(dict, key)
			continue
		}
		switch v := value.(type) {
		case map[string]interface{}:
			dict[key], err = processExtensions(v, p.Next(key), extensions)
			if err != nil {
				return nil, err
			}
		case []interface{}:
			for i, e := range v {
				if m, ok := e.(map[string]interface{}); ok {
					v[i], err = processExtensions(m, p.Next(strconv.Itoa(i)), extensions)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}
	for name, val := range extras {
		if typ, ok := extensions[name]; ok {
			target := reflect.New(reflect.TypeOf(typ)).Elem().Interface()
			err = Transform(val, &target)
			if err != nil {
				return nil, err
			}
			extras[name] = target
		}
	}
	if len(extras) > 0 {
		dict[consts.Extensions] = extras
	}
	return dict, nil
}

// Transform projects a canonical compose dict (produced by the loader
// pipeline) into a typed compose-go struct. It marshals the source to
// yaml and decodes it back into target via yaml.v4 so each registered
// UnmarshalYAML method on the destination types (Services injects Name,
// SecretConfig / ConfigObjConfig lift x-content, the per-type short /
// long form decoders, ...) runs naturally without a parallel
// mapstructure decode-hook stack. processExtensions has already moved
// each x-* attribute into a nested "#extensions" sub-map; the inline
// yaml tag on Extensions fields expects them at parent level, so unwind
// that nesting just before the yaml round-trip.
func Transform(source interface{}, target interface{}) error {
	inlineExtensions(source)
	buf, err := yaml.Marshal(source)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(buf, target)
}

// inlineExtensions walks the source recursively and hoists every nested
// "#extensions" map up to its parent level, so a value previously
// rewritten by processExtensions as `{#extensions: {x-foo: bar}}` becomes
// `{x-foo: bar}` again. This is the shape the Extensions inline yaml tag
// captures, and it leaves typed extensions (KnownExtensions decoded into
// concrete structs) untouched because they round-trip through yaml.Marshal
// against the same struct tags as the source type.
func inlineExtensions(v any) {
	switch t := v.(type) {
	case map[string]any:
		if ext, ok := t[consts.Extensions].(map[string]any); ok {
			for k, val := range ext {
				t[k] = val
			}
			delete(t, consts.Extensions)
		}
		for _, child := range t {
			inlineExtensions(child)
		}
	case []any:
		for _, child := range t {
			inlineExtensions(child)
		}
	}
}

func convertToStringKeysRecursive(value interface{}, keyPrefix string) (interface{}, error) {
	if mapping, ok := value.(map[string]interface{}); ok {
		for key, entry := range mapping {
			var newKeyPrefix string
			if keyPrefix == "" {
				newKeyPrefix = key
			} else {
				newKeyPrefix = fmt.Sprintf("%s.%s", keyPrefix, key)
			}
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}
			mapping[key] = convertedEntry
		}
		return mapping, nil
	}
	if mapping, ok := value.(map[interface{}]interface{}); ok {
		dict := make(map[string]interface{})
		for key, entry := range mapping {
			str, ok := key.(string)
			if !ok {
				return nil, formatInvalidKeyError(keyPrefix, key)
			}
			var newKeyPrefix string
			if keyPrefix == "" {
				newKeyPrefix = str
			} else {
				newKeyPrefix = fmt.Sprintf("%s.%s", keyPrefix, str)
			}
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}
			dict[str] = convertedEntry
		}
		return dict, nil
	}
	if list, ok := value.([]interface{}); ok {
		var convertedList []interface{}
		for index, entry := range list {
			newKeyPrefix := fmt.Sprintf("%s[%d]", keyPrefix, index)
			convertedEntry, err := convertToStringKeysRecursive(entry, newKeyPrefix)
			if err != nil {
				return nil, err
			}
			convertedList = append(convertedList, convertedEntry)
		}
		return convertedList, nil
	}
	return value, nil
}

func formatInvalidKeyError(keyPrefix string, key interface{}) error {
	var location string
	if keyPrefix == "" {
		location = "at top level"
	} else {
		location = fmt.Sprintf("in %s", keyPrefix)
	}
	return fmt.Errorf("non-string key %s: %#v", location, key)
}

// Windows path, c:\\my\\path\\shiny, need to be changed to be compatible with
// the Engine. Volume path are expected to be linux style /c/my/path/shiny/
func convertVolumePath(volume types.ServiceVolumeConfig) types.ServiceVolumeConfig {
	volumeName := strings.ToLower(filepath.VolumeName(volume.Source))
	if len(volumeName) != 2 {
		return volume
	}

	convertedSource := fmt.Sprintf("/%c%s", volumeName[0], volume.Source[len(volumeName):])
	convertedSource = strings.ReplaceAll(convertedSource, "\\", "/")

	volume.Source = convertedSource
	return volume
}
