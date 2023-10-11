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

package validation

import (
	"fmt"

	"github.com/compose-spec/compose-go/consts"
	"github.com/compose-spec/compose-go/tree"
)

func checkExternalVolume(v map[string]any, p tree.Path) error {
	if _, ok := v["external"]; !ok {
		return nil
	}
	for k, e := range v {
		switch k {
		case "name", consts.Extensions:
			continue
		case "external":
			vname := v["name"]
			ename, ok := e.(map[string]any)["name"]
			if ok && vname != nil && ename != vname {
				return fmt.Errorf("volume %s: volume.external.name and volume.name conflict; only use volume.name", p.Last())
			}
		default:
			return fmt.Errorf("conflicting parameters \"external\" and %q specified for volume %q", k, p.Last())
		}
	}
	return nil
}
