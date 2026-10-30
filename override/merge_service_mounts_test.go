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
	"testing"
)

// mounts are merged by target, like volumes: a mount that an override declares
// for a target replaces the one of the base, and the others are kept
func TestMergeServiceMounts(t *testing.T) {
	assertMergeYaml(t, `
services:
  foo:
    image: alpine
    mounts:
      - type: bind
        source: /data
        target: /bind
        bind:
          create_src: true
      - type: volume
        source: cache
        target: /cache
        volume:
          labels:
            tier: cache
`, `
services:
  foo:
    mounts:
      - type: bind
        source: /other
        target: /bind
      - type: tmpfs
        target: /scratch
`, `
services:
  foo:
    image: alpine
    mounts:
      - type: bind
        source: /other
        target: /bind
      - type: volume
        source: cache
        target: /cache
        volume:
          labels:
            tier: cache
      - type: tmpfs
        target: /scratch
`)
}
