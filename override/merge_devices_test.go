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

func Test_mergeYamlDevices(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    devices:
      - '/dev/sda:/dev/sda'
      - '/dev/sdb:/dev/sdb'
      - '/dev/sdc:/dev/sdc'
      - '/dev/sdd:/dev/sdd'
`, `
services:
  test:
    devices:
      - '/dev/sde:/dev/sde'
      - '/dev/sdf:/dev/sdf'
      - '/dev/sdg:/dev/sdg'
      - '/dev/sdh:/dev/sdh'
`, `
services:
  test:
    image: foo
    devices:
      - '/dev/sda:/dev/sda'
      - '/dev/sdb:/dev/sdb'
      - '/dev/sdc:/dev/sdc'
      - '/dev/sdd:/dev/sdd'
      - '/dev/sde:/dev/sde'
      - '/dev/sdf:/dev/sdf'
      - '/dev/sdg:/dev/sdg'
      - '/dev/sdh:/dev/sdh'
`)
}

func Test_mergeYamlDevicesOverride(t *testing.T) {
	assertMergeYaml(t, `
services:
  test:
    image: foo
    devices:
      - '/dev/sda:/dev/sda'
      - '/dev/sdb:/dev/sdb'
      - '/dev/sdc:/dev/sdc'
      - '/dev/sdd:/dev/sdd'
`, `
services:
  test:
    devices:
      - '/dev/nvme0n1p1:/dev/sda'
      - '/dev/nvme1n1p1:/dev/sdb'
      - '/dev/nvme2n1p1:/dev/sdc'
`, `
services:
  test:
    image: foo
    devices:
      - '/dev/nvme0n1p1:/dev/sda'
      - '/dev/nvme1n1p1:/dev/sdb'
      - '/dev/nvme2n1p1:/dev/sdc'
      - '/dev/sdd:/dev/sdd'
`)
}
