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
	"slices"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/schema"
	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// The issue #13149 scenario: the runtime declares the full specification
// inventory minus the swarm-only attributes it does not implement, and the
// loader reports exactly those when a compose file uses them.
func TestLoadReportsUnsupportedAttributes(t *testing.T) {
	// removing a whole subtree ("services.*.credential_spec" and everything
	// underneath) yields a single report on the subtree root; removing only
	// some leaves ("...update_config.{parallelism,failure_action}") keeps the
	// screening leaf-precise — siblings still declared keep the descent open
	supported := slices.DeleteFunc(slices.Clone(schema.AttributePaths()), func(p tree.Path) bool {
		return strings.HasPrefix(string(p), "services.*.credential_spec") ||
			p == "services.*.deploy.update_config.failure_action" ||
			p == "services.*.deploy.update_config.parallelism"
	})

	var reported []string
	_, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: []byte(`
services:
  app:
    image: myapp
    ports:
      - "8080:80"
    environment:
      DEBUG: "1"
    deploy:
      replicas: 2
      update_config:
        parallelism: 1
        failure_action: rollback
    credential_spec:
      file: creds.json
    x-custom:
      anything: goes
`)}},
	}, func(options *Options) {
		options.SetProjectName("screening", true)
		options.SupportedAttributes = supported
		options.UnsupportedAttribute = func(path tree.Path) {
			reported = append(reported, path.String())
		}
	})
	assert.NilError(t, err)
	slices.Sort(reported)
	// subtree removal reports once at its root, leaf removal reports each
	// leaf; x-* untouched
	assert.DeepEqual(t, reported, []string{
		"services.app.credential_spec",
		"services.app.deploy.update_config.failure_action",
		"services.app.deploy.update_config.parallelism",
	})
}

// Every attribute of a fully supported file passes the complete inventory
// silently — the screening introduces no false positives on the canonical
// forms (short syntaxes included).
func TestLoadSupportedAttributesNoFalsePositives(t *testing.T) {
	var reported []string
	_, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yaml", Content: []byte(`
services:
  db:
    image: mysql:8
    volumes:
      - data:/var/lib/mysql
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    networks:
      - backend
    healthcheck:
      test: ["CMD", "mysqladmin", "ping"]
      interval: 10s
    deploy:
      resources:
        limits:
          memory: 1g
volumes:
  data:
networks:
  backend:
`)}},
	}, func(options *Options) {
		options.SetProjectName("clean", true)
		options.SupportedAttributes = schema.AttributePaths()
		options.UnsupportedAttribute = func(path tree.Path) {
			reported = append(reported, path.String())
		}
	})
	assert.NilError(t, err)
	assert.Equal(t, len(reported), 0)
}
