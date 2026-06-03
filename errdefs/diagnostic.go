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

package errdefs

import (
	"fmt"
	"strings"
)

// Diagnostic wraps an error with the file location of the offending
// YAML node. It is the format the loader uses to surface
// interpolation, validation and merge failures with a "file:line:col:
// cause" prefix that points back at the source the user wrote.
//
// File is the absolute path of the source file, or "(inline)" when the
// document was built from in-memory bytes. Line and Column are 1-based;
// zero on either field is rendered as missing.
//
// Diagnostic intentionally implements Unwrap so errors.Is / errors.As
// on the wrapped Cause keep working.
type Diagnostic struct {
	File   string
	Line   int
	Column int
	// Path is the dotted compose path of the offending value
	// (e.g. "services.web.ports.0.published"). Optional; included in
	// the rendered message when set.
	Path  string
	Cause error
}

// Error renders the diagnostic. Examples:
//
//	/abs/compose.yaml:12:5: services.web.image: invalid value
//	/abs/compose.yaml:12: services.web.image: invalid value
//	services.web.image: invalid value
func (d *Diagnostic) Error() string {
	if d == nil || d.Cause == nil {
		return ""
	}
	var b strings.Builder
	if d.File != "" {
		b.WriteString(d.File)
		if d.Line > 0 {
			fmt.Fprintf(&b, ":%d", d.Line)
			if d.Column > 0 {
				fmt.Fprintf(&b, ":%d", d.Column)
			}
		}
		b.WriteString(": ")
	}
	if d.Path != "" {
		b.WriteString(d.Path)
		b.WriteString(": ")
	}
	b.WriteString(d.Cause.Error())
	return b.String()
}

// Unwrap exposes the underlying Cause so errors.Is / errors.As walk
// through to the inner error.
func (d *Diagnostic) Unwrap() error {
	if d == nil {
		return nil
	}
	return d.Cause
}

// Diagnose wraps cause as a Diagnostic. Returns nil when cause is nil
// so callers can pass through error returns without an extra check.
func Diagnose(cause error, file string, line, column int, path string) error {
	if cause == nil {
		return nil
	}
	return &Diagnostic{
		File:   file,
		Line:   line,
		Column: column,
		Path:   path,
		Cause:  cause,
	}
}
