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

package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/tree"
)

type resolver func(any) (any, error)

// ResolveRelativePaths make relative paths absolute
func ResolveRelativePaths(project map[string]any, base string) error {
	r := relativePathsResolver{workingDir: base}
	r.resolvers = map[tree.Path]resolver{
		"services.*.build.context":               r.absPath, // TODO(ndeloof) need to detect remote
		"services.*.build.additional_contexts.*": r.absPath, // TODO(ndeloof) need to detect remote
		"services.*.env_file":                    r.absPath,
		"services.*.extends.file":                r.absPath,
		"services.*.develop.watch.*.path":        r.absPath,
		"services.*.volume.source":               r.absPath, // TODO(ndeloof) bind only, maybe unix path
		"config.file":                            r.absPath,
		"secret.file":                            r.absPath,
		"include.path":                           r.absPath,
		"include.project_directory":              r.absPath,
		"include.env_file":                       r.absPath,
	}
	_, err := r.resolveRelativePaths(project, tree.NewPath())
	return err
}

type relativePathsResolver struct {
	workingDir string
	resolvers  map[tree.Path]resolver
}

func (r *relativePathsResolver) resolveRelativePaths(value any, p tree.Path) (any, error) {
	for pattern, resolver := range r.resolvers {
		if p.Matches(pattern) {
			return resolver(value)
		}
	}
	switch v := value.(type) {
	case map[string]any:
		for k, e := range v {
			resolved, err := r.resolveRelativePaths(e, p.Next(k))
			if err != nil {
				return nil, err
			}
			v[k] = resolved
		}
	case []any:
		for i, e := range v {
			resolved, err := r.resolveRelativePaths(e, p.Next("[]"))
			if err != nil {
				return nil, err
			}
			v[i] = resolved
		}
	}
	return value, nil
}

func (r *relativePathsResolver) absPath(value any) (any, error) {
	switch v := value.(type) {
	case []any:
		for i, s := range v {
			abs, err := r.absPath(s)
			if err != nil {
				return nil, err
			}
			v[i] = abs
		}
	case string:
		if strings.HasPrefix(v, "~") {
			home, _ := os.UserHomeDir()
			return filepath.Join(home, v[1:]), nil
		}
		if filepath.IsAbs(v) {
			return v, nil
		}
		return filepath.Join(r.workingDir, v), nil
	}
	return nil, fmt.Errorf("unexpected type %T", value)
}
