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
	"slices"
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
