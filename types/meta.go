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

package types

import "golang.org/x/exp/maps"

type ComposeFileType int

const (
	unknownComposeFileType ComposeFileType = iota << 1
	MainComposeFileType
	OverrideComposeFileType
	IncludeComposeFileType
	EnvComposeFileType
)

// FileMeta from loading a Compose file and might represent the whole project or a part
// of it (in the case of `include`).
//
// This is not part of the Compose Spec; it is populated by compose-go itself during the
// load.
type FileMeta struct {
	// Path to the Compose file that was loaded.
	Path string

	// OverrideFilePaths to any partial/override files used in the load.
	//
	// Because override files are not logical Compose (sub)projects on their own, they are only represented
	// via this field and not FileMeta objects of their own.
	OverrideFilePaths []string

	EnvFilePaths []string

	// ProjectDirectory is the working directory for the load of the Compose file at Path.
	//
	// If this is a sub-project, this might be different than the parent directory project.
	// This can be used to correctly resolve relative paths for sub-projects.
	ProjectDirectory string

	// Services that were directly loaded by this Compose file or its overrides.
	//
	// Any services loaded via `include` in this Compose file will be included on the corresponding
	// FileMeta object in Includes. This ensures each service only exists in a single FileMeta within
	// the tree.
	Services []string

	// Includes are subprojects loaded via `include` by this Compose file or its overrides.
	Includes []FileMeta
}

func (f FileMeta) AllFilePaths() []string {
	paths := f.FilePaths(0)
	return maps.Keys(paths)
}

// FilePaths returns a filtered result
func (f FileMeta) FilePaths(filter ComposeFileType) map[string]ComposeFileType {
	ret := make(map[string]ComposeFileType)
	all := filter == 0
	includeMain := all || filter&MainComposeFileType != 0
	includeInclude := all || filter&IncludeComposeFileType != 0
	includeOverride := all || filter&OverrideComposeFileType != 0
	includeEnv := all || filter&EnvComposeFileType != 0
	iterateFileMeta(f, func(meta FileMeta) {
		if f.Path == meta.Path {
			if includeMain {
				ret[meta.Path] = MainComposeFileType
			}
		} else if includeInclude {
			ret[meta.Path] = IncludeComposeFileType
		}
		if includeOverride {
			for i := range meta.OverrideFilePaths {
				ret[meta.OverrideFilePaths[i]] = OverrideComposeFileType
			}
		}
		if includeEnv {
			for i := range meta.EnvFilePaths {
				ret[meta.EnvFilePaths[i]] = EnvComposeFileType
			}
		}
	})
	return ret
}

func iterateFileMeta(root FileMeta, fn func(meta FileMeta)) {
	fn(root)
	for i := range root.Includes {
		iterateFileMeta(root.Includes[i], fn)
	}
}
