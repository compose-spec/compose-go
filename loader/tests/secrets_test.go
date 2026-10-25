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

// The tests in this file lock the `secrets`, `secrets (top-level)` attributes:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#secrets
//   https://github.com/compose-spec/compose-spec/blob/main/09-secrets.md
//
// Spec: "`secrets` grants access to sensitive data defined by secrets on a
// per-service basis."

import (
	"math"
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
jobs:
  foo:
    triggers:
      manual: true
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

	jobSecrets := p.Jobs["foo"].Secrets
	assert.Equal(t, len(jobSecrets), 2)
	assert.Equal(t, jobSecrets[0].Source, "secret1")
	assert.Equal(t, jobSecrets[0].Target, "/run/secrets/secret1")
	assert.Equal(t, jobSecrets[1].Source, "secret2")
	assert.Equal(t, jobSecrets[1].Target, "my_secret")
	assert.Equal(t, jobSecrets[1].UID, "103")
	assert.Equal(t, jobSecrets[1].GID, "103")
	assert.Equal(t, *jobSecrets[1].Mode, types.FileMode(0o440))
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
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    secrets:
      - source: server-certificate
        target: server.cert
        mode: 0o440
`)
	assert.Equal(t, len(p.Services["foo"].Secrets), 1)
	assert.Equal(t, *p.Services["foo"].Secrets[0].Mode, types.FileMode(0o440))
	assert.Equal(t, len(p.Jobs["foo"].Secrets), 1)
	assert.Equal(t, *p.Jobs["foo"].Secrets[0].Mode, types.FileMode(0o440))
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
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    secrets:
      - source: server-certificate
        target: server.cert
        mode: "0440"
`)
	assert.Equal(t, len(p.Services["foo"].Secrets), 1)
	assert.Equal(t, *p.Services["foo"].Secrets[0].Mode, types.FileMode(0o440))
	assert.Equal(t, len(p.Jobs["foo"].Secrets), 1)
	assert.Equal(t, *p.Jobs["foo"].Secrets[0].Mode, types.FileMode(0o440))
}

// A file mode is an unsigned 32 bit number, as the Docker CLI reads the mode of
// a secret, whichever way it is written: the smallest and the largest values
// load, and marshal to a mode that loads back to the same value.
func TestSecretFileModeBounds(t *testing.T) {
	for name, tt := range map[string]struct {
		mode string
		want types.FileMode
	}{
		"zero":                     {`0`, 0},
		"zero, text":               {`"0"`, 0},
		"largest number":           {`0o37777777777`, math.MaxUint32},
		"largest decimal number":   {`4294967295`, math.MaxUint32},
		"largest octal text":       {`"37777777777"`, math.MaxUint32},
		"largest with a zero text": {`"037777777777"`, math.MaxUint32},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, `
name: test
services:
  foo:
    image: alpine
    secrets:
      - source: server-certificate
        mode: `+tt.mode+`
secrets:
  server-certificate:
    external: true
`)
			assert.Equal(t, *p.Services["foo"].Secrets[0].Mode, tt.want)
			fromYAML, fromJSON := roundTrip(t, p)
			assert.Equal(t, *fromYAML.Services["foo"].Secrets[0].Mode, tt.want)
			assert.Equal(t, *fromJSON.Services["foo"].Secrets[0].Mode, tt.want)
		})
	}
}

// The text of a file mode is read with the syntax of the Docker CLI
// (`--secret ...,mode=`, base 0) wherever it does not contradict the octal
// number Compose has always read: a leading 0 selects the prefixes 0o, 0x and
// 0b, and the digit separator they allow. The one syntax left out is the
// decimal number: `440` is 0o440 here, the CLI reads it as 440.
func TestSecretFileModeCLISyntax(t *testing.T) {
	for name, tt := range map[string]struct {
		mode string
		want types.FileMode
	}{
		"octal prefix":               {`"0o440"`, 0o440},
		"octal prefix, upper case":   {`"0O440"`, 0o440},
		"hexadecimal prefix":         {`"0x1ff"`, 0o777},
		"hexadecimal, upper case":    {`"0X1FF"`, 0o777},
		"binary prefix":              {`"0b101"`, 0b101},
		"binary prefix, upper case":  {`"0B101"`, 0b101},
		"separator after the prefix": {`"0o4_40"`, 0o440},
		"separator after the zero":   {`"0_440"`, 0o440},
		"largest hexadecimal":        {`"0xffffffff"`, math.MaxUint32},
		"no prefix is octal":         {`"440"`, 0o440},
		"leading zero is octal":      {`"0440"`, 0o440},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, `
name: test
services:
  foo:
    image: alpine
    secrets:
      - source: server-certificate
        mode: `+tt.mode+`
secrets:
  server-certificate:
    external: true
`)
			assert.Equal(t, *p.Services["foo"].Secrets[0].Mode, tt.want)
			fromYAML, fromJSON := roundTrip(t, p)
			assert.Equal(t, *fromYAML.Services["foo"].Secrets[0].Mode, tt.want)
			assert.Equal(t, *fromJSON.Services["foo"].Secrets[0].Mode, tt.want)
		})
	}
}

// A file mode is an unsigned 32 bit number, as the Docker CLI reads the mode of
// a secret: a negative value, a value that does not fit in 32 bits and text that
// is not octal are rejected, with the attribute in the error.
func TestSecretFileModeInvalid(t *testing.T) {
	for name, value := range map[string]string{
		"a negative number":          `-1`,
		"a negative numeric string":  `"-1"`,
		"a signed numeric string":    `"+440"`,
		"just beyond 32 bits":        `4294967296`,
		"just beyond 32 bits, octal": `0o40000000000`,
		"just beyond 32 bits, text":  `"40000000000"`,
		"far beyond 32 bits":         `0o77777777777`,
		"far beyond 32 bits, text":   `"77777777777"`,
		"beyond 64 bits, text":       `"7777777777777777777777"`,
		"not octal":                  `"0999"`,
		"text":                       `abc`,
		"a prefix without digits":    `"0x"`,
		"a digit out of the base":    `"0b2"`,
		"hexadecimal beyond 32 bits": `"0x100000000"`,
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, `
name: test
services:
  foo:
    image: alpine
    secrets:
      - source: server-certificate
        mode: `+value+`
secrets:
  server-certificate:
    external: true
`)
			assert.ErrorContains(t, err, "services[foo].secrets[0].mode")
			assert.ErrorContains(t, err, "invalid file mode")
		})
	}
}
