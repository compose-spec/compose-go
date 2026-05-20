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
	"errors"
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestNodeErrf_IncludesSourcePosition(t *testing.T) {
	ctx := &types.NodeContext{Source: "/path/compose.yaml"}
	node := &yaml.Node{Line: 12, Column: 5}
	err := nodeErrf(ctx, node, "service %q not found", "app")
	assert.Equal(t, err.Error(), "/path/compose.yaml:12:5: service \"app\" not found")
}

func TestNodeErrf_NoSource(t *testing.T) {
	err := nodeErrf(nil, nil, "something went wrong")
	assert.Equal(t, err.Error(), "something went wrong")
}

func TestWrapNodeErr_PreservesChain(t *testing.T) {
	root := errors.New("root cause")
	ctx := &types.NodeContext{Source: "/path/compose.yaml"}
	node := &yaml.Node{Line: 3, Column: 1}
	wrapped := wrapNodeErr(ctx, node, root)
	assert.Equal(t, wrapped.Error(), "/path/compose.yaml:3:1: root cause")
	assert.Assert(t, errors.Is(wrapped, root))
}

func TestWrapNodeErr_NilErr(t *testing.T) {
	assert.Assert(t, wrapNodeErr(nil, nil, nil) == nil)
}
