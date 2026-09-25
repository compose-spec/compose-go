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

package tree

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestMatcherMatches(t *testing.T) {
	m := NewMatcher(
		"services.*.image",
		"services.*.ports.[].target",
		"services.*.environment.*",
	)

	assert.Check(t, m.Matches("services.web.image"))
	assert.Check(t, m.Matches("services.web.ports.[].target"))
	assert.Check(t, m.Matches("services.web.environment.DEBUG"))

	assert.Check(t, !(m.Matches("services.web.command")), "undeclared sibling")
	assert.Check(t, !(m.Matches("services.web.image.tag")), "matching is exact, not subtree")
	assert.Check(t, !(m.Matches("services.web")), "ancestor of a pattern is not a match")
}

func TestMatcherMayContain(t *testing.T) {
	m := NewMatcher("services.*.deploy.update_config.delay")

	assert.Check(t, m.MayContain("services"))
	assert.Check(t, m.MayContain("services.web.deploy"))
	assert.Check(t, m.MayContain("services.web.deploy.update_config"))

	assert.Check(t, !(m.MayContain("services.web.deploy.update_config.delay")), "a full match is not a strict ancestor")
	assert.Check(t, !(m.MayContain("services.web.build")))
}
