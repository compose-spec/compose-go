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

package types

import (
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// TestExportedStructFieldsHaveYAMLTagOrCustomDecode walks every exported
// struct type in this package and asserts that each exported field
// either declares a yaml tag or that the type carries an
// UnmarshalYAML method. yaml.v4 is strict about field discovery (it
// matches lowercased field names but the codebase has been bitten by
// snake_case keys that map to PascalCase field names, e.g.
// WeightDevice.Path / Weight before the v3 tags landed), so the test
// blocks future contributors from adding an untagged exported field
// to a struct that compose decodes from YAML.
//
// The reflect walk picks up every type reachable from a Project value
// (the top-level entry point) plus the standalone Compose configuration
// types it references but does not embed (IncludeConfig, ConfigFile,
// ConfigDetails, ExtendsConfig). Adding a new exported struct to the
// package therefore requires either explicit yaml tags or an
// UnmarshalYAML implementation -- both ways are accepted.
func TestExportedStructFieldsHaveYAMLTagOrCustomDecode(t *testing.T) {
	// Seed roots: types the loader projects onto at the end of the
	// pipeline. ConfigDetails / ConfigFile are caller-input shapes
	// (the user fills them before calling LoadWithContext) and are
	// intentionally untagged, so they are out of scope.
	roots := []reflect.Type{
		reflect.TypeOf(Project{}),
		reflect.TypeOf(IncludeConfig{}),
		reflect.TypeOf(ExtendsConfig{}),
	}

	visited := map[reflect.Type]bool{}
	for _, r := range roots {
		visit(t, r, visited)
	}
}

// visit recursively descends into struct fields, slices, maps, pointers
// and arrays. Every struct it reaches is checked for the
// yaml-tag-or-UnmarshalYAML invariant.
func visit(t *testing.T, typ reflect.Type, visited map[reflect.Type]bool) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if visited[typ] {
		return
	}
	visited[typ] = true

	switch typ.Kind() {
	case reflect.Struct:
		// Types out of compose-go (yaml.Node, time.Duration, ...) are out
		// of scope: we only check what the package owns.
		if !strings.HasPrefix(typ.PkgPath(), "github.com/compose-spec/compose-go/v3") {
			return
		}
		if implementsUnmarshalYAML(typ) {
			// Custom decode opts the type out of the field-tag invariant
			// by definition. Still descend into the field types in case
			// they have nested structs that should be checked.
			descend(t, typ, visited)
			return
		}
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if !f.IsExported() {
				continue
			}
			if !hasYAMLTag(f) {
				t.Errorf("%s.%s: exported field has no yaml tag and the type has no UnmarshalYAML method",
					typ.String(), f.Name)
			}
		}
		descend(t, typ, visited)
	case reflect.Slice, reflect.Array, reflect.Map:
		visit(t, typ.Elem(), visited)
		if typ.Kind() == reflect.Map {
			visit(t, typ.Key(), visited)
		}
	}
}

// descend walks the struct field types (recurses one level deeper).
func descend(t *testing.T, typ reflect.Type, visited map[reflect.Type]bool) {
	for i := 0; i < typ.NumField(); i++ {
		visit(t, typ.Field(i).Type, visited)
	}
}

// hasYAMLTag reports whether the field carries a yaml tag (any
// non-empty value, including the special "-" form that disables
// decoding -- a field tagged yaml:"-" is intentionally excluded).
func hasYAMLTag(f reflect.StructField) bool {
	tag, ok := f.Tag.Lookup("yaml")
	if !ok {
		return false
	}
	return tag != ""
}

// implementsUnmarshalYAML returns true when either the type or a
// pointer to the type satisfies the yaml.Unmarshaler interface (yaml.v4
// dispatch checks both addressable and non-addressable receivers).
func implementsUnmarshalYAML(typ reflect.Type) bool {
	unmarshaler := reflect.TypeOf((*yaml.Unmarshaler)(nil)).Elem()
	if typ.Implements(unmarshaler) {
		return true
	}
	return reflect.PointerTo(typ).Implements(unmarshaler)
}
