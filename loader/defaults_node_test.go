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
	"testing"

	"github.com/compose-spec/compose-go/v3/override"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func parseDocNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &n))
	return &n
}

func TestDefaultsNode_PortsProtocolAndMode(t *testing.T) {
	doc := parseDocNode(t, `
services:
  app:
    ports:
      - target: 80
        published: "8080"
`)
	setDefaultValuesNode(doc)

	root := unwrapDocument(doc)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, ports := override.FindKey(app, "ports")
	port := ports.Content[0]
	_, proto := override.FindKey(port, "protocol")
	assert.Assert(t, proto != nil)
	assert.Equal(t, proto.Value, "tcp")
	_, mode := override.FindKey(port, "mode")
	assert.Assert(t, mode != nil)
	assert.Equal(t, mode.Value, "ingress")
}

func TestDefaultsNode_SecretsTarget(t *testing.T) {
	doc := parseDocNode(t, `
services:
  app:
    secrets:
      - source: api_key
`)
	setDefaultValuesNode(doc)

	root := unwrapDocument(doc)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, secrets := override.FindKey(app, "secrets")
	_, tgt := override.FindKey(secrets.Content[0], "target")
	assert.Assert(t, tgt != nil)
	assert.Equal(t, tgt.Value, "/run/secrets/api_key")
}

func TestDefaultsNode_DeviceRequestCount(t *testing.T) {
	doc := parseDocNode(t, `
services:
  app:
    gpus:
      - driver: nvidia
`)
	setDefaultValuesNode(doc)

	root := unwrapDocument(doc)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, gpus := override.FindKey(app, "gpus")
	_, count := override.FindKey(gpus.Content[0], "count")
	assert.Assert(t, count != nil)
	assert.Equal(t, count.Value, "all")
}

func TestDefaultsNode_VolumeBindCreateHostPath(t *testing.T) {
	doc := parseDocNode(t, `
services:
  app:
    volumes:
      - type: bind
        source: ./local
        target: /app
        bind:
          propagation: rshared
`)
	setDefaultValuesNode(doc)

	root := unwrapDocument(doc)
	_, services := override.FindKey(root, "services")
	_, app := override.FindKey(services, "app")
	_, volumes := override.FindKey(app, "volumes")
	_, bind := override.FindKey(volumes.Content[0], "bind")
	_, chp := override.FindKey(bind, "create_host_path")
	assert.Assert(t, chp != nil)
	assert.Equal(t, chp.Value, "true")
}
