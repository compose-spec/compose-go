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
	"regexp"
	"slices"
	"strings"

	"github.com/compose-spec/compose-go/v2/consts"
	"github.com/compose-spec/compose-go/v2/errdefs"
	interp "github.com/compose-spec/compose-go/v2/interpolation"
	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/types"
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
	// ResourceLoaders manages support for remote resources
	ResourceLoaders []ResourceLoader
	// KnownExtensions manages x-* attribute we know and the corresponding go structs
	KnownExtensions map[string]any
	// Metada for telemetry
	Listeners []Listener
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

func (o *Options) SetProjectName(name string, imperativelySet bool) {
	o.projectName = name
	o.projectNameImperativelySet = imperativelySet
}

func (o Options) GetProjectName() (string, bool) {
	return o.projectName, o.projectNameImperativelySet
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
	model, err := LoadLazyModel(ctx, configDetails, options...)
	if err != nil {
		return nil, err
	}
	return model.Resolve(ctx)
}

// LoadModelWithContext reads a ConfigDetails and returns a fully loaded configuration as a yaml dictionary
func LoadModelWithContext(ctx context.Context, configDetails types.ConfigDetails, options ...func(*Options)) (map[string]any, error) {
	project, err := LoadWithContext(ctx, configDetails, options...)
	if err != nil {
		return nil, err
	}
	// Marshal the typed project back to a yaml dictionary for backward compatibility
	b, err := yaml.Marshal(project)
	if err != nil {
		return nil, err
	}
	var dict map[string]any
	if err := yaml.Unmarshal(b, &dict); err != nil {
		return nil, err
	}
	dict["name"] = project.Name
	return dict, nil
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
