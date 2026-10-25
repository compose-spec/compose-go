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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/schema"
	"github.com/compose-spec/compose-go/v2/transform"
	"github.com/compose-spec/compose-go/v2/types"
	"go.yaml.in/yaml/v4"
)

// fuzzSeeds are compose documents exercising the stages of the loader:
// short and long syntaxes, interpolation, extends, include, anchors and merge
// keys, `!reset` and `!override`, and documents of the wrong shape.
var fuzzSeeds = []string{
	"",
	"services: {}",
	"name: demo\nservices:\n  a:\n    build: !0\n",
	"name: demo\nservices: {web: {image: x, build: {context: ., ssh: {key: 1}}}}\n",
	"name: demo\nservices: {web: {image: x, build: {context: ., ssh: [default, key1=/tmp/id_rsa, key2=/tmp/other]}}}\n",
	"name: demo\nservices: {web: {image: busybox, extra_hosts: {host: []}}}\n",
	"name: demo\nservices: {web: {image: x, gpus: [{driver: nvidia, count: !!binary MA==}]}}\n",
	"name: demo\nservices: {web: {image: x, secrets: [{source: pw, mode: -1}]}}\nsecrets: {pw: {external: true}}\n",
	"name: demo\nservices: {web: {image: x, secrets: [{source: pw, mode: '0x1ff'}], configs: [{source: c, mode: '0o440'}]}}\nsecrets: {pw: {external: true}}\nconfigs: {c: {external: true}}\n",
	"name: demo\nservices:\n  web:\n    image: nginx\n",
	`name: demo
services:
  web:
    image: nginx:${TAG:-latest}
    build:
      context: ./web
      args:
        - FOO=bar
    command: ["nginx", "-g", "daemon off;"]
    entrypoint: /entrypoint.sh
    ports:
      - "8080:80"
      - target: 443
        published: "8443"
    volumes:
      - ./html:/usr/share/nginx/html:ro
      - data:/data
      - type: bind
        source: ../logs
        target: /logs
      - type: tmpfs
        target: /tmp
    environment:
      A: b
      EMPTY:
    env_file: [.env]
    depends_on: [db]
    links: [db:database]
    networks: [front]
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost"]
      interval: 30s
    ulimits:
      nproc: 65535
      nofile: {soft: 20000, hard: 40000}
    mem_limit: 512m
    extra_hosts: ["host:127.0.0.1"]
    labels: [a=b]
    deploy:
      resources:
        limits: {cpus: "0.5", memory: 50M}
  db:
    image: postgres
    volumes_from: [web]
    secrets: [pw]
    configs: [cfg]
volumes:
  data:
    driver: local
    driver_opts: {type: none, o: bind, device: ./data}
networks:
  front:
  back: {external: true}
secrets:
  pw: {file: ./pw.txt}
configs:
  cfg: {file: ./cfg.txt}
`,
	"services:\n  a:\n    image: x\n    extends:\n      service: b\n  b:\n    image: y\n    command: bar\n",
	"services:\n  a:\n    extends:\n      file: other.yaml\n      service: base\n",
	"services:\n  a:\n    extends: a\n",
	"include:\n  - other.yaml\n  - path: [other.yaml, more.yaml]\n    project_directory: sub\nservices:\n  a:\n    image: x\n",
	"include: other.yaml\n",
	"x-common: &common\n  image: busybox\n  restart: always\nservices:\n  a:\n    <<: *common\n  b:\n    <<: [*common]\n    image: other\n",
	"services:\n  a:\n    image: x\n    ports: !reset []\n    command: !override [a, b]\n",
	"services:\n  a:\n    image: ${NOPE:?required}\n",
	"services:\n  a:\n    image: ${FOO:-${BAR:-$$x}}\n    labels:\n      - ${FOO}=${EMPTY}\n",
	"services: []\n",
	"services:\n  a: null\n",
	"services:\n  a: 1\n",
	"services:\n  a:\n    volumes: [1, {a: b}, null]\n    networks: 3\n    depends_on: 7\n    build: 5\n",
	"volumes:\n  a: 1\n  b:\n    driver: local\n    driver_opts: x\n",
	"networks: 1\nsecrets: [a]\nconfigs: {a: 1}\n",
	"jobs:\n  j:\n    image: x\n    triggers:\n      manual: true\n",
	"models:\n  m:\n    model: ai/model\nservices:\n  a:\n    models: [m]\n",
	"version: '3'\nservices:\n  a:\n    image: x\n",
	"- a\n- b\n",
	"a: &a [*a]\n",
	"x: &a\n  <<: [*a]\n",
	"x: &a\n  a]<<: [*a]\n",
	"services:\n  a:\n    image: x\n    pre_start:\n      - command: echo\n        volumes: [./a:/a]\n",
	"services:\n  a:\n    image: x\n    develop:\n      watch:\n        - path: ./src\n          action: sync\n          target: /app\n",
	"name: demo\nservices: {web: {image: busybox}}\nx-value: .nan\n",
	"name: demo\nservices:\n  web:\n    image: x\n    build: {context: ., dockerfile: \"\"}\n",
	"name: demo\nservices:\n  web:\n    image: x\n    build: {context: ., dockerfile_inline: \"\"}\n",
	"include: [{}]\n",
	"name: demo\nservices:\n  web:\n    image: x\n    gpus: [{driver: nvidia, count: \"0\"}]\n",
	"name: demo\nservices:\n  web:\n    image: x\n    gpus: [{driver: nvidia, count: '${UNSET:-0}'}]\n",
	"name: demo\nservices: {web: {image: busybox, cpus: \"NaN\"}}\n",
	"name: demo\nservices: {a: {image: x, gpus: all}}\n",
	"name: demo\nservices:\n  web:\n    image: x\n    build: {context: ., ssh: [default=/tmp/id_rsa]}\n",
	"name: demo\nservices:\n  web:\n    image: x\nvolumes:\n  data: {name: \"\"}\n",
	"name: demo\nservices:\n  web:\n    image: x\n    gpus: [{driver: nvidia, count: 0}]\n",
	"services:\n  \"pre_st\\u0061rt\": 1\n",
	"\t:\n",
	"\"unterminated",
	"services:\n  a:\n    image: !!binary Zm9v\n",
}

// fuzzMistypedDocuments declares, one document each, every attribute the
// schema knows with a value of the wrong type (a number, null, a list or a mapping
// holding numbers, entries with a number where a string is expected): the shapes that stages running before
// schema validation, or with it skipped, have to reject without crashing.
func fuzzMistypedDocuments(t testing.TB) []string {
	var spec struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal([]byte(schema.Schema), &spec); err != nil {
		t.Fatal(err)
	}
	// where the attributes of each definition are declared
	parents := map[string]string{
		"container_spec": "services:\n  a:\n    image: x\n",
		"workload_spec":  "services:\n  a:\n    image: x\n",
		"service":        "services:\n  a:\n    image: x\n",
		"volume":         "volumes:\n  a:\n",
		"network":        "networks:\n  a:\n",
		"secret":         "secrets:\n  a:\n",
		"config":         "configs:\n  a:\n",
		"model":          "models:\n  a:\n",
		"healthcheck":    "services:\n  a:\n    image: x\n    healthcheck:\n",
		"deployment":     "services:\n  a:\n    image: x\n    deploy:\n",
		"development":    "services:\n  a:\n    image: x\n    develop:\n",
	}
	var docs []string
	for _, def := range slices.Sorted(maps.Keys(parents)) {
		// the attribute is indented like the last line of the parent, one
		// level deeper when that line opens a mapping
		lines := strings.Split(strings.TrimSuffix(parents[def], "\n"), "\n")
		last := lines[len(lines)-1]
		indent := last[:len(last)-len(strings.TrimLeft(last, " "))]
		if strings.HasSuffix(last, ":") {
			indent += "  "
		}
		for _, attribute := range slices.Sorted(maps.Keys(spec.Defs[def].Properties)) {
			for _, value := range []string{"3", "null", "[3]", "{a: 3}", "[{source: 3, target: 3, path: 3, type: bind}]", "{b: {source: 3, target: 3, path: 3, condition: 3}}"} {
				docs = append(docs, parents[def]+indent+attribute+": "+value+"\n")
			}
		}
	}
	return docs
}

// fuzzCorpusFiles returns the compose files of the loader tests as seeds.
func fuzzCorpusFiles(t testing.TB) []string {
	var docs []string
	for _, pattern := range []string{"*.yaml", "golden/[a-z]*[a-z].yaml", "extends/*.yaml", "include/*.yaml"} {
		names, err := filepath.Glob(filepath.Join("testdata", pattern))
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if strings.HasSuffix(name, ".golden.yaml") {
				continue
			}
			if data, err := os.ReadFile(name); err == nil {
				docs = append(docs, string(data))
			}
		}
	}
	return docs
}

// FuzzNormalize locks that Normalize, which completes a compose model with
// the implicit defaults, reports a model of the wrong shape as an error
// instead of panicking: whatever mapping a YAML document holds, once put in
// canonical form as the loader does, normalizing it must return, even when
// schema validation never saw it.
func FuzzNormalize(f *testing.F) {
	for _, doc := range append(fuzzCorpusFiles(f), fuzzSeeds...) {
		f.Add(doc)
	}
	f.Fuzz(func(_ *testing.T, doc string) {
		var raw any
		if err := yaml.Unmarshal([]byte(doc), &raw); err != nil {
			return
		}
		converted, err := convertToStringKeysRecursive(raw, "")
		if err != nil {
			return
		}
		model, ok := converted.(map[string]any)
		if !ok {
			return
		}
		model, err = transform.Canonical(model, true)
		if err != nil {
			return
		}
		_, _ = Normalize(model, types.Mapping{"FOO": "foo", "EMPTY": ""})
	})
}

// fuzzResourceLoader serves the same small compose file for every `include`
// and `extends.file` reference, so that loading a fuzzed document never reads
// the disk beyond that file.
type fuzzResourceLoader struct{ path string }

func (l fuzzResourceLoader) Accept(string) bool                           { return true }
func (l fuzzResourceLoader) Load(context.Context, string) (string, error) { return l.path, nil }
func (l fuzzResourceLoader) Dir(string) string                            { return filepath.Dir(l.path) }

const fuzzIncluded = `services:
  base:
    image: busybox
    environment:
      FROM: included
    volumes:
      - ./data:/data
volumes:
  shared: {}
`

// documentNodes decodes the YAML documents of doc as trees, which keep what the
// loader sees before it interprets anything: the keys once unescaped, the tags
// and the aliases. It reports false when a document cannot be decoded here:
// this decoder rejects documents the loader accepts (a merge key holding an
// alias to a sequence), so nothing can be said about them.
func documentNodes(doc string) ([]*yaml.Node, bool) {
	var nodes []*yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(doc))
	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			return nodes, errors.Is(err, io.EOF)
		}
		nodes = append(nodes, &node)
	}
}

// anyNode tells whether visit holds for one of the nodes of the trees.
func anyNode(nodes []*yaml.Node, visit func(*yaml.Node) bool) bool {
	for _, node := range nodes {
		if visit(node) || anyNode(node.Content, visit) {
			return true
		}
	}
	return false
}

// hasKey tells whether a mapping of the documents has one of the keys, in any
// case (the fields of the model are matched case-insensitively), or a key that
// cannot be decoded.
func hasKey(nodes []*yaml.Node, keys ...string) bool {
	return anyNode(nodes, func(node *yaml.Node) bool {
		if node.Kind != yaml.MappingNode {
			return false
		}
		for i := 0; i < len(node.Content); i += 2 {
			// the key as the loader reads it: a !!binary key is base64 in the text
			var name string
			if err := node.Content[i].Decode(&name); err != nil {
				return true
			}
			if slices.ContainsFunc(keys, func(key string) bool { return strings.EqualFold(name, key) }) {
				return true
			}
		}
		return false
	})
}

// includesHostFiles tells whether loading doc could read files the fuzzer
// names, which would make the result depend on the host: an `include` along
// with an env_file or a project_directory, or with a custom tag (!reset,
// !override) or an alias, which the loader resolves before it reads the
// include and so can change what it holds. A document that cannot be decoded
// here counts as such. An include naming no path is harmless: the .env it
// then looks for is the one of the working directory, which the harness moves
// to its temporary directory.
func includesHostFiles(doc string) bool {
	nodes, ok := documentNodes(doc)
	if !ok {
		return true
	}
	if !hasKey(nodes, "include") {
		return false
	}
	return hasKey(nodes, "env_file", "project_directory") || anyNode(nodes, func(node *yaml.Node) bool {
		return node.Kind == yaml.AliasNode || (strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!"))
	})
}

// hasEmptyHostAddresses tells whether an `extra_hosts` mapping lists a host with
// no address (`extra_hosts: {host: []}`). The attribute marshals as an empty
// list, which loads as no host at all and is omitted the second time, so such a
// document is not a fixed point of loading.
func hasEmptyHostAddresses(doc string) bool {
	nodes, ok := documentNodes(doc)
	return !ok || anyNode(nodes, func(node *yaml.Node) bool {
		if node.Kind != yaml.MappingNode {
			return false
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			var name string
			if err := node.Content[i].Decode(&name); err != nil {
				return true
			}
			if !strings.EqualFold(name, "extra_hosts") {
				continue
			}
			value := node.Content[i+1]
			if value.Kind == yaml.AliasNode {
				return true
			}
			if value.Kind != yaml.MappingNode {
				continue
			}
			for j := 1; j < len(value.Content); j += 2 {
				addresses := value.Content[j]
				if addresses.Kind == yaml.AliasNode || (addresses.Kind == yaml.SequenceNode && len(addresses.Content) == 0) {
					return true
				}
			}
		}
		return false
	})
}

// hasZeroScalar tells whether a document has an explicit empty string or zero,
// quoted or left to interpolation (`build: ""`, `count: "0"`, `${UNSET:-0}`, `dockerfile_inline: ""`, `name: ""`, `count: 0`,
// `mem_swappiness: 0`). Marshaling omits the zero values while loading defaults
// the missing ones, so such a document is not a fixed point of loading.
func hasZeroScalar(doc string) bool {
	nodes, ok := documentNodes(doc)
	return !ok || anyNode(nodes, func(node *yaml.Node) bool {
		if node.Kind != yaml.ScalarNode {
			return false
		}
		switch node.Tag {
		case "!!str", "!!binary":
			// a quoted zero, or a reference that can interpolate to a zero; the
			// loader reads a binary scalar as the string it encodes
			value := node.Value
			if node.Tag == "!!binary" && node.Decode(&value) != nil {
				return true
			}
			zero, err := strconv.ParseFloat(value, 64)
			return value == "" || strings.Contains(value, "$") || (err == nil && zero == 0)
		case "!!int", "!!float":
			var number float64
			return node.Decode(&number) == nil && number == 0
		}
		// an empty value under a tag the loader does not know (`build: !0`) is
		// read as an empty string
		return node.Value == "" && strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!")
	})
}

// FuzzLoad locks that loading an arbitrary document never panics, whichever
// stages are skipped, and that it terminates in a reasonable time. The low
// bits of mode select the options: SkipValidation, SkipInterpolation,
// SkipNormalization, SkipConsistencyCheck, SkipExtends, SkipInclude, no path
// resolution and Windows path conversion. A project that loads marshals to
// YAML, and to JSON when it was validated against the schema, with the
// interpolation that casts the values (without them, a value such as .nan or
// "NaN" has no JSON form), and, when validated, is a
// fixed point of loading: the YAML it marshals to loads back to the very same
// project.
//
// Documents that could read files of the host are skipped (see
// includesHostFiles). Some documents are not expected to survive a round trip,
// each for a known reason: an explicit empty string or zero is dropped by
// marshaling and defaulted by loading (see hasZeroScalar), an extra_hosts entry
// without an address is marshaled as an empty list and then dropped (see
// hasEmptyHostAddresses).
func FuzzLoad(f *testing.F) {
	dir := f.TempDir()
	included := filepath.Join(dir, "included.yaml")
	if err := os.WriteFile(included, []byte(fuzzIncluded), 0o600); err != nil {
		f.Fatal(err)
	}

	for _, doc := range append(fuzzCorpusFiles(f), fuzzSeeds...) {
		for _, mode := range []uint8{0, 0b1, 0b10, 0b11, 0b10111} {
			f.Add(doc, mode)
		}
	}
	for _, doc := range fuzzMistypedDocuments(f) {
		f.Add(doc, uint8(0))
		f.Add(doc, uint8(0b1))
	}
	f.Fuzz(func(t *testing.T, doc string, mode uint8) {
		if includesHostFiles(doc) {
			t.Skip()
		}
		// an include naming no path looks for a .env in the working directory
		t.Chdir(dir)
		start := time.Now()
		details := types.ConfigDetails{
			WorkingDir:  dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml"), Content: []byte(doc)}},
			Environment: types.Mapping{"FOO": "foo", "EMPTY": "", "TAG": "1.0"},
		}
		options := func(o *Options) {
			o.SkipValidation = mode&(1<<0) != 0
			o.SkipInterpolation = mode&(1<<1) != 0
			o.SkipNormalization = mode&(1<<2) != 0
			o.SkipConsistencyCheck = mode&(1<<3) != 0
			o.SkipExtends = mode&(1<<4) != 0
			o.SkipInclude = mode&(1<<5) != 0
			o.ResolvePaths = mode&(1<<6) == 0
			o.ConvertWindowsPaths = mode&(1<<7) != 0
			o.SkipResolveEnvironment = true
			o.SkipResolveLabels = true
			o.ResourceLoaders = []ResourceLoader{fuzzResourceLoader{included}}
		}

		project, err := LoadWithContext(t.Context(), details, options)
		if elapsed := time.Since(start); elapsed > 30*time.Second {
			t.Fatalf("loading took %s", elapsed)
		}
		if err != nil {
			return
		}
		out, err := project.MarshalYAML()
		if err != nil {
			t.Fatalf("a project that loads cannot be marshaled to YAML: %v", err)
		}
		if _, err := project.MarshalJSON(); err != nil && mode&3 == 0 {
			t.Fatalf("a project that loads cannot be marshaled to JSON: %v", err)
		}

		if mode&1 != 0 || hasZeroScalar(doc) || hasEmptyHostAddresses(doc) {
			return
		}
		details.ConfigFiles = []types.ConfigFile{{Filename: filepath.Join(dir, "reload.yaml"), Content: out}}
		reloaded, err := LoadWithContext(t.Context(), details, func(o *Options) {
			options(o)
			o.SkipValidation = true
			o.SkipInterpolation = true
		})
		if err != nil {
			t.Fatalf("the marshaled project does not load back: %v\n%s", err, out)
		}
		again, err := reloaded.MarshalYAML()
		if err != nil || string(again) != string(out) {
			t.Fatalf("round trip differs (%v):\n%s\n---\n%s", err, out, again)
		}
	})
}

// FuzzLoadMerge locks that merging two compose files never panics, whatever
// they hold (`!reset` and `!override` tags, lists and mappings of the wrong
// type, anchors, extensions), and that a document merged over itself keeps its
// services.
func FuzzLoadMerge(f *testing.F) {
	dir := f.TempDir()
	docs := append(fuzzCorpusFiles(f), fuzzSeeds...)
	for i, doc := range docs {
		f.Add(doc, docs[(i*7+3)%len(docs)])
	}
	f.Add("services:\n  a:\n    image: x\n    ports: [\"80\"]\n", "services:\n  a:\n    ports: !override [\"81\"]\n    command: !reset null\n")
	f.Add("services:\n  a:\n    image: x\n", "services:\n  a: !reset null\n  b: !override {image: y}\n")
	f.Fuzz(func(t *testing.T, base, override string) {
		load := func(contents ...string) (*types.Project, error) {
			var files []types.ConfigFile
			for i, c := range contents {
				files = append(files, types.ConfigFile{Filename: filepath.Join(dir, fmt.Sprintf("compose-%d.yaml", i)), Content: []byte(c)})
			}
			return LoadWithContext(t.Context(), types.ConfigDetails{
				WorkingDir:  dir,
				ConfigFiles: files,
				Environment: types.Mapping{"FOO": "foo", "EMPTY": ""},
			}, func(o *Options) {
				o.SkipInclude = true
				o.SkipExtends = true
				o.SkipResolveEnvironment = true
				o.SkipResolveLabels = true
				o.SetProjectName("fuzz", true)
			})
		}

		_, _ = load(base, override)

		// the short forms (extends, build ssh, gpus: all) are canonicalized when
		// a file is loaded, and then cannot be merged with the same short form
		// of the next file: such a document may fail to merge over itself, but
		// when it does not, it must not gain or lose services
		if alone, err := load(base); err == nil {
			if merged, err := load(base, base); err == nil && len(merged.Services) != len(alone.Services) {
				t.Fatalf("merging a document over itself changed its services: %d, then %d", len(alone.Services), len(merged.Services))
			}
		}
	})
}

// The guard keeping the fuzz harness off the host files must see an include
// declaring an env_file or a project_directory wherever it hides: in a later
// document, behind a merge key a plain decoding rejects, whatever the value.
func TestIncludesHostFiles(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		{"no include", "services:\n  a:\n    image: x\n    env_file: [.env]\n", false},
		{"plain include", "include:\n  - other.yaml\n", false},
		{"env_file", "include:\n  - path: other.yaml\n    env_file: /outside.env\n", true},
		{"project_directory", "include:\n  - path: other.yaml\n    project_directory: /outside\n", true},
		{"project_directory behind a variable", "include:\n  - path: other.yaml\n    project_directory: ${DIR}\n", true},
		{"second document", "services: {}\n---\ninclude:\n  - path: other.yaml\n    env_file: /outside.env\n", true},
		{"merge of an alias to a sequence", "x: &s [a]\nm:\n  <<: *s\ninclude:\n  - path: other.yaml\n    env_file: /outside.env\n", true},
		{"escaped key", "include:\n  - path: other.yaml\n    \"env_f\\x69le\": /outside.env\n", true},
		{"escaped key behind an undecodable merge", "x-list: &s [{}]\nx-merge: {<<: *s}\ninclude: [{path: other.yaml, \"env_f\\x69le\": /outside.env}]\n", true},
		{"not YAML", "include: [\n", true},
		{"env_file in upper case", "include:\n  - path: other.yaml\n    ENV_FILE: /outside.env\n", true},
		{"project_directory in mixed case", "include:\n  - path: other.yaml\n    Project_Directory: /outside\n", true},
		{"path reset", "include: [{path: !reset other.yaml}]\n", true},
		{"list of paths reset", "include: [{path: !reset [other.yaml]}]\n", true},
		{"path through an alias", "x: &p other.yaml\ninclude: [{path: *p}]\n", true},
		{"include without a path", "include: [{}]\n", false},
		{"binary key", "include: [{path: other.yaml, !!binary ZW52X2ZpbGU=: /outside.env}]\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := includesHostFiles(tt.doc); got != tt.want {
				t.Fatalf("includesHostFiles(%q) = %v, want %v", tt.doc, got, tt.want)
			}
		})
	}
}
