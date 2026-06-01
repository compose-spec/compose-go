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

package override

import (
	"testing"

	"gotest.tools/v3/assert"
)

// enforceUnicityYAML parses src, runs EnforceUnicityNode, decodes the result
// and returns the decoded map[string]any for shape comparison.
func enforceUnicityYAML(t *testing.T, src string) map[string]any {
	t.Helper()
	root := parseNode(t, src)
	out, err := EnforceUnicityNode(root)
	assert.NilError(t, err)
	var m map[string]any
	assert.NilError(t, out.Decode(&m))
	return m
}

func TestEnforceUnicityNode_EnvironmentLaterWins(t *testing.T) {
	got := enforceUnicityYAML(t, `
services:
  web:
    environment:
      - FOO=1
      - BAR=2
      - FOO=overridden
`)
	env := got["services"].(map[string]any)["web"].(map[string]any)["environment"].([]any)
	// FOO=overridden replaces FOO=1 at the original FOO slot; BAR=2 stays.
	assert.DeepEqual(t, env, []any{"FOO=overridden", "BAR=2"})
}

func TestEnforceUnicityNode_LabelsDeduped(t *testing.T) {
	got := enforceUnicityYAML(t, `
services:
  web:
    labels:
      - com.example.a=1
      - com.example.a=2
      - com.example.b=3
`)
	labels := got["services"].(map[string]any)["web"].(map[string]any)["labels"].([]any)
	assert.DeepEqual(t, labels, []any{"com.example.a=2", "com.example.b=3"})
}

func TestEnforceUnicityNode_PortsShortFormDeduped(t *testing.T) {
	got := enforceUnicityYAML(t, `
services:
  web:
    ports:
      - "8080:80"
      - "8080:80"
      - "8443:443"
`)
	ports := got["services"].(map[string]any)["web"].(map[string]any)["ports"].([]any)
	assert.DeepEqual(t, ports, []any{"8080:80", "8443:443"})
}

func TestEnforceUnicityNode_VolumesByTarget(t *testing.T) {
	got := enforceUnicityYAML(t, `
services:
  web:
    volumes:
      - "./old:/data"
      - "./new:/data"
      - "./logs:/var/log"
`)
	vols := got["services"].(map[string]any)["web"].(map[string]any)["volumes"].([]any)
	assert.DeepEqual(t, vols, []any{"./new:/data", "./logs:/var/log"})
}

func TestEnforceUnicityNode_PortsLongFormByTuple(t *testing.T) {
	got := enforceUnicityYAML(t, `
services:
  web:
    ports:
      - target: 80
        published: 8080
        protocol: tcp
      - target: 80
        published: 8080
        protocol: tcp
      - target: 443
        published: 8443
        protocol: tcp
`)
	ports := got["services"].(map[string]any)["web"].(map[string]any)["ports"].([]any)
	assert.Equal(t, len(ports), 2)
}

func TestEnforceUnicityNode_LeavesNonUnicityPathsAlone(t *testing.T) {
	// services.*.command is overridden, not de-duplicated by EnforceUnicity.
	got := enforceUnicityYAML(t, `
services:
  web:
    command:
      - sh
      - "-c"
      - echo hi
      - echo hi
`)
	cmd := got["services"].(map[string]any)["web"].(map[string]any)["command"].([]any)
	assert.Equal(t, len(cmd), 4, "command sequence is not unicity-controlled")
}

func TestEnforceUnicityNode_NetworkAliasesDeduped(t *testing.T) {
	got := enforceUnicityYAML(t, `
services:
  web:
    networks:
      default:
        aliases:
          - web
          - api
          - web
          - workers
`)
	aliases := got["services"].(map[string]any)["web"].(map[string]any)["networks"].(map[string]any)["default"].(map[string]any)["aliases"].([]any)
	assert.DeepEqual(t, aliases, []any{"web", "api", "workers"})
}
