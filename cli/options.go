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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// ProjectOptions groups the command line options recommended for a Compose implementation
type ProjectOptions struct {
	Name        string
	WorkingDir  string
	ConfigPaths []string
	Environment []string
}

// WithOsEnv imports environment variables from OS until those have been overridden by ProjectOptions.Environment
func (o ProjectOptions) WithOsEnv() ProjectOptions {
	env := getAsEqualsMap(o.Environment)
	for k, v := range getAsEqualsMap(os.Environ()) {
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}

	return ProjectOptions{
		Name:        o.Name,
		WorkingDir:  o.WorkingDir,
		ConfigPaths: o.ConfigPaths,
		Environment: getAsStringList(env),
	}
}

// WithDotEnv imports environment variables from .env file until those have been overridden by ProjectOptions.Environment
func (o ProjectOptions) WithDotEnv() (ProjectOptions, error) {
	dir, err := o.GetWorkingDir()
	if err != nil {
		return o, err
	}
	dotEnvFile := filepath.Join(dir, ".env")
	if _, err := os.Stat(dotEnvFile); os.IsNotExist(err) {
		return o, nil
	}
	file, err := os.Open(dotEnvFile)
	if err != nil {
		return o, err
	}
	defer file.Close()

	env, err := godotenv.Parse(file)
	if err != nil {
		return o, err
	}

	return ProjectOptions{
		Name:        o.Name,
		WorkingDir:  o.WorkingDir,
		ConfigPaths: o.ConfigPaths,
		Environment: getAsStringList(env),
	}, nil
}

// DefaultFileNames defines the Compose file names for auto-discovery (in order of preference)
var DefaultFileNames = []string{"compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml"}

const (
	ComposeProjectName   = "COMPOSE_PROJECT_NAME"
	ComposeFileSeparator = "COMPOSE_FILE_SEPARATOR"
	ComposeFilePath      = "COMPOSE_FILE"
)

func (o ProjectOptions) GetWorkingDir() (string, error) {
	if o.WorkingDir != "" {
		return o.WorkingDir, nil
	}
	for _, path := range o.ConfigPaths {
		if path != "-" {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return "", err
			}
			return filepath.Dir(absPath), nil
		}
	}
	return os.Getwd()
}

// ProjectFromOptions load a compose project based on command line options
func ProjectFromOptions(options *ProjectOptions) (*types.Project, error) {
	configPaths, err := getConfigPathsFromOptions(options)
	if err != nil {
		return nil, err
	}

	configs, err := parseConfigs(configPaths)
	if err != nil {
		return nil, err
	}

	workingDir, err := options.GetWorkingDir()
	if err != nil {
		return nil, err
	}

	return loader.Load(types.ConfigDetails{
		ConfigFiles: configs,
		WorkingDir:  workingDir,
		Environment: getAsEqualsMap(options.Environment),
	}, func(opts *loader.Options) {
		if options.Name != "" {
			opts.Name = options.Name
		} else if nameFromEnv, ok := os.LookupEnv(ComposeProjectName); ok {
			opts.Name = nameFromEnv
		} else {
			opts.Name = regexp.MustCompile(`[^a-z0-9\\-_]+`).
				ReplaceAllString(strings.ToLower(filepath.Base(workingDir)), "")
		}
	})
}

// getConfigPathsFromOptions retrieves the config files for project based on project options
func getConfigPathsFromOptions(options *ProjectOptions) ([]string, error) {
	paths := []string{}
	pwd := options.WorkingDir
	if pwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		pwd = wd
	}

	if len(options.ConfigPaths) != 0 {
		for _, f := range options.ConfigPaths {
			if f == "-" {
				paths = append(paths, f)
				continue
			}
			if !filepath.IsAbs(f) {
				f = filepath.Join(pwd, f)
			}
			if _, err := os.Stat(f); err != nil {
				return nil, err
			}
			paths = append(paths, f)
		}
		return paths, nil
	}

	sep := os.Getenv(ComposeFileSeparator)
	if sep == "" {
		sep = string(os.PathListSeparator)
	}
	f := os.Getenv(ComposeFilePath)
	if f != "" {
		return strings.Split(f, sep), nil
	}

	for {
		candidates := []string{}
		for _, n := range DefaultFileNames {
			f := filepath.Join(pwd, n)
			if _, err := os.Stat(f); err == nil {
				candidates = append(candidates, f)
			}
		}
		if len(candidates) > 0 {
			winner := candidates[0]
			if len(candidates) > 1 {
				logrus.Warnf("Found multiple config files with supported names: %s", strings.Join(candidates, ", "))
				logrus.Warnf("Using %s", winner)
			}
			return []string{winner}, nil
		}
		parent := filepath.Dir(pwd)
		if parent == pwd {
			return nil, errors.Wrap(errdefs.ErrNotFound, "can't find a suitable configuration file in this directory or any parent")
		}
		pwd = parent
	}
}

func parseConfigs(configPaths []string) ([]types.ConfigFile, error) {
	files := []types.ConfigFile{}
	for _, f := range configPaths {
		var (
			b   []byte
			err error
		)
		if f == "-" {
			b, err = ioutil.ReadAll(os.Stdin)
		} else {
			b, err = ioutil.ReadFile(f)
		}
		if err != nil {
			return nil, err
		}
		config, err := loader.ParseYAML(b)
		if err != nil {
			return nil, err
		}
		files = append(files, types.ConfigFile{Filename: f, Config: config})
	}
	return files, nil
}

// getAsEqualsMap split key=value formatted strings into a key : value map
func getAsEqualsMap(em []string) map[string]string {
	m := make(map[string]string)
	for _, v := range em {
		kv := strings.SplitN(v, "=", 2)
		m[kv[0]] = kv[1]
	}
	return m
}

// getAsEqualsMap format a key : value map into key=value strings
func getAsStringList(em map[string]string) []string {
	m := make([]string, 0, len(em))
	for k, v := range em {
		m = append(m, fmt.Sprintf("%s=%s", k, v))
	}
	return m
}
