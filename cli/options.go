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

package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/compose-spec/compose-go/consts"
	"github.com/compose-spec/compose-go/dotenv"
	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/compose-spec/compose-go/utils"
)

// ProjectOptions provides common configuration for loading a project.
type ProjectOptions struct {
	// Name is a valid Compose project name to be used or empty.
	//
	// If empty, the project loader will automatically infer a reasonable
	// project name if possible.
	Name string

	// WorkingDir is a file path to use as the project directory or empty.
	//
	// If empty, the project loader will automatically infer a reasonable
	// working directory if possible.
	WorkingDir string

	// ConfigPaths are file paths to one or more Compose files.
	//
	// These are applied in order by the loader following the merge logic
	// as described in the spec.
	//
	// The first entry is required and is the primary Compose file.
	// For convenience, WithConfigFileEnv and WithDefaultConfigPath
	// are provided to populate this in a predictable manner.
	ConfigPaths []string

	// Environment are additional environment variables to make available
	// for interpolation.
	//
	// NOTE: For security, the loader does not automatically expose any
	// process environment variables. For convenience, WithOsEnv can be
	// used if appropriate.
	Environment map[string]string

	// EnvFiles are file paths to ".env" files with additional environment
	// variable data.
	//
	// These are loaded in-order, so it is possible to override variables or
	// in subsequent files.
	//
	// This field is optional, but any file paths that are included here must
	// exist or an error will be returned during load.
	EnvFiles []string

	// HomeDir is the user's home directory to be used for resolving shell-style
	// `~/foo` home-relative paths, which are supported by path fields in Compose.
	HomeDir string

	loadOptions []func(*loader.Options)
}

type ProjectOptionsFn func(*ProjectOptions) error

// NewProjectOptions creates ProjectOptions
func NewProjectOptions(configs []string, opts ...ProjectOptionsFn) (*ProjectOptions, error) {
	options := &ProjectOptions{
		ConfigPaths: configs,
		Environment: map[string]string{},
	}
	for _, o := range opts {
		err := o(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

// WithName defines ProjectOptions' name
func WithName(name string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		// a project (once loaded) cannot have an empty name
		// however, on the options object, the name is optional: if unset,
		// a name will be inferred by the loader, so it's legal to set the
		// name to an empty string here
		if name != loader.NormalizeProjectName(name) {
			return loader.InvalidProjectNameErr(name)
		}
		o.Name = name
		return nil
	}
}

// WithWorkingDirectory defines ProjectOptions' working directory
func WithWorkingDirectory(wd string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		if wd == "" {
			return nil
		}
		abs, err := RealAbsPath(wd)
		if err != nil {
			return err
		}
		o.WorkingDir = abs
		return nil
	}
}

// WithOsUserHomeDirectory sets the home directory based on os.UserHomeDir().
func WithOsUserHomeDirectory(o *ProjectOptions) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return WithHomeDirectory(homeDir)(o)
}

// WithHomeDirectory sets the home directory to a custom value.
//
// An error is returned if the path does not exist.
func WithHomeDirectory(homeDir string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		if homeDir == "" {
			return nil
		}
		homeDir, err := RealAbsPath(homeDir)
		if err != nil {
			return err
		}
		o.HomeDir = homeDir
		return nil
	}
}

// WithConfigFileEnv allow to set compose config file paths by COMPOSE_FILE environment variable
func WithConfigFileEnv(o *ProjectOptions) error {
	if len(o.ConfigPaths) > 0 {
		return nil
	}
	sep := o.Environment[consts.ComposePathSeparator]
	if sep == "" {
		sep = string(os.PathListSeparator)
	}
	f, ok := o.Environment[consts.ComposeFilePath]
	if ok {
		paths, err := absoluteComposeFilePaths(strings.Split(f, sep))
		o.ConfigPaths = paths
		return err
	}
	return nil
}

// WithDefaultConfigPath searches for default config files from working directory
func WithDefaultConfigPath(o *ProjectOptions) error {
	if len(o.ConfigPaths) > 0 {
		return nil
	}
	pwd, err := o.GetWorkingDir()
	if err != nil {
		return err
	}
	for {
		candidates := findFiles(DefaultFileNames, pwd)
		if len(candidates) > 0 {
			winner := candidates[0]
			if len(candidates) > 1 {
				logrus.Warnf("Found multiple config files with supported names: %s", strings.Join(candidates, ", "))
				logrus.Warnf("Using %s", winner)
			}
			o.ConfigPaths = append(o.ConfigPaths, winner)

			overrides := findFiles(DefaultOverrideFileNames, pwd)
			if len(overrides) > 0 {
				if len(overrides) > 1 {
					logrus.Warnf("Found multiple override files with supported names: %s", strings.Join(overrides, ", "))
					logrus.Warnf("Using %s", overrides[0])
				}
				o.ConfigPaths = append(o.ConfigPaths, overrides[0])
			}
			return nil
		}
		parent := filepath.Dir(pwd)
		if parent == pwd {
			// no config file found, but that's not a blocker if caller only needs project name
			return nil
		}
		pwd = parent
	}
}

// WithEnv defines a key=value set of variables used for compose file interpolation
func WithEnv(env []string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		for k, v := range utils.GetAsEqualsMap(env) {
			o.Environment[k] = v
		}
		return nil
	}
}

// WithDiscardEnvFiles sets discards the `env_file` section after resolving to
// the `environment` section
func WithDiscardEnvFile(o *ProjectOptions) error {
	o.loadOptions = append(o.loadOptions, loader.WithDiscardEnvFiles)
	return nil
}

// WithLoadOptions provides a hook to control how compose files are loaded
func WithLoadOptions(loadOptions ...func(*loader.Options)) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, loadOptions...)
		return nil
	}
}

// WithProfiles sets profiles to be activated
func WithProfiles(profiles []string) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, loader.WithProfiles(profiles))
		return nil
	}
}

// WithOsEnv imports environment variables from OS
func WithOsEnv(o *ProjectOptions) error {
	for k, v := range utils.GetAsEqualsMap(os.Environ()) {
		if _, set := o.Environment[k]; set {
			continue
		}
		o.Environment[k] = v
	}
	return nil
}

// WithEnvFile set an alternate env file
// deprecated - use WithEnvFiles
func WithEnvFile(file string) ProjectOptionsFn {
	var files []string
	if file != "" {
		files = []string{file}
	}
	return WithEnvFiles(files...)
}

// WithEnvFiles set alternate env files
func WithEnvFiles(file ...string) ProjectOptionsFn {
	return func(options *ProjectOptions) error {
		options.EnvFiles = file
		return nil
	}
}

// WithDotEnv imports environment variables from .env file
func WithDotEnv(o *ProjectOptions) error {
	wd, err := o.GetWorkingDir()
	if err != nil {
		return err
	}
	envMap, err := dotenv.GetEnvFromFile(o.Environment, wd, o.EnvFiles)
	if err != nil {
		return err
	}
	for k, v := range envMap {
		if _, set := o.Environment[k]; !set {
			o.Environment[k] = v
		}
	}
	return nil
}

// WithInterpolation set ProjectOptions to enable/skip interpolation
func WithInterpolation(interpolation bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *loader.Options) {
			options.SkipInterpolation = !interpolation
		})
		return nil
	}
}

// WithNormalization set ProjectOptions to enable/skip normalization
func WithNormalization(normalization bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *loader.Options) {
			options.SkipNormalization = !normalization
		})
		return nil
	}
}

// WithConsistency set ProjectOptions to enable/skip consistency
func WithConsistency(consistency bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *loader.Options) {
			options.SkipConsistencyCheck = !consistency
		})
		return nil
	}
}

// WithResolvedPaths set ProjectOptions to enable paths resolution
func WithResolvedPaths(resolve bool) ProjectOptionsFn {
	return func(o *ProjectOptions) error {
		o.loadOptions = append(o.loadOptions, func(options *loader.Options) {
			options.ResolvePaths = resolve
		})
		return nil
	}
}

// DefaultFileNames defines the Compose file names for auto-discovery (in order of preference)
var DefaultFileNames = []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"}

// DefaultOverrideFileNames defines the Compose override file names for auto-discovery (in order of preference)
var DefaultOverrideFileNames = []string{"compose.override.yml", "compose.override.yaml", "docker-compose.override.yml", "docker-compose.override.yaml"}

// GetWorkingDir returns the absolute path to the project working directory.
func (o ProjectOptions) GetWorkingDir() (string, error) {
	// explicitly defined takes precedence
	workDir := o.WorkingDir
	// otherwise, pick based on the first non-stdin config file
	if workDir == "" {
		for _, path := range o.ConfigPaths {
			if path != "-" {
				workDir = filepath.Dir(path)
				break
			}
		}
	}
	// fallback to OS working directory
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	return RealAbsPath(workDir)
}

// ProjectFromOptions load a compose project based on command line options
func ProjectFromOptions(options *ProjectOptions) (*types.Project, error) {
	configPaths, err := getConfigPathsFromOptions(options)
	if err != nil {
		return nil, err
	}

	var configs []types.ConfigFile
	for _, f := range configPaths {
		var b []byte
		if f == "-" {
			b, err = io.ReadAll(os.Stdin)
			if err != nil {
				return nil, err
			}
		} else {
			b, err = os.ReadFile(f)
			if err != nil {
				return nil, err
			}
		}
		configs = append(configs, types.ConfigFile{
			Filename: f,
			Content:  b,
		})
	}

	workingDir, err := options.GetWorkingDir()
	if err != nil {
		return nil, err
	}

	options.loadOptions = append(options.loadOptions,
		withNamePrecedenceLoad(workingDir, options),
		withConvertWindowsPaths(options))

	project, err := loader.Load(types.ConfigDetails{
		ConfigFiles: configs,
		WorkingDir:  workingDir,
		Environment: options.Environment,
		HomeDir:     options.HomeDir,
	}, options.loadOptions...)
	if err != nil {
		return nil, err
	}

	project.ComposeFiles = configPaths
	return project, nil
}

func withNamePrecedenceLoad(absWorkingDir string, options *ProjectOptions) func(*loader.Options) {
	return func(opts *loader.Options) {
		if options.Name != "" {
			opts.SetProjectName(options.Name, true)
		} else if nameFromEnv, ok := options.Environment[consts.ComposeProjectName]; ok && nameFromEnv != "" {
			opts.SetProjectName(nameFromEnv, true)
		} else {
			opts.SetProjectName(
				loader.NormalizeProjectName(filepath.Base(absWorkingDir)),
				false,
			)
		}
	}
}

func withConvertWindowsPaths(options *ProjectOptions) func(*loader.Options) {
	return func(o *loader.Options) {
		if o.ResolvePaths {
			o.ConvertWindowsPaths = utils.StringToBool(options.Environment["COMPOSE_CONVERT_WINDOWS_PATHS"])
		}
	}
}

// getConfigPathsFromOptions retrieves the config files for project based on project options
func getConfigPathsFromOptions(options *ProjectOptions) ([]string, error) {
	if len(options.ConfigPaths) != 0 {
		return absoluteComposeFilePaths(options.ConfigPaths)
	}
	return nil, errors.Wrap(errdefs.ErrNotFound, "no configuration file provided")
}

func findFiles(names []string, pwd string) []string {
	candidates := []string{}
	for _, n := range names {
		f := filepath.Join(pwd, n)
		if _, err := os.Stat(f); err == nil {
			candidates = append(candidates, f)
		}
	}
	return candidates
}

// absoluteComposeFilePaths returns a slice of absolute paths for the input paths or an error
// if any path cannot be resolved or does not exist.
//
// The special case of `-` is passed through as-is, which signifies stdin.
func absoluteComposeFilePaths(p []string) ([]string, error) {
	var paths []string
	for _, f := range p {
		if f == "-" {
			paths = append(paths, f)
			continue
		}
		f, err := RealAbsPath(f)
		if err != nil {
			return nil, err
		}
		paths = append(paths, f)
	}
	return paths, nil
}
