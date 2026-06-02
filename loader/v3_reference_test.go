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
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
	"gotest.tools/v3/assert"
)

// Reference tests for the v3 refactoring (see plan.md).
//
// These tests are written first and skipped until the corresponding phase of
// the refactoring closes the underlying gap. They are the discriminant gates
// of the refactoring.

// TestInclude_EnvFile_ProvidesContextToServiceEnvFile asserts that each
// env_file entry is interpolated with the environment of the file that
// declared it:
//
//   - extra.env is declared inside the included sub/compose.yaml; its content
//     `FOO=$BAR` resolves against include.env_file (BAR=bar), yielding FOO=bar.
//   - override.env is declared in the top-level compose.yaml as an override of
//     the included `app` service; its content `OVR=${BAR:-fallback}` is
//     interpolated in the top-level scope, where BAR is not defined, so the
//     default value is selected (OVR=fallback).
//
// Today this fails: WithServicesEnvironmentResolved cannot reach the include's
// env (limitation 3 in plan.md). Will turn green at the end of Phase 7.
func TestInclude_EnvFile_ProvidesContextToServiceEnvFile(t *testing.T) {
	workdir, err := filepath.Abs("testdata/include/env_file")
	assert.NilError(t, err)
	topPath := filepath.Join(workdir, "compose.yaml")

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  workdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: map[string]string{},
	}, withProjectName("test-include-envfile-context", true))
	assert.NilError(t, err)

	resolved, err := p.WithServicesEnvironmentResolved(false)
	assert.NilError(t, err)

	app := resolved.Services["app"]

	foo, ok := app.Environment["FOO"]
	assert.Check(t, ok, "FOO should be present in resolved environment")
	if ok && foo != nil {
		assert.Check(t, *foo == "bar", "FOO should be 'bar' (from include.env_file BAR), got %q", *foo)
	}

	ovr, ok := app.Environment["OVR"]
	assert.Check(t, ok, "OVR should be present in resolved environment")
	if ok && ovr != nil {
		assert.Check(t, *ovr == "fallback", "OVR should be 'fallback' (BAR is not visible in top-level scope), got %q", *ovr)
	}
}
