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
	"github.com/compose-spec/compose-go/v2/tree"
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

// The path recorded for a !reset or !override tag must be resolved to the
// attribute an extension stands for, at any depth and through list items.
func TestResolveAliasPath(t *testing.T) {
	tests := []struct {
		name string
		path tree.Path
		want tree.Path
	}{
		{"service attribute", "services.web.x-develop", "services.web.develop"},
		{
			"nested extensions through a list item",
			"services.web.x-develop.watch.0.x-initialSync",
			"services.web.develop.watch.0.initial_sync",
		},
		{"nested extension only", "services.web.develop.watch.1.x-initialSync", "services.web.develop.watch.1.initial_sync"},
		{"service named after an extension", "services.x-develop.image", "services.x-develop.image"},
		{"extension outside its parent", "x-develop", "x-develop"},
		{"other extension", "services.web.x-other", "services.web.x-other"},
		{"attribute", "services.web.develop.watch.0.initial_sync", "services.web.develop.watch.0.initial_sync"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, resolveAliasPath(tt.path), tt.want)
		})
	}
}
