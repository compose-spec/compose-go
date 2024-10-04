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
	"fmt"

	"github.com/compose-spec/compose-go/v2/tree"
)

func transformDeviceRequest(data any, p tree.Path, ignoreParseError bool) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		_, hasCount := v["count"]
		_, hasIds := v["device_ids"]
		if hasCount && hasIds {
			return nil, fmt.Errorf(`%s: "count" and "device_ids" attributes are exclusive`, p)
		}
		if !hasCount && !hasIds {
			v["count"] = "all"
		}
		return transformMapping(v, p, ignoreParseError)
	default:
		return data, fmt.Errorf("%s: invalid type %T for device request", p, v)
	}
}
