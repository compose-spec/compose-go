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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// resolvesAs is the YAML-in / YAML-out assertion of this package: the "in"
// document is run through ResolveRelativePaths and must come out as the "want"
// document. Documents are written in canonical form (lists rather than
// "string or list" shorthands): the loader canonicalizes short syntaxes before
// paths are resolved.
//
// Three placeholders keep the documents OS independent and are expanded
// (with forward slashes) before parsing:
//   - <wd>    the working directory relative paths are resolved against
//   - <other> another absolute directory (an already absolute path)
//   - <home>  the user's home directory ("~" expansion)
//
// Strings are compared with forward slashes so that the Windows separator
// produced by filepath.Join does not matter; what is locked is *which* path is
// produced, not how the OS spells it.
func resolvesAs(t *testing.T, remotes []RemoteResource, in, want string) {
	t.Helper()
	wd, other, home := t.TempDir(), t.TempDir(), t.TempDir()
	setHome(t, home)

	expand := func(doc string) map[string]any {
		doc = strings.NewReplacer(
			"<wd>", filepath.ToSlash(wd),
			"<other>", filepath.ToSlash(other),
			"<home>", filepath.ToSlash(home),
		).Replace(doc)
		var m map[string]any
		assert.NilError(t, yaml.Unmarshal([]byte(doc), &m))
		return m
	}

	got := expand(in)
	assert.NilError(t, ResolveRelativePaths(got, wd, remotes))
	assert.DeepEqual(t, slashed(got), slashed(expand(want)))
}

// slashed converts every string of a decoded YAML tree to forward slashes.
func slashed(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = slashed(e)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = slashed(e)
		}
		return out
	case string:
		return filepath.ToSlash(v)
	}
	return v
}

// resolveFails runs ResolveRelativePaths on "in" and returns its error.
func resolveFails(t *testing.T, in string) error {
	t.Helper()
	var m map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(in), &m))
	err := ResolveRelativePaths(m, t.TempDir(), nil)
	assert.Assert(t, err != nil, "expected an error")
	return err
}

// build.context is a local directory unless it points to a remote or
// builder-specific context: only local paths are made absolute.
// https://github.com/compose-spec/compose-spec/blob/main/build.md#context
func TestResolveBuildContext(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"relative", "./app", "<wd>/app"},
		{"bare relative", "app", "<wd>/app"},
		{"dot segments are cleaned", "./a/../b", "<wd>/b"},
		{"dot is the working directory", ".", "<wd>"},
		{"absolute", "<other>/app", "<other>/app"},
		{"home", "~/app", "<home>/app"},
		{"https", "https://example.com/ctx.tar.gz", "https://example.com/ctx.tar.gz"},
		{"http", "http://example.com/ctx.tar.gz", "http://example.com/ctx.tar.gz"},
		{"git scheme", "git://example.com/repo.git", "git://example.com/repo.git"},
		{"ssh scheme", "ssh://git@example.com/repo.git", "ssh://git@example.com/repo.git"},
		{"github shorthand", "github.com/org/repo", "github.com/org/repo"},
		{"scp-like git", "git@github.com:org/repo.git", "git@github.com:org/repo.git"},
		{"builder specific scheme", "docker-image://alpine", "docker-image://alpine"},
		{"service reference", "service:base", "service:base"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvesAs(t, nil,
				"services: {web: {build: {context: '"+tt.in+"'}}}",
				"services: {web: {build: {context: '"+tt.want+"'}}}")
		})
	}

	t.Run("additional_contexts", func(t *testing.T) {
		resolvesAs(t, nil, `
services:
  web:
    build:
      context: .
      additional_contexts:
        local: ./lib
        absolute: <other>/lib
        remote: https://example.com/ctx.tar.gz
        image: docker-image://alpine
        other_service: service:base
`, `
services:
  web:
    build:
      context: <wd>
      additional_contexts:
        local: <wd>/lib
        absolute: <other>/lib
        remote: https://example.com/ctx.tar.gz
        image: docker-image://alpine
        other_service: service:base
`)
	})
}

// A job is a service-like workload: its paths resolve like a service's.
func TestResolveJobPaths(t *testing.T) {
	resolvesAs(t, nil, `
jobs:
  migrate:
    build:
      context: ./migrate
      additional_contexts:
        lib: ./lib
    env_file:
      - path: ./job.env
    label_file:
      - ./job.labels
    extends:
      file: ./base.yaml
    volumes:
      - type: bind
        source: ./data
        target: /data
`, `
jobs:
  migrate:
    build:
      context: <wd>/migrate
      additional_contexts:
        lib: <wd>/lib
    env_file:
      - path: <wd>/job.env
    label_file:
      - <wd>/job.labels
    extends:
      file: <wd>/base.yaml
    volumes:
      - type: bind
        source: <wd>/data
        target: /data
`)
}

// env_file and label_file entries are paths relative to the project
// directory; the long syntax carries the path in its "path" attribute and the
// other attributes must be left alone.
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#env_file
func TestResolveEnvAndLabelFiles(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    env_file:
      - path: ./a.env
      - path: ./b.env
        required: false
      - path: <other>/c.env
      - path: ~/d.env
    label_file:
      - ./a.labels
      - ~/b.labels
    pre_start:
      - env_file:
          - path: ./hook.env
        label_file:
          - ./hook.labels
`, `
services:
  web:
    env_file:
      - path: <wd>/a.env
      - path: <wd>/b.env
        required: false
      - path: <other>/c.env
      - path: <home>/d.env
    label_file:
      - <wd>/a.labels
      - <home>/b.labels
    pre_start:
      - env_file:
          - path: <wd>/hook.env
        label_file:
          - <wd>/hook.labels
`)
}

// An empty path stays empty rather than becoming the working directory, so
// that later validation can report it.
func TestResolveEmptyPathIsKept(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    env_file:
      - path: ""
`, `
services:
  web:
    env_file:
      - path: ""
`)
}

// absPath only knows strings (and lists of them): any other type is a
// document error reported with its type, not silently dropped.
func TestResolveUnexpectedPathType(t *testing.T) {
	err := resolveFails(t, `
services:
  web:
    env_file:
      - path: 42
`)
	assert.ErrorContains(t, err, "unexpected type int")
}

// extends.file is a local file made absolute, unless one of the remote
// resource matchers of the loader claims it (a remote file is resolved by the
// loader itself, not on the local disk).
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#extends
func TestResolveExtendsFile(t *testing.T) {
	remote := func(p string) bool { return strings.HasPrefix(p, "git://") }
	other := func(p string) bool { return strings.HasPrefix(p, "oci://") }

	resolvesAs(t, []RemoteResource{remote, other}, `
services:
  local:
    extends:
      file: ./base.yaml
      service: base
  absolute:
    extends:
      file: <other>/base.yaml
      service: base
  git:
    extends:
      file: git://example.com/repo.git
      service: base
  oci:
    extends:
      file: oci://example.com/base
      service: base
  inherited:
    extends:
      service: base
`, `
services:
  local:
    extends:
      file: <wd>/base.yaml
      service: base
  absolute:
    extends:
      file: <other>/base.yaml
      service: base
  git:
    extends:
      file: git://example.com/repo.git
      service: base
  oci:
    extends:
      file: oci://example.com/base
      service: base
  inherited:
    extends:
      service: base
`)

	// without a matcher the same reference is taken for a local path
	resolvesAs(t, nil, `
services:
  git:
    extends:
      file: git://example.com/repo.git
`, `
services:
  git:
    extends:
      file: <wd>/git:/example.com/repo.git
`)
}

// configs and secrets sources are made absolute; the ones defined inline or
// from the environment carry no file and are left untouched.
// https://github.com/compose-spec/compose-spec/blob/main/08-configs.md
func TestResolveConfigsSecrets(t *testing.T) {
	resolvesAs(t, nil, `
configs:
  from_file:
    file: ./app.conf
  absolute:
    file: <other>/app.conf
  inline:
    content: hello
  from_env:
    environment: APP_CONF
secrets:
  from_file:
    file: ~/secret.txt
  from_env:
    environment: SECRET
`, `
configs:
  from_file:
    file: <wd>/app.conf
  absolute:
    file: <other>/app.conf
  inline:
    content: hello
  from_env:
    environment: APP_CONF
secrets:
  from_file:
    file: <home>/secret.txt
  from_env:
    environment: SECRET
`)
}

// Paths that are absolute for *another* OS than the one running compose are
// left untouched (a Windows client driving a Unix daemon, or the opposite),
// instead of being glued to the working directory.
func TestResolveForeignAbsolutePaths(t *testing.T) {
	resolvesAs(t, nil, `
secrets:
  windows_drive:
    file: 'C:\secrets\token'
  windows_drive_slash:
    file: 'C:/secrets/token'
  windows_unc:
    file: '\\\\host\\share\\token'
  unix:
    file: /run/secrets/token
`, `
secrets:
  windows_drive:
    file: 'C:\secrets\token'
  windows_drive_slash:
    file: 'C:/secrets/token'
  windows_unc:
    file: '\\\\host\\share\\token'
  unix:
    file: /run/secrets/token
`)
}

// build.ssh maps an identifier to an optional key path: a null path (the
// "default" agent) stays null, a path is made absolute.
func TestResolveBuildSSH(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    build:
      context: .
      ssh:
        default:
        key: ./id_ed25519
        abs: <other>/id_ed25519
`, `
services:
  web:
    build:
      context: <wd>
      ssh:
        default:
        key: <wd>/id_ed25519
        abs: <other>/id_ed25519
`)
}

// The path of a key may be written as a number or a boolean in a mapping: the
// model reads it as the text it is written with, so it is resolved like that
// text, rather than left relative and resolved only when it is read back.
func TestResolveBuildSSHScalarPath(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    build:
      context: .
      ssh:
        number: 1
        flag: true
        text: "1"
`, `
services:
  web:
    build:
      context: <wd>
      ssh:
        number: <wd>/1
        flag: <wd>/true
        text: <wd>/1
`)
}

// An empty key path stays empty, like a null one, rather than becoming the
// working directory: an empty path is not a key to find, and the working
// directory is no key at all.
func TestResolveBuildSSHEmptyPath(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    build:
      context: .
      ssh:
        default: ""
        key: ./id_ed25519
`, `
services:
  web:
    build:
      context: <wd>
      ssh:
        default: ""
        key: <wd>/id_ed25519
`)
}

// The other attributes resolved as a path that may be Unix or Windows keep an
// empty value empty too, whatever the attribute.
func TestResolveEmptyUnixPathIsKept(t *testing.T) {
	resolvesAs(t, nil, `
configs:
  from_file:
    file: ""
secrets:
  from_file:
    file: ""
volumes:
  bind:
    driver: local
    driver_opts: {type: none, o: bind, device: ""}
`, `
configs:
  from_file:
    file: ""
secrets:
  from_file:
    file: ""
volumes:
  bind:
    driver: local
    driver_opts: {type: none, o: bind, device: ""}
`)
}

// Only "~" alone or followed by a separator is the home directory. A "~"
// followed by a name is another user's home, which is not looked up: the value
// is a relative path like any other, and is not expanded to the current
// user's home.
func TestResolveTildeFollowedByName(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    build:
      context: ~bob/app
      ssh:
        key: ~bob/id_ed25519
        mine: ~/id_ed25519
`, `
services:
  web:
    build:
      context: <wd>/~bob/app
      ssh:
        key: <wd>/~bob/id_ed25519
        mine: <home>/id_ed25519
`)
}

// Long-syntax bind mounts get an absolute source; every other mount type
// keeps its source (a volume name or nothing); short syntax is left to the
// later normalization.
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#long-syntax-5
func TestResolveServiceVolumes(t *testing.T) {
	resolvesAs(t, nil, `
services:
  web:
    volumes:
      - ./data:/data
      - named:/named
      - type: bind
        source: ./relative
        target: /relative
      - type: bind
        source: <other>/absolute
        target: /absolute
      - type: bind
        source: ~/home
        target: /home
      - type: bind
        source: /unix/absolute
        target: /unix
      - type: bind
        source: 'C:\\windows\\absolute'
        target: /windows
      - type: volume
        source: named
        target: /named
      - type: volume
        target: /anonymous
      - type: tmpfs
        target: /tmp
    pre_start:
      - volumes:
          - type: bind
            source: ./hook
            target: /hook
jobs:
  migrate:
    volumes:
      - type: bind
        source: ./job
        target: /job
`, `
services:
  web:
    volumes:
      - ./data:/data
      - named:/named
      - type: bind
        source: <wd>/relative
        target: /relative
      - type: bind
        source: <other>/absolute
        target: /absolute
      - type: bind
        source: <home>/home
        target: /home
      - type: bind
        source: /unix/absolute
        target: /unix
      - type: bind
        source: 'C:\\windows\\absolute'
        target: /windows
      - type: volume
        source: named
        target: /named
      - type: volume
        target: /anonymous
      - type: tmpfs
        target: /tmp
    pre_start:
      - volumes:
          - type: bind
            source: <wd>/hook
            target: /hook
jobs:
  migrate:
    volumes:
      - type: bind
        source: <wd>/job
        target: /job
`)
}

// A bind mount without a source, or with an empty one, can not be resolved:
// the error carries the same message as the engine's for the same mistake. An
// empty source is not the working directory.
func TestResolveBindMountWithoutSource(t *testing.T) {
	for name, mount := range map[string]string{
		"no source":    "- type: bind\n        target: /data",
		"empty source": "- type: bind\n        source: \"\"\n        target: /data",
	} {
		t.Run(name, func(t *testing.T) {
			err := resolveFails(t, "services:\n  web:\n    volumes:\n      "+mount+"\n")
			assert.ErrorContains(t, err, `invalid mount config for type "bind": field Source must not be empty`)
		})
	}
}

// A local volume declared as a bind mount through driver_opts
// (type: none, o: bind, device: <path>) is a host path: its device must be
// resolved. Any other volume (other driver, other options, no options, empty
// declaration) is not.
// https://github.com/compose-spec/compose-spec/blob/main/07-volumes.md
func TestResolveVolumeDriverOpts(t *testing.T) {
	resolvesAs(t, nil, `
volumes:
  bind_relative:
    driver: local
    driver_opts: {type: none, o: bind, device: ./data}
  bind_absolute:
    driver: local
    driver_opts: {type: none, o: bind, device: <other>/data}
  bind_home:
    driver: local
    driver_opts: {type: none, o: bind, device: ~/data}
  bind_without_device:
    driver: local
    driver_opts: {type: none, o: bind}
  other_option:
    driver: local
    driver_opts: {type: nfs, o: "addr=10.0.0.1,rw", device: ":/export"}
  no_driver_opts:
    driver: local
  other_driver:
    driver: custom
    driver_opts: {o: bind, device: ./data}
  no_driver:
    driver_opts: {o: bind, device: ./data}
  empty:
`, `
volumes:
  bind_relative:
    driver: local
    driver_opts: {type: none, o: bind, device: <wd>/data}
  bind_absolute:
    driver: local
    driver_opts: {type: none, o: bind, device: <other>/data}
  bind_home:
    driver: local
    driver_opts: {type: none, o: bind, device: <home>/data}
  bind_without_device:
    driver: local
    driver_opts: {type: none, o: bind}
  other_option:
    driver: local
    driver_opts: {type: nfs, o: "addr=10.0.0.1,rw", device: ":/export"}
  no_driver_opts:
    driver: local
  other_driver:
    driver: custom
    driver_opts: {o: bind, device: ./data}
  no_driver:
    driver_opts: {o: bind, device: ./data}
  empty:
`)
}

// develop.watch paths are made absolute AND followed through symbolic links,
// so that file watching observes the real directory. A path that does not
// exist (or has no link) is only made absolute.
// https://github.com/compose-spec/compose-spec/blob/main/develop.md
func TestResolveWatchPath(t *testing.T) {
	// EvalSymlinks: on some systems the temp dir itself lives behind a link
	// (/var -> /private/var on macOS) which would be resolved too.
	wd, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	assert.NilError(t, os.MkdirAll(filepath.Join(wd, "real", "src"), 0o755))

	resolveWatch := func(t *testing.T, path string) string {
		t.Helper()
		m := map[string]any{"services": map[string]any{"web": map[string]any{
			"develop": map[string]any{"watch": []any{
				map[string]any{"path": path, "action": "sync"},
			}},
		}}}
		assert.NilError(t, ResolveRelativePaths(m, wd, nil))
		watch := m["services"].(map[string]any)["web"].(map[string]any)["develop"].(map[string]any)["watch"].([]any)
		return watch[0].(map[string]any)["path"].(string)
	}

	t.Run("existing directory is only made absolute", func(t *testing.T) {
		assert.Equal(t, resolveWatch(t, "./real/src"), filepath.Join(wd, "real", "src"))
	})
	t.Run("missing path is only made absolute", func(t *testing.T) {
		assert.Equal(t, resolveWatch(t, "./missing"), filepath.Join(wd, "missing"))
	})

	t.Run("symbolic link is followed", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			// utils.getSymbolinkLink starts from the root separator and
			// joins the drive letter to it (\ + C: = \C:), a path that never
			// exists: on Windows no link is ever found, so there is no
			// behavior worth locking yet.
			t.Skip("symbolic links are not followed on Windows")
		}
		link := filepath.Join(wd, "link")
		if err := os.Symlink(filepath.Join(wd, "real"), link); err != nil {
			t.Skipf("cannot create symbolic links on this system: %v", err)
		}
		assert.Equal(t, resolveWatch(t, "./link/src"), filepath.Join(wd, "real", "src"))
		assert.Equal(t, resolveWatch(t, link), filepath.Join(wd, "real"))
	})
}

// The path of a watch rule must be a string like any other path.
func TestResolveWatchPathUnexpectedType(t *testing.T) {
	err := resolveFails(t, `
services:
  web:
    develop:
      watch:
        - path: 42
`)
	assert.ErrorContains(t, err, "unexpected type int")
}

// absPath accepts a list of paths as well as a single one, resolving each
// entry, and fails on the first entry that is not a path.
func TestAbsPathList(t *testing.T) {
	wd, other := t.TempDir(), t.TempDir()
	r := relativePathsResolver{workingDir: wd}

	got, err := r.absPath([]any{"a.env", filepath.Join(other, "b.env"), ""})
	assert.NilError(t, err)
	assert.DeepEqual(t, got, []any{filepath.Join(wd, "a.env"), filepath.Join(other, "b.env"), ""})

	_, err = r.absPath([]any{"a.env", 42})
	assert.ErrorContains(t, err, "unexpected type int")
}

// A build context or an extends file that is not a path is a document error
// reported with its type, in the same way as any other path: resolving the
// paths of a model not checked against the schema must not crash.
func TestResolveUnexpectedContextAndExtendsType(t *testing.T) {
	for name, doc := range map[string]struct{ in, want string }{
		"build context number": {"services:\n  web:\n    build:\n      context: 42\n", "unexpected type int"},
		"build context empty":  {"services:\n  web:\n    build:\n      context:\n", "unexpected type <nil>"},
		"build context list":   {"services:\n  web:\n    build:\n      context: [.]\n", "unexpected type []interface {}"},
		"extends file number":  {"services:\n  web:\n    extends:\n      file: 42\n      service: base\n", "unexpected type int"},
		"extends file empty":   {"services:\n  web:\n    extends:\n      file:\n      service: base\n", "unexpected type <nil>"},
	} {
		t.Run(name, func(t *testing.T) {
			assert.ErrorContains(t, resolveFails(t, doc.in), doc.want)
		})
	}
}

// An error of the resolution names the attribute whose value is rejected, like
// the other errors of the loader do, so that it can be found in a document:
// "unexpected type int" alone says nothing of where the number is. The entries
// of a list are all written "[]", as in the paths of the transformations.
func TestResolveErrorNamesTheAttribute(t *testing.T) {
	for name, doc := range map[string]struct{ in, want string }{
		"build context":      {"services:\n  web:\n    build:\n      context: 42\n", "services.web.build.context: unexpected type int"},
		"job build context":  {"jobs:\n  web:\n    build:\n      context: 42\n", "jobs.web.build.context: unexpected type int"},
		"extends file":       {"services:\n  web:\n    extends:\n      file: [x]\n      service: base\n", "services.web.extends.file: unexpected type []interface {}"},
		"env file path":      {"services:\n  web:\n    env_file:\n      - path: 42\n", "services.web.env_file.[].path: unexpected type int"},
		"label file":         {"services:\n  web:\n    label_file: [42]\n", "services.web.label_file.[]: unexpected type int"},
		"watch path":         {"services:\n  web:\n    develop:\n      watch:\n        - path: 42\n", "services.web.develop.watch.[].path: unexpected type int"},
		"pre_start env file": {"services:\n  web:\n    pre_start:\n      - env_file:\n          - path: 42\n", "services.web.pre_start.[].env_file.[].path: unexpected type int"},
		"bind mount":         {"services:\n  web:\n    volumes:\n      - type: bind\n        target: /data\n", `services.web.volumes.[]: invalid mount config for type "bind"`},
	} {
		t.Run(name, func(t *testing.T) {
			assert.ErrorContains(t, resolveFails(t, doc.in), doc.want)
		})
	}
}

// What is not in canonical form is left as written by the resolution of
// paths, which only completes the attributes it recognizes: a bind mount whose
// source is not a string, or a volume that is not a definition, is left for
// the schema validation to reject, not made a crash here.
func TestResolveLeavesMisshapenVolumesAlone(t *testing.T) {
	for name, doc := range map[string]string{
		"bind source number": "services:\n  web:\n    volumes:\n      - type: bind\n        source: 42\n        target: /data\n",
		"volume is a string": "volumes:\n  data: local\n",
		"driver_opts number": "volumes:\n  data:\n    driver: local\n    driver_opts: 42\n",
		"driver_opts list":   "volumes:\n  data:\n    driver: local\n    driver_opts: [o=bind]\n",
	} {
		t.Run(name, func(t *testing.T) {
			resolvesAs(t, nil, doc, doc)
		})
	}
}
