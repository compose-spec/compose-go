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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
)

// BenchmarkLoadSmall measures the loader cost on a small project
// (one service). Establishes the per-invocation overhead floor:
// every Load goes through the full pipeline (parse, reset, alias
// normalize, merge, interpolate, canonical, paths, validate,
// normalize, decode) regardless of the project size.
func BenchmarkLoadSmall(b *testing.B) {
	dir := b.TempDir()
	writeFileBench(b, dir, "compose.yaml", `
services:
  web:
    image: nginx
    ports:
      - "80:80"
`)
	cd := types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
		Environment: map[string]string{},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := LoadWithContext(context.TODO(), cd, withProjectName("bench-small", true))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadMedium exercises a project with 50 services that share
// a YAML anchor for build defaults. Useful as a comparison point for
// the small benchmark to estimate scaling under realistic input.
func BenchmarkLoadMedium(b *testing.B) {
	dir := b.TempDir()
	var sb strings.Builder
	sb.WriteString(`x-build: &build
  context: .
  dockerfile: Dockerfile

services:
`)
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, `  svc%d:
    image: alpine:3.${TAG:-19}
    build: *build
    environment:
      INDEX: %d
    ports:
      - "%d:%d"
`, i, i, 8000+i, 80)
	}
	writeFileBench(b, dir, "compose.yaml", sb.String())
	cd := types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
		Environment: map[string]string{"TAG": "20"},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := LoadWithContext(context.TODO(), cd, withProjectName("bench-medium", true))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadWithDiagnostics measures the additional cost of the
// WithDiagnostics opt-in (the buildPathPositions snapshot must be
// retained across the whole pipeline and attached to Project.Sources).
func BenchmarkLoadWithDiagnostics(b *testing.B) {
	dir := b.TempDir()
	writeFileBench(b, dir, "compose.yaml", `
services:
  web:
    image: nginx
    ports:
      - "80:80"
`)
	cd := types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
		Environment: map[string]string{},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := LoadWithContext(context.TODO(), cd, func(o *Options) {
			o.SetProjectName("bench-diag", true)
			WithDiagnostics(o)
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func writeFileBench(b *testing.B, dir, name, content string) {
	b.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		b.Fatal(err)
	}
}
