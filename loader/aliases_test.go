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
	"slices"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/schema"
	"gotest.tools/v3/assert"
)

// An alias must target an attribute the specification defines, and be an
// extension, so that a typo cannot silently disable a promotion.
func TestExtensionAliasesTargetSpecificationAttributes(t *testing.T) {
	attributes := schema.AttributePaths()
	for _, alias := range extensionAliases {
		t.Run(alias.from, func(t *testing.T) {
			assert.Assert(t, strings.HasPrefix(alias.from, "x-"), "%q is not an extension", alias.from)
			target := alias.parent.Next(alias.to)
			assert.Assert(t, slices.Contains(attributes, target), "%s is not a specification attribute", target)
		})
	}
}
