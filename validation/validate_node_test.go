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

func parseDoc(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &n))
	return &n
}

func TestValidateSemantics_FileObjectMutuallyExclusive(t *testing.T) {
	doc := parseDoc(t, `
secrets:
  s1:
    file: ./secret.txt
    environment: SECRET_VAR
`)
	err := ValidateSemantics(doc)
	assert.ErrorContains(t, err, "mutually exclusive")
}

func TestValidateSemantics_FileObjectMissingRequired(t *testing.T) {
	doc := parseDoc(t, `
secrets:
  s1: {}
`)
	err := ValidateSemantics(doc)
	assert.ErrorContains(t, err, "must be set")
}

func TestValidateSemantics_FileObjectExternalAcceptsEmpty(t *testing.T) {
	doc := parseDoc(t, `
secrets:
  s1:
    external: true
`)
	assert.NilError(t, ValidateSemantics(doc))
}

func TestValidateSemantics_ExternalAndDriverConflict(t *testing.T) {
	doc := parseDoc(t, `
volumes:
  v1:
    external: true
    driver: local
`)
	err := ValidateSemantics(doc)
	assert.ErrorContains(t, err, "conflicting parameters")
}

func TestValidateSemantics_DeviceRequestCountAndIdsExclusive(t *testing.T) {
	doc := parseDoc(t, `
services:
  app:
    deploy:
      resources:
        reservations:
          devices:
            - count: 1
              device_ids: ["GPU-0"]
`)
	err := ValidateSemantics(doc)
	assert.ErrorContains(t, err, "exclusive")
}

func TestValidateSemantics_PortsHostIpInvalid(t *testing.T) {
	doc := parseDoc(t, `
services:
  app:
    ports:
      - target: 80
        host_ip: "not-an-ip"
`)
	err := ValidateSemantics(doc)
	assert.ErrorContains(t, err, "invalid ip address")
}

func TestValidateSemantics_PortsHostIpValid(t *testing.T) {
	doc := parseDoc(t, `
services:
  app:
    ports:
      - target: 80
        host_ip: 127.0.0.1
`)
	assert.NilError(t, ValidateSemantics(doc))
}

func TestValidateSemantics_WatchPathBlank(t *testing.T) {
	doc := parseDoc(t, `
services:
  app:
    develop:
      watch:
        - action: sync
          path: ""
`)
	err := ValidateSemantics(doc)
	assert.ErrorContains(t, err, "can't be blank")
}
