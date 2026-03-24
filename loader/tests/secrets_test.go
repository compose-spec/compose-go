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

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestServiceSecrets(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    secrets:
      - source: secret1
        target: /run/secrets/secret1
      - source: secret2
        target: my_secret
        uid: '103'
        gid: '103'
        mode: 0440
secrets:
  secret1:
    file: ./secret_data
  secret2:
    external: true
`)
	secrets := p.Services["foo"].Secrets
	assert.Equal(t, len(secrets), 2)
	assert.Equal(t, secrets[0].Source, "secret1")
	assert.Equal(t, secrets[0].Target, "/run/secrets/secret1")
	assert.Equal(t, secrets[1].Source, "secret2")
	assert.Equal(t, secrets[1].Target, "my_secret")
	assert.Equal(t, secrets[1].UID, "103")
	assert.Equal(t, secrets[1].GID, "103")
	assert.Equal(t, *secrets[1].Mode, types.FileMode(0o440))
}

func TestTopLevelSecrets(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
secrets:
  secret1:
    file: ./secret_data
    labels:
      foo: bar
  secret2:
    external:
      name: my_secret
  secret3:
    external: true
  secret4:
    name: bar
    environment: BAR
    x-bar: baz
    x-foo: bar
  secret5:
    file: /abs/secret_data
`, map[string]string{"BAR": "this is a secret"})

	assert.DeepEqual(t, p.Secrets["secret1"].Labels, types.Labels{"foo": "bar"})
	assert.Equal(t, p.Secrets["secret2"].Name, "my_secret")
	assert.Equal(t, p.Secrets["secret2"].External, types.External(true))
	assert.Equal(t, p.Secrets["secret3"].External, types.External(true))
	assert.Equal(t, p.Secrets["secret4"].Name, "bar")
	assert.Equal(t, p.Secrets["secret4"].Environment, "BAR")
	assert.Equal(t, p.Secrets["secret4"].Content, "this is a secret")
	assert.DeepEqual(t, p.Secrets["secret4"].Extensions, types.Extensions{
		"x-bar": "baz",
		"x-foo": "bar",
	})
	assert.Equal(t, p.Secrets["secret5"].File, "/abs/secret_data")
}

func TestSecretEnvironmentResolution(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
configs:
  config:
    environment: GA
secrets:
  secret:
    environment: MEU
`, map[string]string{"GA": "BU", "MEU": "Shadoks"})

	assert.Equal(t, p.Configs["config"].Environment, "GA")
	assert.Equal(t, p.Configs["config"].Content, "BU")
	assert.Equal(t, p.Secrets["secret"].Environment, "MEU")
	assert.Equal(t, p.Secrets["secret"].Content, "Shadoks")
}

func TestSecretFileModeNumber(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    secrets:
      - source: server-certificate
        target: server.cert
        mode: 0o440
`)
	assert.Equal(t, len(p.Services["foo"].Secrets), 1)
	assert.Equal(t, *p.Services["foo"].Secrets[0].Mode, types.FileMode(0o440))
}

func TestSecretFileModeString(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    secrets:
      - source: server-certificate
        target: server.cert
        mode: "0440"
`)
	assert.Equal(t, len(p.Services["foo"].Secrets), 1)
	assert.Equal(t, *p.Services["foo"].Secrets[0].Mode, types.FileMode(0o440))
}
