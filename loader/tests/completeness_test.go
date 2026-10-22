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

package tests

// This test keeps the conformance matrix complete by construction. For each
// of the following parts of the JSON schema, every element must either have a
// declared test file or be an explicit, reviewed entry in a known-gaps list.
// Adding an element to the schema without deciding where it is tested fails
// here.
//
//   - the service attributes (the `service` definition and the definitions it
//     composes), declared in coveredBy and knownGaps;
//   - the top-level elements (`services`, `networks`, `volumes`, `secrets`,
//     `configs`, `models`, `jobs`, `include`, `name`, `version`), declared in
//     topLevelCoveredBy and topLevelKnownGaps;
//   - the properties of the structured definitions the matrix relies on
//     (`network`, `volume`, `secret`, `config`, `model`, `healthcheck`, `job`,
//     `include`...), declared in subStructures.
//
// Declaring a file is not enough: the file must also use the element as a
// YAML key (see usesYAMLKey), so that a declaration cannot silently outlive
// the test it points at. The check is deliberately shallow, as a guard
// against a typo, a moved file or a copy-pasted entry; it does not prove that
// an assertion exercises the element, and review remains the judge of that.
//
// Limits: the check stops at the properties of the definitions listed in
// subStructures. It does not descend into the deeper structures (the
// `deploy` resources, the `build` object, the entries of `ports` or
// `volumes`...), whose files are free to be as detailed as their spec
// section demands without a matrix entry for each key.

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// coveredBy declares, for every service attribute of the schema, the test
// file locking its behavior. Paths are relative to the repository root.
// Attributes sharing a file are grouped in that file's spec-link header.
var coveredBy = map[string]string{
	"annotations":         "loader/tests/annotations_test.go",
	"attach":              "loader/tests/attach_test.go",
	"blkio_config":        "loader/tests/blkio_config_test.go",
	"build":               "loader/tests/build_test.go",
	"cap_add":             "loader/tests/cap_add_drop_test.go",
	"cap_drop":            "loader/tests/cap_add_drop_test.go",
	"cgroup":              "loader/tests/cgroup_test.go",
	"cgroup_parent":       "loader/tests/cgroup_parent_test.go",
	"command":             "loader/tests/command_test.go",
	"configs":             "loader/tests/configs_test.go",
	"container_name":      "loader/tests/container_name_test.go",
	"cpu_count":           "loader/tests/cpu_test.go",
	"cpu_percent":         "loader/tests/cpu_test.go",
	"cpu_period":          "loader/tests/cpu_test.go",
	"cpu_quota":           "loader/tests/cpu_test.go",
	"cpu_rt_period":       "loader/tests/cpu_test.go",
	"cpu_rt_runtime":      "loader/tests/cpu_test.go",
	"cpu_shares":          "loader/tests/cpu_test.go",
	"cpus":                "loader/tests/cpu_test.go",
	"cpuset":              "loader/tests/cpu_test.go",
	"credential_spec":     "loader/tests/credential_spec_test.go",
	"depends_on":          "loader/tests/depends_on_test.go",
	"deploy":              "loader/tests/deploy_test.go",
	"develop":             "loader/tests/develop_test.go",
	"device_cgroup_rules": "loader/tests/device_cgroup_rules_test.go",
	"devices":             "loader/tests/devices_test.go",
	"dns":                 "loader/tests/dns_test.go",
	"dns_opt":             "loader/tests/dns_opt_test.go",
	"dns_search":          "loader/tests/dns_test.go",
	"domainname":          "loader/tests/domainname_test.go",
	"entrypoint":          "loader/tests/entrypoint_test.go",
	"env_file":            "loader/tests/env_file_test.go",
	"environment":         "loader/tests/environment_test.go",
	"expose":              "loader/tests/expose_test.go",
	"extends":             "loader/extends_test.go",
	"external_links":      "loader/tests/external_links_test.go",
	"extra_hosts":         "loader/tests/extra_hosts_test.go",
	"gpus":                "loader/tests/gpus_test.go",
	"group_add":           "loader/tests/group_add_test.go",
	"healthcheck":         "loader/tests/healthcheck_test.go",
	"hostname":            "loader/tests/hostname_test.go",
	"image":               "loader/tests/image_test.go",
	"init":                "loader/tests/init_test.go",
	"ipc":                 "loader/tests/ipc_uts_pid_test.go",
	"isolation":           "loader/tests/isolation_test.go",
	"label_file":          "loader/loader_test.go",
	"labels":              "loader/tests/labels_test.go",
	"links":               "loader/tests/links_test.go",
	"logging":             "loader/tests/logging_test.go",
	"mac_address":         "loader/tests/mac_address_test.go",
	"mem_limit":           "loader/tests/mem_limit_test.go",
	"mem_reservation":     "loader/tests/mem_reservation_test.go",
	"mem_swappiness":      "loader/tests/mem_swappiness_test.go",
	"memswap_limit":       "loader/tests/memswap_limit_test.go",
	"models":              "loader/tests/models_test.go",
	"network_mode":        "loader/tests/network_mode_test.go",
	"networks":            "loader/tests/networks_test.go",
	"oom_kill_disable":    "loader/tests/oom_test.go",
	"oom_score_adj":       "loader/tests/oom_test.go",
	"pid":                 "loader/tests/ipc_uts_pid_test.go",
	"pids_limit":          "loader/tests/pids_limit_test.go",
	"platform":            "loader/tests/platform_test.go",
	"ports":               "loader/tests/ports_test.go",
	"post_start":          "loader/tests/service_hooks_test.go",
	"pre_start":           "loader/tests/service_hooks_test.go",
	"pre_stop":            "loader/tests/service_hooks_test.go",
	"privileged":          "loader/tests/privileged_test.go",
	"profiles":            "loader/tests/profiles_test.go",
	"provider":            "loader/tests/provider_test.go",
	"pull_policy":         "loader/tests/pull_policy_test.go",
	"read_only":           "loader/tests/read_only_test.go",
	"restart":             "loader/tests/restart_test.go",
	"runtime":             "loader/tests/runtime_test.go",
	"scale":               "loader/tests/scale_test.go",
	"secrets":             "loader/tests/secrets_test.go",
	"security_opt":        "loader/tests/security_opt_test.go",
	"shm_size":            "loader/tests/shm_size_test.go",
	"stdin_open":          "loader/tests/stdin_tty_test.go",
	"stop_grace_period":   "loader/tests/stop_test.go",
	"stop_signal":         "loader/tests/stop_test.go",
	"storage_opt":         "loader/tests/storage_opt_test.go",
	"sysctls":             "loader/tests/sysctls_test.go",
	"tmpfs":               "loader/tests/tmpfs_test.go",
	"tty":                 "loader/tests/stdin_tty_test.go",
	"ulimits":             "loader/tests/ulimits_test.go",
	"umask":               "loader/tests/umask_test.go",
	"use_api_socket":      "loader/tests/use_api_socket_test.go",
	"user":                "loader/tests/user_test.go",
	"userns_mode":         "loader/tests/userns_mode_test.go",
	"uts":                 "loader/tests/ipc_uts_pid_test.go",
	"volumes":             "loader/tests/volumes_test.go",
	"volumes_from":        "loader/tests/volumes_from_test.go",
	"working_dir":         "loader/tests/working_dir_test.go",
}

// knownGaps lists service attributes that have no dedicated conformance test
// yet, with the reason. Shrinking this list is welcome; growing it is a
// reviewed decision.
var knownGaps = map[string]string{
	"pull_refresh_after": "declared by the schema but absent from the specification text, and not interpreted by compose-go: " +
		"the value is a string passed through as is (pull_policy `every_<duration>` is what drives GetPullPolicy), so there is no behavior to lock yet",
}

// topLevelCoveredBy declares, for every top-level element of the schema, the
// test file locking its behavior. Paths are relative to the repository root.
var topLevelCoveredBy = map[string]string{
	"configs":  "loader/tests/configs_test.go",
	"include":  "loader/include_test.go",
	"jobs":     "loader/tests/triggers_test.go",
	"models":   "loader/tests/models_test.go",
	"name":     "loader/loader_test.go",
	"networks": "loader/tests/networks_test.go",
	"secrets":  "loader/tests/secrets_test.go",
	"volumes":  "loader/tests/volumes_test.go",
}

// topLevelKnownGaps lists top-level elements that have no dedicated
// conformance test yet, with the reason.
var topLevelKnownGaps = map[string]string{
	"services": "every service attribute has its own file, but the `services` map itself (service names, an empty or missing map) has no dedicated test",
	"version":  "obsolete and ignored by the specification: the only test loads it to check a concurrency regression, not the warning the specification describes",
}

// subStructure declares the test files of the properties of a schema
// definition: each property must be used as a YAML key by one of the files,
// unless it is listed in gaps, with the reason.
type subStructure struct {
	def   string
	files []string
	gaps  map[string]string
}

var subStructures = []subStructure{
	{def: "network", files: []string{
		"loader/tests/networks_test.go",
		"loader/tests/network_driver_opts_test.go",
		"loader/tests/network_internal_test.go",
		"loader/tests/network_ipam_test.go",
	}},
	{def: "volume", files: []string{"loader/tests/volumes_test.go"}},
	{def: "secret", files: []string{"loader/tests/secrets_test.go", "loader/tests/template_driver_test.go"}},
	{def: "config", files: []string{"loader/tests/configs_test.go", "loader/tests/template_driver_test.go"}},
	{def: "model", files: []string{"loader/tests/models_test.go"}},
	{def: "include", files: []string{"loader/include_test.go"}},
	{def: "job", files: []string{"loader/tests/triggers_test.go", "loader/tests/profiles_test.go", "loader/extends_test.go"}},
	{def: "schedule", files: []string{"loader/tests/triggers_test.go"}},
	{def: "healthcheck", files: []string{"loader/tests/healthcheck_test.go"}, gaps: map[string]string{
		"disable": "no test yet for `disable: true`, the alternative to `test: [NONE]` documented by the specification",
	}},
}

// schemaDefinitions is the part of the JSON schema this test reads.
type schemaDefinitions struct {
	Properties map[string]any              `json:"properties"`
	Defs       map[string]schemaDefinition `json:"$defs"`
}

type schemaDefinition struct {
	AllOf      []schemaRef    `json:"allOf"`
	OneOf      []schemaObject `json:"oneOf"`
	Properties map[string]any `json:"properties"`
}

type schemaRef struct {
	Ref string `json:"$ref"`
}

type schemaObject struct {
	Properties map[string]any `json:"properties"`
}

// properties returns the property names of a definition: its own and those of
// its oneOf alternatives. With composed, also those of the definitions it
// composes through allOf $refs, which is how a service gets the attributes of
// the container and workload specs.
func (s schemaDefinitions) properties(t *testing.T, def string, composed bool) []string {
	t.Helper()
	d, ok := s.Defs[def]
	assert.Assert(t, ok, "unknown schema definition %q", def)
	names := map[string]bool{}
	for name := range d.Properties {
		names[name] = true
	}
	for _, alternative := range d.OneOf {
		for name := range alternative.Properties {
			names[name] = true
		}
	}
	for _, ref := range d.AllOf {
		if !composed {
			break
		}
		for _, name := range s.properties(t, strings.TrimPrefix(ref.Ref, "#/$defs/"), true) {
			names[name] = true
		}
	}
	assert.Assert(t, len(names) > 0, "no properties found for schema definition %q", def)
	return slices.Sorted(maps.Keys(names))
}

func readSchema(t *testing.T) schemaDefinitions {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "compose-spec.json"))
	assert.NilError(t, err)
	var schema schemaDefinitions
	assert.NilError(t, json.Unmarshal(raw, &schema))
	return schema
}

// usesYAMLKey reports whether the Go test source uses name as a YAML (or JSON)
// key, which is how a conformance test exercises an attribute: its inline
// compose documents declare it. For a top-level element the key must start a
// line of the document, or the document itself, so that `name` is not
// satisfied by every `name:` nested in the file; for the others any
// indentation will do.
func usesYAMLKey(source, name string, topLevel bool) bool {
	// \n: a key following a "\n" escape in an interpreted Go string
	boundary := `(?:^|\\n|[^\w.-])`
	if topLevel {
		boundary = "(?:^|\\\\n|[\"`])"
	}
	return regexp.MustCompile("(?m)" + boundary + `"?` + regexp.QuoteMeta(name) + `"?\s*:`).MatchString(source)
}

// assertDeclared checks every element against its declarations: it is either
// covered by a file that uses it as a YAML key, or a known gap, never both;
// and no declaration refers to an element the schema does not have.
func assertDeclared(t *testing.T, kind string, elements []string, coveredBy map[string]string, gaps map[string]string, topLevel bool) {
	t.Helper()
	for _, element := range elements {
		file, covered := coveredBy[element]
		_, gap := gaps[element]
		switch {
		case covered && gap:
			t.Errorf("%s %q is both covered and a known gap: remove it from the known gaps", kind, element)
		case covered:
			assertFileUses(t, kind, element, []string{file}, topLevel)
		case gap:
			// tracked, nothing to check
		default:
			t.Errorf("schema %s %q has no declared test: add a conformance test in loader/tests/ (see TESTING.md) or a reviewed known-gaps entry", kind, element)
		}
	}
	// stale entries: elements removed from the schema must not linger here
	for element := range coveredBy {
		if !slices.Contains(elements, element) {
			t.Errorf("covered entry %q is not a schema %s anymore", element, kind)
		}
	}
	for element := range gaps {
		if !slices.Contains(elements, element) {
			t.Errorf("known-gaps entry %q is not a schema %s anymore", element, kind)
		}
	}
}

// assertFileUses fails unless one of the files exists and uses the element as
// a YAML key.
func assertFileUses(t *testing.T, kind, element string, files []string, topLevel bool) {
	t.Helper()
	for _, file := range files {
		source, err := os.ReadFile(filepath.Join("..", "..", file))
		if err != nil {
			t.Errorf("%s %q declares test file %s, which cannot be read: %v", kind, element, file, err)
			return
		}
		if usesYAMLKey(string(source), element, topLevel) {
			return
		}
	}
	t.Errorf("%s %q declares %s, but no such file uses it as a YAML key: the declaration is stale or points at the wrong file", kind, element, strings.Join(files, ", "))
}

func TestEveryServiceAttributeHasADeclaredTest(t *testing.T) {
	schema := readSchema(t)
	assertDeclared(t, "service attribute", schema.properties(t, "service", true), coveredBy, knownGaps, false)
}

func TestEveryTopLevelElementHasADeclaredTest(t *testing.T) {
	schema := readSchema(t)
	elements := slices.Sorted(maps.Keys(schema.Properties))
	assert.Assert(t, len(elements) > 0, "no top-level element found in schema")
	assertDeclared(t, "top-level element", elements, topLevelCoveredBy, topLevelKnownGaps, true)
}

func TestEverySubStructurePropertyHasADeclaredTest(t *testing.T) {
	schema := readSchema(t)
	for _, sub := range subStructures {
		t.Run(sub.def, func(t *testing.T) {
			properties := schema.properties(t, sub.def, false)
			for _, property := range properties {
				if _, gap := sub.gaps[property]; gap {
					continue
				}
				assertFileUses(t, sub.def+" property", property, sub.files, false)
			}
			var sources strings.Builder
			for _, file := range sub.files {
				content, err := os.ReadFile(filepath.Join("..", "..", file))
				assert.NilError(t, err)
				sources.Write(content)
			}
			for property := range sub.gaps {
				if !slices.Contains(properties, property) {
					t.Errorf("known-gaps entry %q is not a property of the schema definition %q anymore", property, sub.def)
				}
				if usesYAMLKey(sources.String(), property, false) {
					t.Errorf("%s property %q is a known gap but its files use it: remove it from the gaps", sub.def, property)
				}
			}
		})
	}
}
