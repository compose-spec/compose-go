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

package transform

import (
	"testing"

	"github.com/compose-spec/compose-go/tree"
	"gotest.tools/v3/assert"
)

func TestSSHConfig(t *testing.T) {
	ssh, err := transformSSH([]any{
		"default",
		"foo=bar",
	}, tree.NewPath("test"))
	assert.NilError(t, err)
	assert.DeepEqual(t, ssh, map[string]any{
		"default": nil,
		"foo":     "bar",
	})
}
