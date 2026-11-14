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

package schema

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
)

func TestAttributePaths(t *testing.T) {
	paths := AttributePaths()

	// representative entries across shapes: top level, pattern-keyed
	// mappings, nested objects, sequence items, arbitrary-key mappings
	for _, expected := range []tree.Path{
		"services",
		"services.*",
		"services.*.image",
		"services.*.deploy.update_config.failure_action",
		"services.*.ports.[].target",
		"services.*.environment.*",
		"volumes.*.driver_opts.*",
		"services.*.storage_opt.*",
		"services.*.pre_start.[].storage_opt.*",
		"jobs.*.storage_opt.*",
		"configs.*.file",
	} {
		assert.Check(t, slices.Contains(paths, expected), "missing %s", expected)
	}

	// extension escape hatches must not leak into the inventory as blanket
	// wildcards: they would blanket-accept every undeclared sibling
	for _, path := range paths {
		last := path.Last()
		parent := path.Parent()
		if last == tree.PathMatchAll && (parent == "services" || parent == "networks" || parent == "volumes" || parent == "configs" || parent == "secrets" || parent == "models") {
			continue // the resource maps themselves are legitimately pattern-keyed
		}
		assert.Check(t, !strings.HasSuffix(string(path), ".*.*"), "suspicious blanket wildcard: %s", path)
	}

	assert.Check(t, slices.IsSorted(paths))
}

// A mapping the specification declares must say which keys it accepts: a
// pattern for free keys (driver_opts, labels, storage_opt...) or properties
// for fixed ones. [AttributePaths] derives the inventory from those
// declarations, so an object that declares neither (`{"type": "object"}` alone,
// or `additionalProperties: true`) holds attributes the inventory cannot see,
// and a runtime built on it reports each of their keys as unsupported
// (docker/compose#14505).
func TestSchemaObjectsDeclareTheirKeys(t *testing.T) {
	var root any
	assert.NilError(t, json.Unmarshal([]byte(Schema), &root))

	declaring := []string{"properties", "patternProperties", "additionalProperties", "$ref", "oneOf", "anyOf", "allOf"}
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch n := node.(type) {
		case map[string]any:
			if isObject(n["type"]) {
				declared := slices.ContainsFunc(declaring, func(key string) bool { _, ok := n[key]; return ok })
				assert.Check(t, declared, "%s is an object that declares no keys", path)
				assert.Check(t, n["additionalProperties"] != true, "%s accepts any key through a boolean additionalProperties, which AttributePaths does not record", path)
			}
			for key, child := range n {
				walk(child, path+"/"+key)
			}
		case []any:
			for i, child := range n {
				walk(child, path+"/"+strconv.Itoa(i))
			}
		}
	}
	walk(root, "#")
}

func isObject(schemaType any) bool {
	switch v := schemaType.(type) {
	case string:
		return v == "object"
	case []any:
		return slices.Contains(v, any("object"))
	}
	return false
}
