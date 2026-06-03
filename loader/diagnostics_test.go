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
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v3/errdefs"
	"github.com/compose-spec/compose-go/v3/types"
	"gotest.tools/v3/assert"
)

// TestDiagnostic_ProjectSourcesOptIn confirms that the WithDiagnostics
// option populates *Project.Sources with the source Location of every
// reachable compose path, so downstream tooling can resolve a path
// (e.g. "services.web.image") to its file:line:column.
func TestDiagnostic_ProjectSourcesOptIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
services:
  web:
    image: nginx
`)

	withDiag := func(opts *Options) {
		opts.SetProjectName("diag-sources", true)
		WithDiagnostics(opts)
	}
	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withDiag)

	assert.NilError(t, err)
	assert.Assert(t, p.Sources != nil, "Project.Sources should be populated")

	imgLoc, ok := p.Sources["services.web.image"]
	assert.Assert(t, ok, "expected services.web.image in Sources, have %v",
		p.Sources)
	assert.Equal(t, imgLoc.File, filepath.Join(dir, "compose.yaml"))
	assert.Assert(t, imgLoc.Line > 0, "Line should be > 0, got %d", imgLoc.Line)
	assert.Assert(t, imgLoc.Column > 0, "Column should be > 0, got %d", imgLoc.Column)
}

// TestDiagnostic_ProjectSourcesDefaultOff confirms that without
// WithDiagnostics, Project.Sources stays nil so the project shape is
// unchanged for callers that did not opt in.
func TestDiagnostic_ProjectSourcesDefaultOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
services:
  web:
    image: nginx
`)

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-off", true))

	assert.NilError(t, err)
	assert.Assert(t, p.Sources == nil,
		"Project.Sources should be nil without WithDiagnostics, got %v", p.Sources)
}

// TestDiagnostic_IncludeMustBeAList confirms that an `include:` value
// that isn't a sequence surfaces with the file / line / column of the
// offending node.
func TestDiagnostic_IncludeMustBeAList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
include:
  path: other.yaml
services:
  foo:
    image: alpine
`)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-include-list", true))

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.File, filepath.Join(dir, "compose.yaml"))
	assert.Equal(t, diag.Path, "include")
	assert.Assert(t, diag.Line > 0, "Line must be set, got %d", diag.Line)
	assert.Assert(t, strings.Contains(diag.Cause.Error(),
		"`include` must be a list"),
		"unexpected cause: %v", diag.Cause)
}

// TestDiagnostic_IncludeCycleHasFile confirms that a self-including
// compose file surfaces an "include cycle detected" diagnostic whose
// File points at the offending source.
func TestDiagnostic_IncludeCycleHasFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
include:
  - compose.yaml
services:
  foo:
    image: alpine
`)
	target := filepath.Join(dir, "compose.yaml")

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: target,
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-cycle", true))

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.File, target)
	assert.Assert(t, strings.Contains(diag.Cause.Error(),
		"include cycle detected"),
		"unexpected cause: %v", diag.Cause)
}

// TestDiagnostic_ExtendsServiceNotFound confirms that an extends.file
// pointing at a service the base file does not declare surfaces with
// the file / line / column of the extends node on the derived service.
func TestDiagnostic_ExtendsServiceNotFound(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `
services:
  other:
    image: alpine
`)
	writeFile(t, dir, "compose.yaml", `
services:
  derived:
    extends:
      file: base.yaml
      service: ghost
`)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-extends-missing", true))

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.File, filepath.Join(dir, "compose.yaml"))
	assert.Equal(t, diag.Path, "services.derived.extends")
	assert.Assert(t, diag.Line > 0, "Line must be set, got %d", diag.Line)
	assert.Assert(t, strings.Contains(diag.Cause.Error(),
		`service "ghost" not found`),
		"unexpected cause: %v", diag.Cause)
}

// TestDiagnostic_ExtendsMissingService confirms that an extends mapping
// without the required `service` key surfaces with the position of the
// extends node.
func TestDiagnostic_ExtendsMissingService(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "compose.yaml", `
services:
  derived:
    extends:
      file: base.yaml
`)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-extends-noservice", true))

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.File, filepath.Join(dir, "compose.yaml"))
	assert.Equal(t, diag.Path, "services.derived.extends")
	assert.Assert(t, diag.Line > 0, "Line must be set, got %d", diag.Line)
	assert.Assert(t, strings.Contains(diag.Cause.Error(),
		"extends.derived.service is required"),
		"unexpected cause: %v", diag.Cause)
}

// TestDiagnostic_ValidationKeepsPositionAcrossCanonical confirms that
// CanonicalNode's node-level walker preserves Line / Column on every
// node it does not actually reshape, so a post-canonical
// compose-rule validation failure still points at the line and column
// the user wrote (rather than zero, which the full-tree decode/encode
// bridge used to produce).
func TestDiagnostic_ValidationKeepsPositionAcrossCanonical(t *testing.T) {
	dir := t.TempDir()
	src := `
services:
  foo:
    image: alpine
configs:
  bad:
    file: /tmp/cfg
    environment: VAR
`
	writeFile(t, dir, "compose.yaml", src)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-canonical", true))

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.Path, "configs.bad")
	assert.Assert(t, diag.Line > 0,
		"Line must survive CanonicalNode walk, got %d", diag.Line)
	assert.Assert(t, diag.Column > 0,
		"Column must survive CanonicalNode walk, got %d", diag.Column)
}

// TestDiagnostic_InterpolationStrictModeIncludesFileLineColumn confirms
// that a strict-mode unset variable surfaces with the file, line and
// column of the offending scalar.
func TestDiagnostic_InterpolationStrictModeIncludesFileLineColumn(t *testing.T) {
	dir := t.TempDir()
	src := `
services:
  web:
    image: nginx:${MISSING:?must be set}
`
	writeFile(t, dir, "compose.yaml", src)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-interp", true))

	assert.Assert(t, err != nil, "expected interpolation error")

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.File, filepath.Join(dir, "compose.yaml"))
	assert.Assert(t, diag.Line > 0, "Line must be set, got %d", diag.Line)
	assert.Equal(t, diag.Path, "services.web.image")
	assert.Assert(t, strings.Contains(diag.Cause.Error(), "must be set"),
		"unexpected cause: %v", diag.Cause)
}

// TestDiagnostic_SchemaErrorIncludesFileLineColumn confirms that a
// JSON Schema failure surfaces the file, line and column the user
// wrote, via *errdefs.Diagnostic.
func TestDiagnostic_SchemaErrorIncludesFileLineColumn(t *testing.T) {
	dir := t.TempDir()
	src := `
services:
  bad:
    image: 42
`
	writeFile(t, dir, "compose.yaml", src)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-schema", true))

	assert.Assert(t, err != nil, "expected schema error")

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)
	assert.Equal(t, diag.File, filepath.Join(dir, "compose.yaml"))
	assert.Assert(t, diag.Line > 0, "Line must be set, got %d", diag.Line)
	assert.Assert(t, strings.HasPrefix(diag.Path, "services.bad"),
		"path should target the offending value, got %q", diag.Path)
}

// TestDiagnostic_ValidateNodeIncludesFileLineColumn confirms that a
// validation error surfaces the source file, line and column of the
// offending node alongside the failure reason, via *errdefs.Diagnostic.
func TestDiagnostic_ValidateNodeIncludesFileLineColumn(t *testing.T) {
	dir := t.TempDir()
	src := `
services:
  foo:
    image: alpine
secrets:
  bad:
    file: /tmp/secret
    environment: VAR
`
	writeFile(t, dir, "compose.yaml", src)

	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: dir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(dir, "compose.yaml"),
		}},
		Environment: map[string]string{},
	}, withProjectName("diag-test", true))

	assert.Assert(t, err != nil, "expected validation error")

	var diag *errdefs.Diagnostic
	assert.Assert(t, errors.As(err, &diag),
		"expected *errdefs.Diagnostic, got %T: %v", err, err)

	assert.Equal(t, diag.File, filepath.Join(dir, "compose.yaml"))
	assert.Assert(t, diag.Line > 0, "Line must be set, got %d", diag.Line)
	assert.Assert(t, diag.Column > 0, "Column must be set, got %d", diag.Column)
	assert.Equal(t, diag.Path, "secrets.bad")
	assert.Assert(t, strings.Contains(diag.Cause.Error(),
		"file|environment attributes are mutually exclusive"),
		"unexpected cause: %v", diag.Cause)
	// Rendered form: file:line:col: path: cause
	rendered := diag.Error()
	assert.Assert(t, strings.HasPrefix(rendered, diag.File+":"),
		"diagnostic should start with file: %q", rendered)
	assert.Assert(t, strings.Contains(rendered, "secrets.bad"),
		"diagnostic should include path: %q", rendered)
}
