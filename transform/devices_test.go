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

	"gotest.tools/v3/assert"
)

func TestExpandDevicesAll(t *testing.T) {
	assertExpand(t, `
services:
  test:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
`, `
services:
  test:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: -1

`)
}

func TestExpandDevicesCountAsString(t *testing.T) {
	assertExpand(t, `
services:
  test:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: "1"
`, `
services:
  test:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: 1

`)
}

func TestExpandDevicesCountInvalidString(t *testing.T) {
	_, err := ExpandShortSyntax(unmarshall(t, `
services:
  test:
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: some_string
`))
	assert.Error(t, err, "invalid string value for 'count' (the only value allowed is 'all' or a number)")

}
