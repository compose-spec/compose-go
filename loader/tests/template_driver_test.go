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

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestTemplateDriver(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
configs:
  config:
    name: config
    external: true
    template_driver: config-driver
secrets:
  secret:
    name: secret
    external: true
    template_driver: secret-driver
`)
	assert.Equal(t, p.Configs["config"].TemplateDriver, "config-driver")
	assert.Equal(t, p.Secrets["secret"].TemplateDriver, "secret-driver")

	yamlP, jsonP := roundTrip(t, p)
	assert.Equal(t, yamlP.Configs["config"].TemplateDriver, "config-driver")
	assert.Equal(t, yamlP.Secrets["secret"].TemplateDriver, "secret-driver")
	assert.Equal(t, jsonP.Configs["config"].TemplateDriver, "config-driver")
	assert.Equal(t, jsonP.Secrets["secret"].TemplateDriver, "secret-driver")
}

func TestSecretDriver(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
secrets:
  secret:
    name: secret
    driver: secret-bucket
    driver_opts:
      OptionA: value for driver option A
      OptionB: value for driver option B
`)
	assert.Equal(t, p.Secrets["secret"].Driver, "secret-bucket")
	assert.Equal(t, p.Secrets["secret"].DriverOpts["OptionA"], "value for driver option A")
	assert.Equal(t, p.Secrets["secret"].DriverOpts["OptionB"], "value for driver option B")

	yamlP, jsonP := roundTrip(t, p)
	assert.Equal(t, yamlP.Secrets["secret"].Driver, "secret-bucket")
	assert.Equal(t, jsonP.Secrets["secret"].Driver, "secret-bucket")
}

func TestConfigContent(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
configs:
  my_config:
    content: |
      some config content here
`)
	assert.Equal(t, p.Configs["my_config"].Content, "some config content here\n")

	yamlP, jsonP := roundTrip(t, p)
	assert.Equal(t, yamlP.Configs["my_config"].Content, "some config content here\n")
	assert.Equal(t, jsonP.Configs["my_config"].Content, "some config content here\n")
}

func TestConfigEnvironment(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
configs:
  my_config:
    environment: MY_VAR
`, map[string]string{"MY_VAR": "my_value"})

	assert.Equal(t, p.Configs["my_config"].Environment, "MY_VAR")
	assert.Equal(t, p.Configs["my_config"].Content, "my_value")
}
