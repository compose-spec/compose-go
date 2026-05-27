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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// loadResetYAML is a test helper that loads one or more inline YAML config files
// with normalization and consistency checks disabled.
func loadResetYAML(ctx context.Context, configs ...string) (*types.Project, error) {
	files := make([]types.ConfigFile, len(configs))
	for i, c := range configs {
		name := "(inline)"
		if i > 0 {
			name = fmt.Sprintf("(override-%d)", i)
		}
		files[i] = types.ConfigFile{Filename: name, Content: []byte(c)}
	}
	return LoadWithContext(ctx, types.ConfigDetails{ConfigFiles: files}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
}

func TestResetRemove(t *testing.T) {
	p, err := loadResetYAML(context.TODO(), `
name: test-reset
networks:
  test:
    name: test
    external: true
`, `
networks:
  test: !reset {}
`)
	assert.NilError(t, err)
	_, ok := p.Networks["test"]
	assert.Check(t, !ok)
}

func TestOverrideReplace(t *testing.T) {
	p, err := loadResetYAML(context.TODO(), `
name: test-override
networks:
  test:
    name: test
    external: true
`, `
networks:
  test: !override {}
`)
	assert.NilError(t, err)
	assert.Check(t, p.Networks["test"].External == false)
}

func TestResetCycle(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "simple_alias_no_cycle",
			config: `
name: test
services:
  a: &a
    image: alpine
  a2: *a
`,
			expectError: false,
		},
		{
			name: "simple_alias_reversed_no_cycle",
			config: `
name: test
services:
  a2: &a
    image: alpine
  a: *a
`,
			expectError: false,
		},
		{
			name: "nested_merge_no_cycle",
			config: `
name: test
x-templates:
  x-gluetun: &gluetun
    environment: &gluetun_env
      a: b
  x-gluetun-pia: &gluetun_pia
    <<: *gluetun
  x-gluetun-env-pia: &gluetun_env_pia
    <<: *gluetun_env
  vp0:
    <<: *gluetun_pia
    environment:
      <<: *gluetun_env_pia
`,
			expectError: false,
		},
		{
			name: "multiple_services_common_config",
			config: `
name: test
x-common:
  &common
  restart: unless-stopped

services:
  backend:
    <<: *common
    image: alpine:latest

  backend-static:
    <<: *common
    image: alpine:latest

  backend-worker:
    <<: *common
    image: alpine:latest
`,
			expectError: false,
		},
		{
			name: "direct_self_reference_cycle",
			config: `
name: test
x-healthcheck: &healthcheck
  egress-service:
    <<: *healthcheck
`,
			expectError: true,
			errorMsg:    "cycle detected: node at path x-healthcheck.egress-service.egress-service references node at path x-healthcheck.egress-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadResetYAML(context.TODO(), tt.config)
			if tt.expectError {
				assert.ErrorContains(t, err, tt.errorMsg)
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

// TestAliasBombPrevented verifies that deeply nested alias structures complete in bounded time.
// The 5-second timeout is a test-process safety net only.
func TestAliasBombPrevented(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		errorRequired bool // when false, no error is also acceptable (both outcomes are fast)
	}{
		{
			name: "B3_N10_small_branching",
			config: `
name: test
x-a: &a {k: v}
x-b: &b [*a, *a, *a]
x-c: &c [*b, *b, *b]
x-d: &d [*c, *c, *c]
x-e: &e [*d, *d, *d]
x-f: &f [*e, *e, *e]
x-g: &g [*f, *f, *f]
x-h: &h [*g, *g, *g]
x-i: &i [*h, *h, *h]
x-j: &j [*i, *i, *i]
x-k: &k [*j, *j, *j]
services:
  svc:
    image: alpine
`,
			errorRequired: false,
		},
		{
			name: "B9_N9_original_poc",
			config: `
name: test
x-a: &a {k: v}
x-b: &b [*a, *a, *a, *a, *a, *a, *a, *a, *a]
x-c: &c [*b, *b, *b, *b, *b, *b, *b, *b, *b]
x-d: &d [*c, *c, *c, *c, *c, *c, *c, *c, *c]
x-e: &e [*d, *d, *d, *d, *d, *d, *d, *d, *d]
x-f: &f [*e, *e, *e, *e, *e, *e, *e, *e, *e]
x-g: &g [*f, *f, *f, *f, *f, *f, *f, *f, *f]
x-h: &h [*g, *g, *g, *g, *g, *g, *g, *g, *g]
x-i: &i [*h, *h, *h, *h, *h, *h, *h, *h, *h]
x-j: &j [*i, *i, *i, *i, *i, *i, *i, *i, *i]
services:
  svc:
    image: alpine
`,
			errorRequired: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := loadResetYAML(ctx, tt.config)
			if err != nil || tt.errorRequired {
				assert.ErrorContains(t, err, "excessive aliasing")
			}
		})
	}
}

// TestVisitCounterLimit verifies that a document with more than the default node visit cap
// is rejected with a clear error, providing a safety belt independent of alias memoization.
func TestVisitCounterLimit(t *testing.T) {
	// Two mappings of (defaultMaxNodeVisits/2 + 1) entries each → total > cap value visits.
	half := defaultMaxNodeVisits/2 + 1
	var sb strings.Builder
	sb.WriteString("name: test\nx-data1:\n")
	for i := 0; i < half; i++ {
		fmt.Fprintf(&sb, "  k%d: v\n", i)
	}
	sb.WriteString("x-data2:\n")
	for i := 0; i < half; i++ {
		fmt.Fprintf(&sb, "  k%d: v\n", i)
	}
	_, err := loadResetYAML(context.TODO(), sb.String())
	assert.ErrorContains(t, err, "exceeds maximum node visit limit")
}

// TestVisitCounterLimitOverride verifies that Options.MaxNodeVisits raises the cap, allowing
// documents that would be rejected at the default limit to load successfully.
func TestVisitCounterLimitOverride(t *testing.T) {
	half := defaultMaxNodeVisits/2 + 1
	var sb strings.Builder
	sb.WriteString("name: test\nx-data1:\n")
	for i := 0; i < half; i++ {
		fmt.Fprintf(&sb, "  k%d: v\n", i)
	}
	sb.WriteString("x-data2:\n")
	for i := 0; i < half; i++ {
		fmt.Fprintf(&sb, "  k%d: v\n", i)
	}
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "(inline)", Content: []byte(sb.String())}},
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
		options.MaxNodeVisits = defaultMaxNodeVisits * 2
	})
	assert.NilError(t, err)
}

// TestLargeLegitimateFile verifies that a large but realistic compose file (400 services x 122
// environment variables, ~50,000 node visits) is accepted without hitting the visit counter.
func TestLargeLegitimateFile(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("name: test\nservices:\n")
	for s := 0; s < 400; s++ {
		fmt.Fprintf(&sb, "  svc%d:\n    image: alpine\n    environment:\n", s)
		for e := 0; e < 122; e++ {
			fmt.Fprintf(&sb, "      KEY_%d: val\n", e)
		}
	}
	_, err := loadResetYAML(context.TODO(), sb.String())
	assert.NilError(t, err)
}

// TestResetTagWithSharedAlias verifies that a !reset tag inside an anchor is correctly applied
// at every call site when the anchor is referenced by multiple services.
func TestResetTagWithSharedAlias(t *testing.T) {
	p, err := loadResetYAML(context.TODO(), `
name: test
services:
  svc1:
    image: alpine
    ports:
      - "8080:80"
  svc2:
    image: nginx
    ports:
      - "9090:90"
`, `
x-reset-ports: &reset-ports
  ports: !reset []

services:
  svc1:
    <<: *reset-ports
  svc2:
    <<: *reset-ports
`)
	assert.NilError(t, err)
	assert.Check(t, len(p.Services["svc1"].Ports) == 0, "svc1 ports should be reset")
	assert.Check(t, len(p.Services["svc2"].Ports) == 0, "svc2 ports should be reset")
}

// TestOverrideTagWithSharedAlias verifies that a !override tag inside a shared anchor is
// correctly applied at every call site (exercises the non-nil cached node path in cachedResolve).
func TestOverrideTagWithSharedAlias(t *testing.T) {
	p, err := loadResetYAML(context.TODO(), `
name: test
networks:
  net1:
    external: true
    name: external-net-1
  net2:
    external: true
    name: external-net-2
`, `
x-override: &ov !override {}

networks:
  net1: *ov
  net2: *ov
`)
	assert.NilError(t, err)
	assert.Check(t, p.Networks["net1"].External == false, "net1 should be overridden to non-external")
	assert.Check(t, p.Networks["net2"].External == false, "net2 should be overridden to non-external")
}

// TestNestedAliasReset verifies that !reset tags propagate correctly through multiple levels
// of alias indirection, testing transitive subPath computation in the cache relay logic.
func TestNestedAliasReset(t *testing.T) {
	p, err := loadResetYAML(context.TODO(), `
name: test
services:
  svc:
    image: alpine
    ports:
      - "8080:80"
`, `
x-inner: &inner
  ports: !reset []

x-outer: &outer
  <<: *inner

services:
  svc:
    <<: *outer
`)
	assert.NilError(t, err)
	assert.Check(t, len(p.Services["svc"].Ports) == 0, "ports should be reset through two alias levels")
}

// TestAliasNodePreservation is a regression-prevention pinning test (B=9, N=8 alias levels).
// The test must return an aliasing error. A nil error indicates a regression.
func TestAliasNodePreservation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := loadResetYAML(ctx, `
name: test
x-a: &a {k: v}
x-b: &b [*a, *a, *a, *a, *a, *a, *a, *a, *a]
x-c: &c [*b, *b, *b, *b, *b, *b, *b, *b, *b]
x-d: &d [*c, *c, *c, *c, *c, *c, *c, *c, *c]
x-e: &e [*d, *d, *d, *d, *d, *d, *d, *d, *d]
x-f: &f [*e, *e, *e, *e, *e, *e, *e, *e, *e]
x-g: &g [*f, *f, *f, *f, *f, *f, *f, *f, *f]
x-h: &h [*g, *g, *g, *g, *g, *g, *g, *g, *g]
x-i: &i [*h, *h, *h, *h, *h, *h, *h, *h, *h]
services:
  svc:
    image: alpine
`)
	assert.ErrorContains(t, err, "excessive aliasing")
}

// TestDirectAliasWithReset verifies that a !reset anchor used via direct alias assignment
// (not << merge key) is correctly applied at every call site.
func TestDirectAliasWithReset(t *testing.T) {
	p, err := loadResetYAML(context.TODO(), `
name: test
services:
  svc1:
    image: alpine
    ports:
      - "8080:80"
  svc2:
    image: nginx
    ports:
      - "9090:90"
`, `
x-reset: &reset !reset []

services:
  svc1:
    ports: *reset
  svc2:
    ports: *reset
`)
	assert.NilError(t, err)
	assert.Check(t, len(p.Services["svc1"].Ports) == 0, "svc1 ports should be reset")
	assert.Check(t, len(p.Services["svc2"].Ports) == 0, "svc2 ports should be reset")
}

// TestMergeKeyAliasTargets verifies that a `<<:` merge key accepts alias values
// regardless of whether the anchored target is a mapping or a sequence of mappings.
// Regression for docker/compose#13812: alias-to-sequence used to fail with
// "map merge requires map or sequence of maps as the value" because the YAML
// library only accepts AliasNode→MappingNode at merge sites.
func TestMergeKeyAliasTargets(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name: "alias_to_mapping",
			config: `
name: test
x-base: &base
  image: nginx
  restart: unless-stopped
services:
  s1:
    <<: *base
`,
		},
		{
			name: "alias_to_sequence_of_mappings",
			config: `
name: test
x-list: &alist
  - image: nginx
services:
  s1:
    <<: *alist
`,
		},
		{
			name: "alias_to_sequence_shared_across_services",
			config: `
name: test
x-list: &alist
  - image: nginx
    restart: unless-stopped
services:
  s1:
    <<: *alist
  s2:
    <<: *alist
`,
		},
		{
			name: "inline_sequence_of_mappings",
			config: `
name: test
x-base: &base
  image: nginx
services:
  s1:
    <<: [*base, {restart: unless-stopped}]
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadResetYAML(context.TODO(), tt.config)
			assert.NilError(t, err)
		})
	}
}

// TestMergeKeyAliasToScalarRejected verifies that a `<<:` merge key value that
// resolves to a scalar (neither mapping nor sequence of mappings) is still
// rejected. Guards against the fix making the parser too permissive.
func TestMergeKeyAliasToScalarRejected(t *testing.T) {
	_, err := loadResetYAML(context.TODO(), `
name: test
x-scalar: &s "not-a-map"
services:
  s1:
    <<: *s
`)
	assert.ErrorContains(t, err, "map merge requires map")
}
