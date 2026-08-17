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

// This test keeps the conformance matrix complete by construction: every
// service attribute of the JSON schema must either have a declared test file
// or be an explicit, reviewed entry in the known-gaps list. Adding an
// attribute to the schema without deciding where it is tested fails here.

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	"restart":             "loader/tests/restart_test.go",
	"runtime":             "loader/tests/runtime_test.go",
	"scale":               "loader/tests/scale_test.go",
	"secrets":             "loader/tests/secrets_test.go",
	"security_opt":        "loader/tests/security_opt_test.go",
	"stdin_open":          "loader/tests/stdin_tty_test.go",
	"stop_grace_period":   "loader/tests/stop_test.go",
	"stop_signal":         "loader/tests/stop_test.go",
	"storage_opt":         "loader/tests/storage_opt_test.go",
	"sysctls":             "loader/tests/sysctls_test.go",
	"tmpfs":               "loader/tests/tmpfs_test.go",
	"tty":                 "loader/tests/stdin_tty_test.go",
	"ulimits":             "loader/tests/ulimits_test.go",
	"use_api_socket":      "loader/tests/use_api_socket_test.go",
	"user":                "loader/tests/user_test.go",
	"userns_mode":         "loader/tests/userns_mode_test.go",
	"uts":                 "loader/tests/ipc_uts_pid_test.go",
	"volumes":             "loader/tests/volumes_test.go",
	"volumes_from":        "loader/tests/volumes_from_test.go",
	"working_dir":         "loader/tests/working_dir_test.go",
}

// knownGaps lists service attributes that have no dedicated conformance test
// yet. Shrinking this list is welcome; growing it is a reviewed decision.
var knownGaps = map[string]bool{
	"mem_limit":          true,
	"mem_reservation":    true,
	"mem_swappiness":     true,
	"memswap_limit":      true,
	"pull_refresh_after": true,
	"read_only":          true,
	"shm_size":           true,
}

func TestEveryServiceAttributeHasADeclaredTest(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "schema", "compose-spec.json"))
	assert.NilError(t, err)
	var schema struct {
		Defs struct {
			Service struct {
				Properties map[string]any `json:"properties"`
			} `json:"service"`
		} `json:"$defs"`
	}
	assert.NilError(t, json.Unmarshal(raw, &schema))
	properties := schema.Defs.Service.Properties
	assert.Assert(t, len(properties) > 0, "no service properties found in schema")

	for attr := range properties {
		file, covered := coveredBy[attr]
		gap := knownGaps[attr]
		switch {
		case covered && gap:
			t.Errorf("attribute %q is both covered and a known gap: remove it from knownGaps", attr)
		case covered:
			if _, err := os.Stat(filepath.Join("..", "..", file)); err != nil {
				t.Errorf("attribute %q declares test file %s, which does not exist", attr, file)
			}
		case gap:
			// tracked, nothing to check
		default:
			t.Errorf("schema attribute %q has no declared test: add a conformance test in loader/tests/ (see TESTING.md) or a reviewed knownGaps entry", attr)
		}
	}
	// stale entries: attributes removed from the schema must not linger here
	for attr := range coveredBy {
		if _, ok := properties[attr]; !ok {
			t.Errorf("coveredBy entry %q is not a schema service attribute anymore", attr)
		}
	}
	for attr := range knownGaps {
		if _, ok := properties[attr]; !ok {
			t.Errorf("knownGaps entry %q is not a schema service attribute anymore", attr)
		}
	}
}
