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

package interpolation

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestInterpolateNode_Simple(t *testing.T) {
	input := `
services:
  web:
    image: ${IMAGE}
`
	lookup := func(key string) (string, bool) {
		if key == "IMAGE" {
			return "nginx", true
		}
		return "", false
	}

	var node yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(input), &node))
	err := InterpolateNode(&node, Options{LookupValue: lookup})
	assert.NilError(t, err)

	var result map[string]interface{}
	assert.NilError(t, node.Decode(&result))

	services := result["services"].(map[string]interface{})
	web := services["web"].(map[string]interface{})
	assert.Equal(t, "nginx", web["image"])
}

func TestInterpolateNode_Default(t *testing.T) {
	input := `
services:
  web:
    image: ${IMAGE:-default}
`
	lookup := func(_ string) (string, bool) {
		return "", false
	}

	var node yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(input), &node))
	err := InterpolateNode(&node, Options{LookupValue: lookup})
	assert.NilError(t, err)

	var result map[string]interface{}
	assert.NilError(t, node.Decode(&result))

	services := result["services"].(map[string]interface{})
	web := services["web"].(map[string]interface{})
	assert.Equal(t, "default", web["image"])
}

func TestInterpolateNode_NoSubstitution(t *testing.T) {
	input := `
services:
  web:
    image: nginx
    ports:
      - "8080"
`
	lookup := func(_ string) (string, bool) {
		return "", false
	}

	var node yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(input), &node))

	// Take a snapshot before interpolation
	var before map[string]interface{}
	assert.NilError(t, node.Decode(&before))

	err := InterpolateNode(&node, Options{LookupValue: lookup})
	assert.NilError(t, err)

	var after map[string]interface{}
	assert.NilError(t, node.Decode(&after))

	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	assert.Equal(t, string(beforeJSON), string(afterJSON))
}

func TestInterpolateNode_TypeCast(t *testing.T) {
	input := `
services:
  web:
    ports:
      - ${PORT}
`
	lookup := func(key string) (string, bool) {
		if key == "PORT" {
			return "8080", true
		}
		return "", false
	}

	toInt := func(value string) (interface{}, error) {
		return strconv.Atoi(value)
	}

	var node yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(input), &node))
	err := InterpolateNode(&node, Options{
		LookupValue: lookup,
		TypeCastMapping: map[tree.Path]Cast{
			tree.NewPath("services", tree.PathMatchAll, "ports", tree.PathMatchList): toInt,
		},
	})
	assert.NilError(t, err)

	var result map[string]interface{}
	assert.NilError(t, node.Decode(&result))

	services := result["services"].(map[string]interface{})
	web := services["web"].(map[string]interface{})
	ports := web["ports"].([]interface{})
	assert.Equal(t, 8080, ports[0])
}

func TestInterpolateNode_Parity(t *testing.T) {
	input := `
services:
  web:
    image: ${IMAGE}
    environment:
      FOO: ${FOO_VAL}
      BAR: ${BAR_VAL:-default_bar}
    labels:
      version: ${VERSION}
`
	env := map[string]string{
		"IMAGE":   "nginx",
		"FOO_VAL": "hello",
		"VERSION": "1.0",
	}
	testInterpolateParity(t, input, env)
}

func testInterpolateParity(t *testing.T, input string, env map[string]string) {
	t.Helper()
	lookup := func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
	opts := Options{
		LookupValue: lookup,
	}

	// Map-based
	var mapData map[string]interface{}
	assert.NilError(t, yaml.Unmarshal([]byte(input), &mapData))
	mapResult, err := Interpolate(mapData, opts)
	assert.NilError(t, err)

	// Node-based
	var node yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(input), &node))
	err = InterpolateNode(&node, opts)
	assert.NilError(t, err)

	var nodeMap map[string]interface{}
	assert.NilError(t, node.Decode(&nodeMap))

	// Compare via JSON
	mapJSON, _ := json.Marshal(mapResult)
	nodeJSON, _ := json.Marshal(nodeMap)
	assert.Equal(t, string(mapJSON), string(nodeJSON))
}

func TestInterpolateNode_Error(t *testing.T) {
	input := `
services:
  web:
    image: ${IMAGE:?}
`
	lookup := func(_ string) (string, bool) {
		return "", false
	}

	var node yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(input), &node))
	err := InterpolateNode(&node, Options{LookupValue: lookup})
	assert.Assert(t, err != nil, "expected an error for missing required variable")
}
