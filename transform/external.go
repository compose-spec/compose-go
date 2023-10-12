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

	"github.com/compose-spec/compose-go/tree"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func transformExternal(data any, p tree.Path) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		if _, ok := v["name"]; ok {
			logrus.Warnf("%s: external.name is deprecated in favor of name", p.Parent())
		}
		if _, ok := v["external"]; !ok {
			v["external"] = true
		}
		return v, nil
	case bool:
		return map[string]any{
			"external": v,
		}, nil
	default:
		return data, errors.Errorf("invalid type %T for external", v)
	}
}

func transformMaybeExternal(data any, p tree.Path) (any, error) {
	if data == nil {
		return data, nil
	}
	resource, err := transformMapping(data.(map[string]any), p)
	if err != nil {
		return nil, err
	}

	if ext, ok := resource["external"]; ok {
		external, ok := ext.(map[string]any)
		if !ok {
			return resource, nil
		}
		name := resource["name"]
		if ename, ok := external["name"]; ok {
			if name != nil && ename != name {
				return nil, fmt.Errorf("%s: name and external.name conflict; only use name", p)
			}
			delete(external, "name")
			resource["name"] = ename
		} else {
			if name == nil {
				resource["name"] = p.Last()
			}
		}
	}

	return resource, nil
}
