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
	"path/filepath"

	"github.com/compose-spec/compose-go/dotenv"
	interp "github.com/compose-spec/compose-go/interpolation"
	"github.com/compose-spec/compose-go/types"
)

// loadIncludeConfig parse the require config from raw yaml
func loadIncludeConfig(source any) ([]types.IncludeConfig, error) {
	if source == nil {
		return nil, nil
	}
	var requires []types.IncludeConfig
	err := Transform(source, &requires)
	return requires, err
}

func ApplyInclude(ctx context.Context, filename string, configDetails types.ConfigDetails, model map[string]any, options *Options) error {
	included := make(map[string][]types.IncludeConfig)
	includeConfig, err := loadIncludeConfig(model["include"])
	if err != nil {
		return err
	}
	for _, r := range includeConfig {
		included[filename] = append(included[filename], r)

		for i, p := range r.Path {
			for _, loader := range options.ResourceLoaders {
				if loader.Accept(p) {
					path, err := loader.Load(ctx, p)
					if err != nil {
						return err
					}
					p = path
					break
				}
			}
			r.Path[i] = absPath(configDetails.WorkingDir, p)
		}
		if r.ProjectDirectory == "" {
			r.ProjectDirectory = filepath.Dir(r.Path[0])
		}

		loadOptions := options.clone()
		loadOptions.ResolvePaths = true
		loadOptions.SkipNormalization = true
		loadOptions.SkipConsistencyCheck = true

		envFromFile, err := dotenv.GetEnvFromFile(configDetails.Environment, r.ProjectDirectory, r.EnvFile)
		if err != nil {
			return err
		}

		config := types.ConfigDetails{
			WorkingDir:  r.ProjectDirectory,
			ConfigFiles: types.ToConfigFiles(r.Path),
			Environment: configDetails.Environment.Clone().Merge(envFromFile),
		}
		loadOptions.Interpolate = &interp.Options{
			Substitute:      options.Interpolate.Substitute,
			LookupValue:     config.LookupEnv,
			TypeCastMapping: options.Interpolate.TypeCastMapping,
		}
		imported, err := loadYamlModel(ctx, config, loadOptions, &cycleTracker{})
		if err != nil {
			return err
		}
		err = importResources(imported, model)
		if err != nil {
			return err
		}
	}
	return nil
}

// importResources import into model all resources defined by imported, and report error on conflict
func importResources(source map[string]any, target map[string]any) error {
	importResource(source, target, "services")
	importResource(source, target, "volumes")
	importResource(source, target, "networks")
	importResource(source, target, "secrets")
	importResource(source, target, "configs")
	return nil
}

func importResource(source map[string]any, target map[string]any, key string) {
	from := source[key]
	if from != nil {
		var target map[string]any
		if v, ok := target[key]; ok {
			target = v.(map[string]any)
		} else {
			target = map[string]any{}
		}
		for name, a := range from.(map[string]any) {
			target[name] = a
		}
		target[key] = target
	}
}
