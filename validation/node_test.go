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

package validation

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func parseNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func TestValidateNode_AcceptsValidConfig(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    image: nginx
volumes:
  data:
    external: true
`)
	assert.NilError(t, ValidateNode(root))
}

func TestValidateNode_RejectsExternalWithExtraField(t *testing.T) {
	root := parseNode(t, `
volumes:
  data:
    external: true
    driver: local
`)
	err := ValidateNode(root)
	assert.ErrorContains(t, err, "conflicting parameters")
}

func TestValidateNode_RejectsSecretsWithMultipleSources(t *testing.T) {
	root := parseNode(t, `
secrets:
  s1:
    file: ./s1
    environment: S1
`)
	err := ValidateNode(root)
	assert.ErrorContains(t, err, "mutually exclusive")
}

func TestValidateNode_AcceptsSecretsWithDriver(t *testing.T) {
	root := parseNode(t, `
secrets:
  s1:
    driver: custom-driver
`)
	assert.NilError(t, ValidateNode(root))
}

func TestValidateNode_RejectsConfigsMissingSource(t *testing.T) {
	root := parseNode(t, `
configs:
  c1: {}
`)
	err := ValidateNode(root)
	assert.ErrorContains(t, err, "must be set")
}

func TestValidateNode_RejectsBadHostIP(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    ports:
      - target: 80
        host_ip: not-an-ip
`)
	err := ValidateNode(root)
	assert.ErrorContains(t, err, "invalid ip address")
}

func TestValidateNode_AcceptsValidHostIP(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    ports:
      - target: 80
        host_ip: 192.168.1.1
`)
	assert.NilError(t, ValidateNode(root))
}

func TestValidateNode_RejectsDeviceRequestWithCountAndIDs(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    deploy:
      resources:
        reservations:
          devices:
            - count: 1
              device_ids: ["GPU-0"]
`)
	err := ValidateNode(root)
	assert.ErrorContains(t, err, "exclusive")
}

func TestValidateNode_RejectsBlankWatchPath(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    develop:
      watch:
        - action: sync
          path: ""
`)
	err := ValidateNode(root)
	assert.ErrorContains(t, err, "blank")
}

func TestValidateNode_NilSafe(t *testing.T) {
	assert.NilError(t, ValidateNode(nil))
}

// TestCheckVolumeNode_NonMappingErrorIncludesKind covers the Copilot
// review finding that the previous error printed n.Value -- which is
// empty for non-scalar nodes -- and was therefore useless. The fix
// formats the offending node's kind ("sequence" / "mapping" / ...).
func TestCheckVolumeNode_NonMappingErrorIncludesKind(t *testing.T) {
	var root yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(`
volumes:
  bad:
    - element
`), &root))
	err := ValidateNode(&root)
	assert.ErrorContains(t, err, "expected volume")
	assert.ErrorContains(t, err, "sequence")
}
