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
