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

package tests

// The tests in this file lock the `services` top-level element:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md
//
// Spec: "A Compose file must declare a `services` top-level element as a map
// whose keys are string representations of service names, and whose values are
// service definitions."

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

// The key of each entry is the name of the service.
func TestServicesNamedByTheirKey(t *testing.T) {
	p := load(t, `
name: test
services:
  web:
    image: nginx
  db:
    image: postgres
`)
	names := p.ServiceNames()
	slices.Sort(names)
	assert.DeepEqual(t, names, []string{"db", "web"})
	assert.Equal(t, p.Services["web"].Name, "web")
	assert.Equal(t, p.Services["db"].Name, "db")

	yamlP, jsonP := roundTrip(t, p)
	assert.Equal(t, yamlP.Services["web"].Name, "web")
	assert.Equal(t, jsonP.Services["db"].Name, "db")
}

// A service name is made of letters, digits, dots, underscores and hyphens:
// any other name is rejected, with the name in the error.
func TestServicesNames(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		for _, name := range []string{"web", "web-1", "web_1", "web.1", "Web", "1web"} {
			t.Run(name, func(t *testing.T) {
				p := load(t, "name: test\nservices:\n  "+name+":\n    image: nginx\n")
				assert.Equal(t, p.Services[name].Name, name)
			})
		}
	})
	t.Run("rejected", func(t *testing.T) {
		for name, key := range map[string]string{
			"a space":  `"my web"`,
			"a colon":  `"web:1"`,
			"a slash":  `"web/1"`,
			"an empty": `""`,
		} {
			t.Run(name, func(t *testing.T) {
				err := loadErr(t, "name: test\nservices:\n  "+key+":\n    image: nginx\n")
				assert.ErrorContains(t, err, "services")
			})
		}
	})
}

// `services` is a map, and so is each service definition: anything else is
// rejected, with the path in the error.
func TestServicesMustBeMappings(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, message string }{
		"services as a list":    {"services: [web]\n", "services must be a mapping"},
		"services as a string":  {"services: web\n", "services must be a mapping"},
		"a service as a string": {"services:\n  web: nginx\n", "services.web must be a mapping"},
		"a service as a list":   {"services:\n  web: [nginx]\n", "services.web must be a mapping"},
		"a null service":        {"services:\n  web:\n", "services.web must be a mapping"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, "name: test\n"+tc.yaml)
			assert.ErrorContains(t, err, tc.message)
		})
	}
}
