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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// The relative paths declared by an `include` entry (`env_file`,
// `project_directory`, and the default `.env` of the project directory) are
// resolved against the project directory of the file declaring the entry, at
// every depth of the include tree, and never against the working directory of
// the process.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md
// https://github.com/docker/compose/issues/14488
//
// Each scenario below describes a tree of files, and is loaded once from each
// of the working directories of `workingDirectories`: the project directory,
// a directory inside the tree, and a directory outside of it holding decoy
// files at the very places a path resolved against the working directory
// would reach. The model loaded must be the same from all of them.
//
// The tests move the process with t.Chdir, so none of them can run in parallel.

// fileTree is a set of files, by path relative to the root of the tree, with
// their content.
type fileTree map[string]string

// workingDirectory is a directory the process is placed in to load a tree.
type workingDirectory struct {
	name string
	// dir is relative to the root of the tree, and created if missing
	dir string
}

// workingDirectories are the places the process may run from. "elsewhere" is
// outside of the project: it is given decoys by decoys().
var workingDirectories = []workingDirectory{
	{name: "project", dir: "proj"},
	{name: "inside", dir: "proj/p"},
	{name: "deep", dir: "proj/p/q"},
	{name: "elsewhere", dir: "elsewhere"},
	{name: "root", dir: "."},
}

// decoyNames are the files each scenario gets in the working directory
// "elsewhere", so that a path resolved against it is found, with a content
// which is obviously wrong, instead of failing with a missing file.
var decoyNames = []string{
	"marker.env", "p/marker.env", "q/marker.env", "p/q/marker.env",
	"r/marker.env", "pd/marker.env", ".env", "p/.env", "q/.env", "p/q/.env",
	"pd/.env", "sub/.env",
}

// writeTree writes the files of t under root, together with the decoys.
func writeTree(t *testing.T, root string, files fileTree) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	for _, name := range decoyNames {
		path := filepath.Join(root, "elsewhere", filepath.FromSlash(name))
		assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		assert.NilError(t, os.WriteFile(path, []byte("WHICH=decoy-"+name+"\n"), 0o600))
	}
}

// includeCase is a tree loaded from each working directory.
type includeCase struct {
	files fileTree
	// environment is the one of the including project
	environment types.Mapping
	// options are given the root of the tree
	options func(root string) []func(*Options)
}

// run loads `proj/compose.yaml` of the tree, with the absolute project
// directory `proj`, once from each working directory, and calls check with the
// outcome. The tree is written once, the process is moved around.
func (c includeCase) run(t *testing.T, check func(t *testing.T, root string, p *types.Project, err error)) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	writeTree(t, root, c.files)
	for _, wd := range workingDirectories {
		t.Run("from "+wd.name, func(t *testing.T) {
			dir := filepath.Join(root, filepath.FromSlash(wd.dir))
			assert.NilError(t, os.MkdirAll(dir, 0o700))
			t.Chdir(dir)
			proj := filepath.Join(root, "proj")
			opts := []func(*Options){func(o *Options) {
				o.SkipNormalization = true
				o.SetProjectName("project", true)
			}}
			if c.options != nil {
				opts = append(opts, c.options(root)...)
			}
			p, err := LoadWithContext(t.Context(), types.ConfigDetails{
				WorkingDir:  proj,
				ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(proj, "compose.yaml")}},
				Environment: c.environment,
			}, opts...)
			check(t, root, p, err)
		})
	}
}

// loadFromEachWorkingDirectory is includeCase.run for the common case of a
// tree loaded with the default options.
func loadFromEachWorkingDirectory(t *testing.T, files fileTree, check func(t *testing.T, root string, p *types.Project, err error)) {
	t.Helper()
	includeCase{files: files}.run(t, check)
}

// whichOf is the value of WHICH in the environment of the service "s" of p.
func whichOf(t *testing.T, p *types.Project) string {
	t.Helper()
	s, ok := p.Services["s"]
	assert.Assert(t, ok, "service s is missing from the model")
	v, ok := s.Environment["WHICH"]
	assert.Assert(t, ok && v != nil, "WHICH is not set for service s")
	return *v
}

// reader is a service reading the variable WHICH, defaulted when no env file
// defines it.
const reader = `
services:
  s:
    image: alpine
    environment:
      WHICH: "${WHICH:-unset}"
`

// expectWhich checks the value of WHICH read by the service "s", whatever the
// working directory.
func expectWhich(want string) func(*testing.T, string, *types.Project, error) {
	return func(t *testing.T, _ string, p *types.Project, err error) {
		t.Helper()
		assert.NilError(t, err)
		assert.Equal(t, whichOf(t, p), want)
	}
}

// The `env_file` of an include entry is relative to the project directory of
// the file declaring the entry, whatever the depth of that file in the include
// tree. This is the reproducer of docker/compose#14488, where the third level
// read the env file of the working directory.
func TestIncludeEnvFileIsRelativeToTheDeclaringFileAtEveryDepth(t *testing.T) {
	// every level has a marker.env of its own: the one read tells which
	// directory the path was resolved against.
	markers := fileTree{
		"proj/marker.env":       "WHICH=proj\n",
		"proj/p/marker.env":     "WHICH=p\n",
		"proj/p/q/marker.env":   "WHICH=p/q\n",
		"proj/p/q/r/marker.env": "WHICH=p/q/r\n",
	}
	merge := func(files ...fileTree) fileTree {
		all := fileTree{}
		for _, f := range files {
			for k, v := range f {
				all[k] = v
			}
		}
		return all
	}

	tests := []struct {
		name  string
		files fileTree
		want  string
	}{
		{
			name: "declared by the main file",
			files: fileTree{
				"proj/compose.yaml": "include:\n  - path: p/inc.yaml\n    env_file: marker.env\n",
				"proj/p/inc.yaml":   reader,
			},
			want: "proj",
		},
		{
			name: "declared one level deep",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include:\n  - path: q/inc2.yaml\n    env_file: marker.env\n",
				"proj/p/q/inc2.yaml": reader,
			},
			want: "p",
		},
		{
			name: "declared two levels deep",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    env_file: marker.env\n",
				"proj/p/q/i3.yaml":   reader,
			},
			want: "p/q",
		},
		{
			name: "declared three levels deep",
			files: fileTree{
				"proj/compose.yaml":    "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":      "include: [q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml":   "include: [r/inc3.yaml]\n",
				"proj/p/q/r/inc3.yaml": "include:\n  - path: i4.yaml\n    env_file: marker.env\n",
				"proj/p/q/r/i4.yaml":   reader,
			},
			want: "p/q/r",
		},
		{
			name: "declared in a file included from a sibling directory",
			files: fileTree{
				"proj/compose.yaml":     "include: [p/inc.yaml, other/inc.yaml]\n",
				"proj/p/inc.yaml":       "services:\n  unrelated:\n    image: alpine\n",
				"proj/other/inc.yaml":   "include:\n  - path: ../p/q/i3.yaml\n    env_file: marker.env\n",
				"proj/p/q/i3.yaml":      reader,
				"proj/other/marker.env": "WHICH=other\n",
			},
			want: "other",
		},
		{
			name: "a path going up the tree",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml": "include:\n  - path: ../../p/q/i3.yaml\n    env_file: ../marker.env\n",
				"proj/p/q/i3.yaml":   reader,
			},
			want: "p",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadFromEachWorkingDirectory(t, merge(markers, tt.files), expectWhich(tt.want))
		})
	}
}

// When the declaring file of an include entry has its own `project_directory`,
// the relative `env_file` of the entry is relative to that directory, which is
// the base path of every relative path set in the file.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#project_directory
func TestIncludeEnvFileIsRelativeToTheProjectDirectoryOfTheDeclaringFile(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml": "include:\n  - path: p/inc.yaml\n    project_directory: pd\n",
		// inc.yaml declares an include of its own: its relative paths
		// are those of the project directory pd, not of p
		"proj/p/inc.yaml":    "include:\n  - path: ../p/q/i3.yaml\n    env_file: marker.env\n",
		"proj/pd/marker.env": "WHICH=pd\n",
		"proj/p/marker.env":  "WHICH=p\n",
		"proj/p/q/i3.yaml":   reader,
	}, expectWhich("pd"))
}

// Several env files of an entry are merged in order, each of them relative to
// the declaring file, at every depth: the last one wins.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvFilesAreMergedAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":  "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    env_file:\n      - base.env\n      - ./over/override.env\n",
		"proj/p/q/i3.yaml": `
services:
  s:
    image: alpine
    environment:
      FIRST: "${FIRST:-unset}"
      SECOND: "${SECOND:-unset}"
`,
		"proj/p/q/base.env":          "FIRST=base\nSECOND=base\n",
		"proj/p/q/over/override.env": "SECOND=override\n",
	}, func(t *testing.T, _ string, p *types.Project, err error) {
		assert.NilError(t, err)
		assert.Equal(t, *p.Services["s"].Environment["FIRST"], "base")
		assert.Equal(t, *p.Services["s"].Environment["SECOND"], "override")
	})
}

// An absolute `env_file` is used as it is, at every depth.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeAbsoluteEnvFileAtEveryDepth(t *testing.T) {
	shared, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	abs := filepath.Join(shared, "abs.env")
	assert.NilError(t, os.WriteFile(abs, []byte("WHICH=absolute\n"), 0o600))
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":   "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":     "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":  "include:\n  - path: i3.yaml\n    env_file: " + filepath.ToSlash(abs) + "\n",
		"proj/p/q/i3.yaml":    reader,
		"proj/p/q/marker.env": "WHICH=wrong\n",
	}, expectWhich("absolute"))
}

// With no `env_file`, the `.env` file of the project directory of the included
// file is the default, at every depth, and the working directory has none. The
// variables an including file got from its own `.env` are visible to the files
// it includes, which complete them with their own.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeDefaultEnvFileAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":    "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":      "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":   "include: [r/inc3.yaml]\n",
		"proj/p/q/r/inc3.yaml": "include: [s/i4.yaml]\n",
		"proj/p/q/r/s/i4.yaml": `
services:
  s:
    image: alpine
    environment:
      LEVEL_P: "${LEVEL_P:-unset}"
      LEVEL_Q: "${LEVEL_Q:-unset}"
      LEVEL_R: "${LEVEL_R:-unset}"
      LEVEL_S: "${LEVEL_S:-unset}"
`,
		"proj/p/.env":       "LEVEL_P=p\n",
		"proj/p/q/.env":     "LEVEL_Q=p/q\n",
		"proj/p/q/r/.env":   "LEVEL_R=p/q/r\n",
		"proj/p/q/r/s/.env": "LEVEL_S=p/q/r/s\n",
	}, func(t *testing.T, _ string, p *types.Project, err error) {
		assert.NilError(t, err)
		env := p.Services["s"].Environment
		assert.Equal(t, *env["LEVEL_P"], "p")
		assert.Equal(t, *env["LEVEL_Q"], "p/q")
		assert.Equal(t, *env["LEVEL_R"], "p/q/r")
		assert.Equal(t, *env["LEVEL_S"], "p/q/r/s")
	})
}

// A declared `env_file` replaces the default `.env` of the project directory
// of the included file at every depth, it does not add to it.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvFileReplacesTheDefaultAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":  "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml": "include:\n  - path: r/i3.yaml\n    env_file: declared.env\n",
		"proj/p/q/r/i3.yaml": `
services:
  s:
    image: alpine
    environment:
      FROM_DOTENV: "${FROM_DOTENV:-unset}"
      FROM_DECLARED: "${FROM_DECLARED:-unset}"
`,
		"proj/p/q/r/.env":       "FROM_DOTENV=yes\n",
		"proj/p/q/declared.env": "FROM_DECLARED=yes\n",
	}, func(t *testing.T, _ string, p *types.Project, err error) {
		assert.NilError(t, err)
		assert.Equal(t, *p.Services["s"].Environment["FROM_DOTENV"], "unset")
		assert.Equal(t, *p.Services["s"].Environment["FROM_DECLARED"], "yes")
	})
}

// `/dev/null` opts out of any env file, the default `.env` of the project
// directory included, at every depth.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvFileDevNullAtEveryDepth(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null is not an absolute path on windows")
	}
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":  "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml": "include:\n  - path: r/i3.yaml\n    env_file: /dev/null\n",
		"proj/p/q/r/i3.yaml": reader,
		"proj/p/q/r/.env":    "WHICH=p/q/r\n",
	}, expectWhich("unset"))
}

// The environment of the including project takes precedence over the env
// files of an include entry, at every depth.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvironmentOverridesTheEnvFileAtEveryDepth(t *testing.T) {
	includeCase{
		files: fileTree{
			"proj/compose.yaml":   "include: [p/inc.yaml]\n",
			"proj/p/inc.yaml":     "include: [q/inc2.yaml]\n",
			"proj/p/q/inc2.yaml":  "include:\n  - path: i3.yaml\n    env_file: marker.env\n",
			"proj/p/q/i3.yaml":    reader,
			"proj/p/q/marker.env": "WHICH=p/q\n",
		},
		environment: types.Mapping{"WHICH": "from-environment"},
	}.run(t, expectWhich("from-environment"))
}

// An `env_file` that does not exist is reported with the path it was looked
// for, absolute and under the directory of the declaring file, from every
// working directory, at every depth. It is never found in the working
// directory instead.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeMissingEnvFileIsReportedWhereItIsLookedFor(t *testing.T) {
	tests := []struct {
		name  string
		files fileTree
		want  string
	}{
		{
			name: "main file",
			files: fileTree{
				"proj/compose.yaml": "include:\n  - path: p/inc.yaml\n    env_file: missing.env\n",
				"proj/p/inc.yaml":   reader,
			},
			want: filepath.Join("proj", "missing.env"),
		},
		{
			name: "two levels deep",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    env_file: missing.env\n",
				"proj/p/q/i3.yaml":   reader,
			},
			want: filepath.Join("proj", "p", "q", "missing.env"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadFromEachWorkingDirectory(t, tt.files, func(t *testing.T, root string, _ *types.Project, err error) {
				assert.ErrorContains(t, err, filepath.Join(root, tt.want))
				// the wording of the system differs, not the kind of the error
				assert.Assert(t, errors.Is(err, os.ErrNotExist), "got %v", err)
			})
		})
	}
}

// An `env_file` which is a directory is rejected with its absolute path, at
// every depth.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvFileDirectoryIsRejectedAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":     "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":       "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":    "include:\n  - path: i3.yaml\n    env_file: adir\n",
		"proj/p/q/i3.yaml":      reader,
		"proj/p/q/adir/keep.me": "",
	}, func(t *testing.T, root string, _ *types.Project, err error) {
		assert.ErrorContains(t, err, filepath.Join(root, "proj", "p", "q", "adir")+" is not a file")
	})
}

// An `env_file` may live outside of the project directory of the declaring
// file: it is looked for relative to that file, not to the working directory.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvFileOutsideOfTheProjectAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":  "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    env_file: ../../../shared/shared.env\n",
		"proj/p/q/i3.yaml":   reader,
		"shared/shared.env":  "WHICH=shared\n",
	}, expectWhich("shared"))
}

// The `project_directory` of an include entry is relative to the project
// directory of the file declaring the entry, at every depth. It is the base
// of the relative paths of the included file, and of its default `.env`.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#project_directory
func TestIncludeProjectDirectoryIsRelativeToTheDeclaringFileAtEveryDepth(t *testing.T) {
	const withPaths = `
services:
  s:
    image: alpine
    build: ./ctx
    environment:
      WHICH: "${WHICH:-unset}"
`
	tests := []struct {
		name  string
		files fileTree
		// dir is the project directory the included file must get
		dir   string
		which string
	}{
		{
			name: "declared by the main file",
			files: fileTree{
				"proj/compose.yaml": "include:\n  - path: p/inc.yaml\n    project_directory: pd\n",
				"proj/p/inc.yaml":   withPaths,
				"proj/pd/.env":      "WHICH=pd\n",
			},
			dir: "proj/pd", which: "pd",
		},
		{
			name: "declared one level deep",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include:\n  - path: q/inc2.yaml\n    project_directory: pd\n",
				"proj/p/q/inc2.yaml": withPaths,
				"proj/p/pd/.env":     "WHICH=p/pd\n",
			},
			dir: "proj/p/pd", which: "p/pd",
		},
		{
			name: "declared two levels deep",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    project_directory: pd\n",
				"proj/p/q/i3.yaml":   withPaths,
				"proj/p/q/pd/.env":   "WHICH=p/q/pd\n",
			},
			dir: "proj/p/q/pd", which: "p/q/pd",
		},
		{
			name: "going up the tree",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    project_directory: ../..\n",
				"proj/p/q/i3.yaml":   withPaths,
				"proj/.env":          "WHICH=proj\n",
			},
			dir: "proj", which: "proj",
		},
		{
			name: "the current directory",
			files: fileTree{
				"proj/compose.yaml":  "include: [p/inc.yaml]\n",
				"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
				"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    project_directory: .\n",
				"proj/p/q/i3.yaml":   withPaths,
				"proj/p/q/.env":      "WHICH=p/q\n",
			},
			dir: "proj/p/q", which: "p/q",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadFromEachWorkingDirectory(t, tt.files, func(t *testing.T, root string, p *types.Project, err error) {
				assert.NilError(t, err)
				assert.Equal(t, whichOf(t, p), tt.which)
				assert.Equal(t, p.Services["s"].Build.Context, filepath.Join(root, filepath.FromSlash(tt.dir), "ctx"))
			})
		})
	}
}

// An absolute `project_directory` is used as it is, at every depth.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#project_directory
func TestIncludeAbsoluteProjectDirectoryAtEveryDepth(t *testing.T) {
	shared, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	abs := filepath.Join(shared, "base")
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":  "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":    "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml": "include:\n  - path: i3.yaml\n    project_directory: " + filepath.ToSlash(abs) + "\n",
		"proj/p/q/i3.yaml":   "services:\n  s:\n    image: alpine\n    build: ./ctx\n",
	}, func(t *testing.T, _ string, p *types.Project, err error) {
		assert.NilError(t, err)
		assert.Equal(t, p.Services["s"].Build.Context, filepath.Join(abs, "ctx"))
	})
}

// The `project_directory` and the `env_file` of one entry are resolved against
// the same directory, the one of the declaring file, not one against the
// other.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#long-syntax
func TestIncludeProjectDirectoryAndEnvFileTogetherAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":      "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":        "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":     "include:\n  - path: i3.yaml\n    project_directory: pd\n    env_file: marker.env\n",
		"proj/p/q/i3.yaml":       reader,
		"proj/p/q/marker.env":    "WHICH=declaring\n",
		"proj/p/q/pd/marker.env": "WHICH=project-directory\n",
		"proj/p/q/pd/.env":       "WHICH=dotenv\n",
	}, expectWhich("declaring"))
}

// The relative paths of an included file at any depth, here a build context,
// a bind mount and an `env_file` of a service, are resolved against the
// project directory of that file, whatever the working directory.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#short-syntax
func TestIncludeRelativePathsOfTheModelAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":      "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":        "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":     "include: [r/i3.yaml]\n",
		"proj/p/q/r/service.env": "KEY=value\n",
		"proj/p/q/r/i3.yaml": `
services:
  s:
    image: alpine
    build:
      context: ./ctx
    volumes:
      - ./data:/data
    env_file:
      - ./service.env
`,
	}, func(t *testing.T, root string, p *types.Project, err error) {
		assert.NilError(t, err)
		dir := filepath.Join(root, "proj", "p", "q", "r")
		s := p.Services["s"]
		assert.Equal(t, s.Build.Context, filepath.Join(dir, "ctx"))
		assert.Equal(t, s.Volumes[0].Source, filepath.Join(dir, "data"))
		assert.Equal(t, s.EnvFiles[0].Path, filepath.Join(dir, "service.env"))
	})
}

// The `path` of an include entry, and its list form with overrides, are
// relative to the declaring file at every depth.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#path
func TestIncludePathAtEveryDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":           "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":             "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":          "include:\n  - path:\n      - i3.yaml\n      - ./over/override.yaml\n",
		"proj/p/q/i3.yaml":            "services:\n  s:\n    image: base\n",
		"proj/p/q/over/override.yaml": "services:\n  s:\n    image: override\n",
	}, func(t *testing.T, _ string, p *types.Project, err error) {
		assert.NilError(t, err)
		assert.Equal(t, p.Services["s"].Image, "override")
	})
}

// The relative paths of a file served by a remote resource loader are still
// relative to its local copy when the file includes others with an `env_file`
// or a `project_directory`.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#path
func TestIncludeEnvFileOfAnIncludeOfARemoteFile(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		which string
	}{
		{name: "env_file", entry: "    env_file: marker.env\n", which: "module"},
		{name: "project_directory", entry: "    project_directory: pd\n", which: "dotenv"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			includeCase{
				files: fileTree{
					"remote/module/compose.yaml":    "include:\n  - path: nested/inc.yaml\n" + tt.entry,
					"remote/module/nested/inc.yaml": reader,
					"remote/module/marker.env":      "WHICH=module\n",
					"remote/module/pd/.env":         "WHICH=dotenv\n",
					"proj/compose.yaml":             "include:\n  - remote:module\n",
				},
				options: func(root string) []func(*Options) {
					return []func(*Options){func(o *Options) {
						o.ResourceLoaders = []ResourceLoader{remoteLoader{filepath.Join(root, "remote")}}
					}}
				},
			}.run(t, expectWhich(tt.which))
		})
	}
}

// A file reached through several include paths (a diamond), each declaring the
// same relative `env_file` from its own directory at depth, is loaded once per
// distinct tuple of paths, directories and environment, and gets that env file
// from every working directory.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md
func TestIncludeDiamondWithAnEnvFileAtDepth(t *testing.T) {
	loadFromEachWorkingDirectory(t, fileTree{
		"proj/compose.yaml":      "include: [a/inc.yaml, b/inc.yaml]\n",
		"proj/a/inc.yaml":        "include:\n  - path: ../shared/i3.yaml\n    env_file: ../shared/common.env\n",
		"proj/b/inc.yaml":        "include:\n  - path: ../shared/i3.yaml\n    env_file: ../shared/common.env\n",
		"proj/shared/common.env": "WHICH=common\n",
		"proj/shared/i3.yaml":    reader,
	}, expectWhich("common"))
}

// Loading from a relative working directory, the one a caller may give, keeps
// working: the paths are then relative to the process, as they always were.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md
func TestIncludeEnvFileWithARelativeWorkingDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	writeTree(t, root, fileTree{
		"proj/compose.yaml":   "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":     "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":  "include:\n  - path: i3.yaml\n    env_file: marker.env\n",
		"proj/p/q/i3.yaml":    reader,
		"proj/p/q/marker.env": "WHICH=p/q\n",
	})
	t.Chdir(root)
	p, err := LoadWithContext(t.Context(), types.ConfigDetails{
		WorkingDir:  "proj",
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join("proj", "compose.yaml")}},
	}, func(o *Options) {
		o.SkipNormalization = true
		o.SetProjectName("project", true)
	})
	assert.NilError(t, err)
	assert.Equal(t, whichOf(t, p), "p/q")
}

// A relative `project_directory` declared at depth is also honored when the
// caller gave a relative working directory: the default `.env` of that
// directory is found, relative to the process, as the caller asked.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#project_directory
func TestIncludeProjectDirectoryWithARelativeWorkingDirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	writeTree(t, root, fileTree{
		"proj/compose.yaml":   "include: [p/inc.yaml]\n",
		"proj/p/inc.yaml":     "include: [q/inc2.yaml]\n",
		"proj/p/q/inc2.yaml":  "include:\n  - path: pd/i3.yaml\n    project_directory: pd2\n",
		"proj/p/q/pd/i3.yaml": reader,
		"proj/p/q/pd2/.env":   "WHICH=pd2\n",
		"proj/p/q/pd/.env":    "WHICH=pd\n",
	})
	t.Chdir(root)
	p, err := LoadWithContext(t.Context(), types.ConfigDetails{
		WorkingDir:  "proj",
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join("proj", "compose.yaml")}},
	}, func(o *Options) {
		o.SkipNormalization = true
		o.SetProjectName("project", true)
	})
	assert.NilError(t, err)
	assert.Equal(t, whichOf(t, p), "pd2")
}
