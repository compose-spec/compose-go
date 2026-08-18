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

package transform

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
)

func Test_transformSchedule(t *testing.T) {
	path := tree.NewPath("jobs.test.triggers.schedule.0")

	// short syntax: a plain crontab expression becomes a schedule object
	out, err := transformSchedule("0 3 * * *", path, false)
	assert.NilError(t, err)
	assert.DeepEqual(t, out, map[string]any{"cron": "0 3 * * *"})

	// long syntax is preserved as-is
	long := map[string]any{"cron": "0 3 * * *", "timezone": "Europe/Paris"}
	out, err = transformSchedule(long, path, false)
	assert.NilError(t, err)
	assert.DeepEqual(t, out, long)

	// invalid entry types are rejected
	_, err = transformSchedule(42, path, false)
	assert.ErrorContains(t, err, "invalid type int for schedule entry")
}
