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
	"testing"

	"gotest.tools/v3/assert"
)

func Test_WithServices(t *testing.T) {
	p := &Project{
		Services: Services{
			"service_1": ServiceConfig{
				Name: "service_1",
				DependsOn: map[string]ServiceDependency{
					"service_3": {
						Condition: ServiceConditionStarted,
						Required:  true,
					},
				},
			},
			"service_2": ServiceConfig{
				Name: "service_2",
			},
			"service_3": ServiceConfig{
				Name: "service_3",
				DependsOn: map[string]ServiceDependency{
					"service_2": {
						Condition: ServiceConditionStarted,
						Required:  true,
					},
				},
			},
		},
	}
	order := []string{}
	fn := func(name string, _ *ServiceConfig) error {
		order = append(order, name)
		return nil
	}

	err := p.ForEachService(nil, fn)
	assert.NilError(t, err)
	assert.DeepEqual(t, order, []string{"service_2", "service_3", "service_1"})
}

func Test_LookupEnv(t *testing.T) {
	tests := []struct {
		name            string
		environment     map[string]string
		caseInsensitive bool
		search          string
		expectedValue   string
		expectedOk      bool
	}{
		{
			name: "case sensitive/case match",
			environment: map[string]string{
				"Env1": "Value1",
				"Env2": "Value2",
			},
			caseInsensitive: false,
			search:          "Env1",
			expectedValue:   "Value1",
			expectedOk:      true,
		},
		{
			name: "case sensitive/case unmatch",
			environment: map[string]string{
				"Env1": "Value1",
				"Env2": "Value2",
			},
			caseInsensitive: false,
			search:          "ENV1",
			expectedValue:   "",
			expectedOk:      false,
		},
		{
			name:            "case sensitive/nil environment",
			environment:     nil,
			caseInsensitive: false,
			search:          "Env1",
			expectedValue:   "",
			expectedOk:      false,
		},
		{
			name: "case insensitive/case match",
			environment: map[string]string{
				"Env1": "Value1",
				"Env2": "Value2",
			},
			caseInsensitive: true,
			search:          "Env1",
			expectedValue:   "Value1",
			expectedOk:      true,
		},
		{
			name: "case insensitive/case unmatch",
			environment: map[string]string{
				"Env1": "Value1",
				"Env2": "Value2",
			},
			caseInsensitive: true,
			search:          "ENV1",
			expectedValue:   "Value1",
			expectedOk:      true,
		},
		{
			name: "case insensitive/unmatch",
			environment: map[string]string{
				"Env1": "Value1",
				"Env2": "Value2",
			},
			caseInsensitive: true,
			search:          "Env3",
			expectedValue:   "",
			expectedOk:      false,
		},
		{
			name:            "case insensitive/nil environment",
			environment:     nil,
			caseInsensitive: true,
			search:          "Env1",
			expectedValue:   "",
			expectedOk:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			origIsCaseInsensitiveEnvVars := isCaseInsensitiveEnvVars
			defer func() {
				isCaseInsensitiveEnvVars = origIsCaseInsensitiveEnvVars
			}()
			isCaseInsensitiveEnvVars = test.caseInsensitive
			cd := ConfigDetails{
				Environment: test.environment,
			}
			v, ok := cd.LookupEnv(test.search)
			assert.Equal(t, v, test.expectedValue)
			assert.Equal(t, ok, test.expectedOk)
		})
	}
}
