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

package override

import (
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
)

func Test_EnvironmentUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    environment:
      - FOO=BAR
      - BAR=QIX
      - QIX=
      - ZOT
      - FOO=ZOT
      - QIX
      - ZOT=
`, `
services:
  test:
    image: foo
    environment:
      - FOO=ZOT
      - BAR=QIX
      - QIX
      - ZOT=
`)
}

func Test_VolumeUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    volumes:
      - .:/foo
      - foo:/bar
      - src:/foo
`, `
services:
  test:
    image: foo
    volumes:
      - src:/foo
      - foo:/bar
`)
}

// Devices are told apart by their target in the container, whatever the
// syntax and permissions: the last mapping of a target takes effect at the
// position of the first, while the same host device mapped to two targets is
// two devices.
func Test_DevicesUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    devices:
      - /dev/sda:/dev/xvda:r
      - /dev/sdb:/dev/xvdb
      - /dev/sda:/dev/xvdc
      - source: /dev/sdd
        target: /dev/xvda
        permissions: rwm
      - vendor1.com/device=gpu
      - vendor1.com/device=gpu
`, `
services:
  test:
    image: foo
    devices:
      - source: /dev/sdd
        target: /dev/xvda
        permissions: rwm
      - /dev/sdb:/dev/xvdb
      - /dev/sda:/dev/xvdc
      - vendor1.com/device=gpu
`)
}

func Test_PortsShortUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    ports:
      - "9080:80"
      - "9081:81"
      - "9080:80"
      - "5000"
      - "6060:6060/udp"
      - "9080:6060/udp"
`, `
services:
  test:
    image: foo
    ports:
      - "9080:80"
      - "9081:81"
      - "5000"
      - "6060:6060/udp"
      - "9080:6060/udp"
`)
}

func Test_PortsLongtUnicity(t *testing.T) {
	assertUnicity(t, `
services:
  test:
    image: foo
    ports:
      - target: 80
        host_ip: 127.0.0.1
        published: "8080"
        protocol: tcp
        mode: host
      - target: 81
        published: "8080"
        protocol: tcp
      - target: 80
        host_ip: 127.0.0.2
        published: "8080"
        protocol: tcp
      - target: 81
        published: "8080"
        protocol: tcp
`, `
services:
  test:
    image: foo
    ports:
      - target: 80
        host_ip: 127.0.0.1
        published: "8080"
        protocol: tcp
        mode: host
      - target: 81
        published: "8080"
        protocol: tcp
      - target: 80
        host_ip: 127.0.0.2
        published: "8080"
        protocol: tcp
`)
}

func assertUnicity(t *testing.T, before string, expected string) {
	got, err := EnforceUnicity(unmarshal(t, before))
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, expected))
}

// Entries of a list are told apart by an identifying key (the target of a
// secret or config mount, the path of an env_file entry): a key that is not a
// string is a document error naming the attribute, not a crash of the loader,
// which enforces unicity before the schema validation.
func TestEnforceUnicityRejectsKeyThatIsNotAString(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr string
	}{
		{
			name:    "secret target",
			model:   "services:\n  a:\n    secrets:\n      - source: s\n        target: 3\n",
			wantErr: "services.a.secrets.[0]: unexpected type int for target",
		},
		{
			name:    "config target",
			model:   "services:\n  a:\n    configs:\n      - source: c\n        target: [x]\n",
			wantErr: "services.a.configs.[0]: unexpected type []interface {} for target",
		},
		{
			name:    "env_file path",
			model:   "services:\n  a:\n    env_file:\n      - path: 3\n",
			wantErr: "services.a.env_file.[0]: unexpected type int for path",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EnforceUnicity(unmarshal(t, tt.model))
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// Jobs share the container specification of services, so a sequence that
// cannot hold two entries for the same thing (a mount target, a published
// port, a key...) is compacted the same way whichever of the two declares it:
// the last entry wins, at the position of the first one. Without it, merging
// two files over a job leaves both entries in the model while the same
// documents under `services:` leave one.
func TestEnforceUnicityAppliesToJobsLikeServices(t *testing.T) {
	tests := []struct {
		name     string
		attr     string
		base     string
		override string
		want     string
	}{
		{
			name: "ports",
			attr: "ports",
			base: `
      - target: 80
        published: "8080"
      - target: 81
        published: "8081"
      - target: 80
        published: "8080"
        mode: host`,
			want: `
      - target: 80
        published: "8080"
        mode: host
      - target: 81
        published: "8081"`,
		},
		{name: "volumes", attr: "volumes", base: `[./a:/data, ./b:/data]`, want: `[./b:/data]`},
		{name: "devices", attr: "devices", base: `[/dev/a:/dev/x, /dev/b:/dev/x]`, want: `[/dev/b:/dev/x]`},
		{
			name: "secrets",
			attr: "secrets",
			base: `
      - s
      - source: s
        mode: 0400`,
			want: `
      - source: s
        mode: 0400`,
		},
		{
			name: "configs",
			attr: "configs",
			base: `
      - c
      - source: c
        mode: 0400`,
			want: `
      - source: c
        mode: 0400`,
		},
		{name: "env_file", attr: "env_file", base: `[a.env, b.env, a.env]`, want: `[a.env, b.env]`},
		{name: "expose", attr: "expose", base: `["80", 80]`, want: `[80]`},
		{name: "cap_add", attr: "cap_add", base: `[NET_ADMIN, NET_ADMIN]`, want: `[NET_ADMIN]`},
		{name: "cap_drop", attr: "cap_drop", base: `[ALL, ALL]`, want: `[ALL]`},
		{name: "dns", attr: "dns", base: `[1.1.1.1, 1.1.1.1]`, want: `[1.1.1.1]`},
		{name: "dns_opt", attr: "dns_opt", base: `[ndots=1, ndots=2]`, want: `[ndots=2]`},
		{name: "dns_search", attr: "dns_search", base: `[a.com, a.com]`, want: `[a.com]`},
		{name: "profiles", attr: "profiles", base: `[p, p]`, want: `[p]`},
		{name: "tmpfs", attr: "tmpfs", base: `[/a, /a]`, want: `[/a]`},
		{name: "sysctls", attr: "sysctls", base: `[a=1, a=2]`, want: `[a=2]`},
		{name: "labels", attr: "labels", base: `[a=1, a=2]`, want: `[a=2]`},
		{name: "annotations", attr: "annotations", base: `[a=1, a=2]`, want: `[a=2]`},
		{name: "environment", attr: "environment", base: `[A=1, A=2]`, want: `[A=2]`},
		{name: "build.tags", attr: "build:\n      tags", base: `[t, t]`, want: `[t]`},
		{name: "build.args", attr: "build:\n      args", base: `[a=1, a=2]`, want: `[a=2]`},
		{name: "build.labels", attr: "build:\n      labels", base: `[a=1, a=2]`, want: `[a=2]`},
		{name: "build.additional_contexts", attr: "build:\n      additional_contexts", base: `[a=x, a=y]`, want: `[a=y]`},
		{name: "networks.*.aliases", attr: "networks:\n      n:\n        aliases", base: `[a, a]`, want: `[a]`},
		{name: "networks.*.link_local_ips", attr: "networks:\n      n:\n        link_local_ips", base: `[1.1.1.1, 1.1.1.1]`, want: `[1.1.1.1]`},
	}
	for _, tt := range tests {
		for _, section := range []string{"services", "jobs"} {
			t.Run(tt.name+" under "+section, func(t *testing.T) {
				doc := func(value string) string {
					return section + ":\n  test:\n    " + tt.attr + ": " + value + "\n"
				}
				assertUnicity(t, doc(tt.base), doc(tt.want))
			})
		}
	}
}

// Every rule of services that applies to the container specification has its
// counterpart for jobs: a rule added for `services.*` alone silently leaves
// jobs out, as the merge rules (which cover both) never do. The attributes
// only services have are listed here.
func TestEnforceUnicityCoversJobsAsServices(t *testing.T) {
	serviceOnly := map[tree.Path]bool{
		"services.*.deploy.labels": true, // jobs have no deploy
		"services.*.links":         true, // jobs have no links
	}
	for pattern := range unique {
		rest, ok := strings.CutPrefix(string(pattern), "services.")
		if !ok || serviceOnly[pattern] {
			continue
		}
		t.Run(string(pattern), func(t *testing.T) {
			_, ok := unique[tree.Path("jobs."+rest)]
			assert.Assert(t, ok, "%s has no counterpart for jobs", pattern)
		})
	}
}
