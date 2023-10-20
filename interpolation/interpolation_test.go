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
	"fmt"
	"strconv"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

var defaults = map[string]string{
	"USER":  "jenny",
	"FOO":   "bar",
	"count": "5",
}

func defaultMapping(name string) (string, bool) {
	val, ok := defaults[name]
	return val, ok
}

func TestInterpolate(t *testing.T) {
	services := map[string]interface{}{
		"servicea": map[string]interface{}{
			"image":   "example:${USER}",
			"volumes": []interface{}{"$FOO:/target"},
			"logging": map[string]interface{}{
				"driver": "${FOO}",
				"options": map[string]interface{}{
					"user": "$USER",
				},
			},
		},
	}
	expected := map[string]interface{}{
		"servicea": map[string]interface{}{
			"image":   "example:jenny",
			"volumes": []interface{}{"bar:/target"},
			"logging": map[string]interface{}{
				"driver": "bar",
				"options": map[string]interface{}{
					"user": "jenny",
				},
			},
		},
	}
	result, err := Interpolate(services, Options{LookupValue: defaultMapping})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, result))
}

func TestInvalidInterpolation(t *testing.T) {
	services := map[string]interface{}{
		"servicea": map[string]interface{}{
			"image": "${",
		},
	}
	_, err := Interpolate(services, Options{LookupValue: defaultMapping})
	assert.Error(t, err, `invalid interpolation format for servicea.image.
You may need to escape any $ with another $.
${`)
}

func TestInterpolateWithDefaults(t *testing.T) {
	t.Setenv("FOO", "BARZ")

	config := map[string]interface{}{
		"networks": map[string]interface{}{
			"foo": "thing_${FOO}",
		},
	}
	expected := map[string]interface{}{
		"networks": map[string]interface{}{
			"foo": "thing_BARZ",
		},
	}
	result, err := Interpolate(config, Options{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, result))
}

func TestValidUnexistentInterpolation(t *testing.T) {
	var testcases = []struct {
		test     string
		expected string
		errMsg   string
	}{
		{test: "{{{ ${FOO:-foo_} }}}", expected: "{{{ foo_ }}}"},
		{test: "{{{ ${FOO:-foo-bar-value} }}}", expected: "{{{ foo-bar-value }}}"},
		{test: "{{{ ${FOO:-foo?bar?value} }}}", expected: "{{{ foo?bar?value }}}"},
		{test: "{{{ ${FOO:-foo:?bar:?value} }}}", expected: "{{{ foo:?bar:?value }}}"},
		{test: "{{{ ${FOO:-foo} ${BAR:-DEFAULT_VALUE} }}}", expected: "{{{ foo DEFAULT_VALUE }}}"},
		{test: "{{{ ${BAR} }}}", expected: "{{{  }}}"},
		{test: "${FOO:-baz} }}}", expected: "baz }}}"},
		{test: "${FOO-baz} }}}", expected: "baz }}}"},

		{test: "{{{ ${FOO:?foo_} }}}", errMsg: "foo_"},
		{test: "{{{ ${FOO:?foo-bar-value} }}}", errMsg: "foo-bar-value"},
		{test: "{{{ ${FOO:?foo} ${BAR:-DEFAULT_VALUE} }}}", errMsg: "foo"},
		{test: "${FOO:?foo} ${BAR:?bar}", errMsg: "foo"},
		{test: "{{{ ${BAR} }}}", expected: "{{{  }}}"},
		{test: "${FOO:?baz} }}}", errMsg: "baz"},
		{test: "${FOO?baz} }}}", errMsg: "baz"},
		// nested variables
		{test: "${FOO:-${BAR:-${ZOT:-qix}}}", expected: "qix"},
		{test: "${FOO:-${BAR:-x}_test_${BAR:-y}}", expected: "x_test_y"},
		{test: "${FOO:-${BAR:-x}_test}", expected: "x_test"},
		{test: "${FOO:-${BAR:-${ZOT:-x}}_test}", expected: "x_test"},
	}

	getServiceConfig := func(val string) map[string]interface{} {
		if val == "" {
			return map[string]interface{}{}
		}
		return map[string]interface{}{
			"myservice": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": val,
				},
			},
		}
	}

	getFullErrorMsg := func(msg string) string {
		return fmt.Sprintf("error while interpolating myservice.environment.TESTVAR: required variable FOO is missing a value: %s", msg)
	}

	for _, testcase := range testcases {
		result, err := Interpolate(getServiceConfig(testcase.test), Options{})
		if testcase.errMsg != "" {
			assert.Assert(t, err != nil, fmt.Sprintf("This should result in an error %q", testcase.errMsg))
			assert.Equal(t, getFullErrorMsg(testcase.errMsg), err.Error())
		}
		assert.Check(t, is.DeepEqual(getServiceConfig(testcase.expected), result))
	}
}

func TestValidExistentInterpolation(t *testing.T) {
	var testcases = []struct {
		test     string
		expected string
	}{
		// Only FOO is set
		{test: "{{{ ${FOO:-foo_} }}}", expected: "{{{ bar }}}"},
		{test: "{{{ ${FOO:-foo-bar-value} }}}", expected: "{{{ bar }}}"},
		{test: "{{{ ${FOO:-foo} ${BAR:-DEFAULT_VALUE} }}}", expected: "{{{ bar DEFAULT_VALUE }}}"},
		{test: "{{{ ${BAR} }}}", expected: "{{{  }}}"},
		{test: "${FOO:-baz} }}}", expected: "bar }}}"},
		{test: "${FOO-baz} }}}", expected: "bar }}}"},

		// Both FOO and USER are set
		{test: "{{{ ${FOO:-foo_} }}}", expected: "{{{ bar }}}"},
		{test: "{{{ ${FOO:-foo-bar-value} }}}", expected: "{{{ bar }}}"},
		{test: "{{{ ${FOO:-foo} ${USER:-bar} }}}", expected: "{{{ bar jenny }}}"},
		{test: "{{{ ${USER} }}}", expected: "{{{ jenny }}}"},
		{test: "${FOO:-baz} }}}", expected: "bar }}}"},
		{test: "${FOO-baz} }}}", expected: "bar }}}"},
	}

	getServiceConfig := func(val string) map[string]interface{} {
		return map[string]interface{}{
			"myservice": map[string]interface{}{
				"environment": map[string]interface{}{
					"TESTVAR": val,
				},
			},
		}
	}

	for _, testcase := range testcases {
		result, err := Interpolate(getServiceConfig(testcase.test), Options{LookupValue: defaultMapping})
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(getServiceConfig(testcase.expected), result))
	}
}

func TestInterpolateWithCast(t *testing.T) {
	config := map[string]interface{}{
		"foo": map[string]interface{}{
			"replicas": "$count",
		},
	}
	toInt := func(value string) (interface{}, error) {
		return strconv.Atoi(value)
	}
	result, err := Interpolate(config, Options{
		LookupValue:     defaultMapping,
		TypeCastMapping: map[tree.Path]Cast{tree.NewPath(tree.PathMatchAll, "replicas"): toInt},
	})
	assert.NilError(t, err)
	expected := map[string]interface{}{
		"foo": map[string]interface{}{
			"replicas": 5,
		},
	}
	assert.Check(t, is.DeepEqual(expected, result))
}

func TestPathMatches(t *testing.T) {
	var testcases = []struct {
		doc      string
		path     tree.Path
		pattern  tree.Path
		expected bool
	}{
		{
			doc:     "pattern too short",
			path:    tree.NewPath("one", "two", "three"),
			pattern: tree.NewPath("one", "two"),
		},
		{
			doc:     "pattern too long",
			path:    tree.NewPath("one", "two"),
			pattern: tree.NewPath("one", "two", "three"),
		},
		{
			doc:     "pattern mismatch",
			path:    tree.NewPath("one", "three", "two"),
			pattern: tree.NewPath("one", "two", "three"),
		},
		{
			doc:     "pattern mismatch with match-all part",
			path:    tree.NewPath("one", "three", "two"),
			pattern: tree.NewPath(tree.PathMatchAll, "two", "three"),
		},
		{
			doc:      "pattern match with match-all part",
			path:     tree.NewPath("one", "two", "three"),
			pattern:  tree.NewPath("one", "*", "three"),
			expected: true,
		},
		{
			doc:      "pattern match",
			path:     tree.NewPath("one", "two", "three"),
			pattern:  tree.NewPath("one", "two", "three"),
			expected: true,
		},
	}
	for _, testcase := range testcases {
		assert.Check(t, is.Equal(testcase.expected, testcase.path.Matches(testcase.pattern)))
	}
}
