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

	"github.com/compose-spec/compose-go/v3/override"
	"github.com/compose-spec/compose-go/v3/paths"
	"github.com/compose-spec/compose-go/v3/transform"
	"github.com/compose-spec/compose-go/v3/types"
	"github.com/compose-spec/compose-go/v3/validation"
)

// postMergeLegacy applies the post-merge map-based pipeline on top of the
// dict produced by the v3 yaml.Node pipeline. It mirrors the legacy
// loadYamlModel + load tail: default values, canonical short-syntax
// expansion, OmitEmpty cleanup, sequence unicity, semantic validation,
// bare-variable environment resolution, version key handling and Normalize.
func postMergeLegacy(_ context.Context, dict map[string]any, configDetails types.ConfigDetails, opts *Options) (map[string]any, error) {
	var err error

	// Canonical first: short syntax sequences (secrets, configs, volumes,
	// ports, ...) are expanded into their canonical mapping form so the
	// later default-value pass operates on the expected shape.
	dict, err = transform.Canonical(dict, opts.SkipInterpolation)
	if err != nil {
		return nil, err
	}

	dict = OmitEmpty(dict)

	dict, err = override.EnforceUnicity(dict)
	if err != nil {
		return nil, err
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

	if _, ok := dict["version"]; ok {
		opts.warnObsoleteVersion(firstFilename(configDetails))
		delete(dict, "version")
	}

	// Resolve relative paths declared at the main project level. Paths that
	// came from an included or extends file have already been rewritten by
	// the v3 resolvePathsPass against their own NodeContext.WorkingDir; the
	// global walker below is a no-op on those (they are absolute or already
	// expressed relative to the main project directory) and only touches
	// values left as relative paths by the legacy SetDefaultValues step
	// (typically `build.context: .`).
	if opts.ResolvePaths {
		var remotes []paths.RemoteResource
		for _, loader := range opts.RemoteResourceLoaders() {
			remotes = append(remotes, loader.Accept)
		}
		if err := paths.ResolveRelativePaths(dict, configDetails.WorkingDir, remotes); err != nil {
			return nil, err
		}
	}

	ResolveEnvironment(dict, configDetails.Environment)

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

func firstFilename(c types.ConfigDetails) string {
	if len(c.ConfigFiles) == 0 {
		return ""
	}
	return c.ConfigFiles[0].Filename
}
