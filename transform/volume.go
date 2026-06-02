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
	"path"

	"github.com/compose-spec/compose-go/v3/format"
	"github.com/compose-spec/compose-go/v3/tree"
)

func transformVolumeMount(data any, p tree.Path, ignoreParseError bool) (any, error) {
	switch v := data.(type) {
	case map[string]any:
		return v, nil
	case string:
		volume, err := format.ParseVolume(v) // TODO(ndeloof) ParseVolume should not rely on types and return map[string]
		if err != nil {
			if ignoreParseError {
				return v, nil
			}
			return nil, err
		}
		volume.Target = cleanTarget(volume.Target)
		volume.Source = cleanSource(volume.Source)

		return encode(volume)
	default:
		return data, fmt.Errorf("%s: invalid type %T for service volume mount", p, v)
	}
}

func cleanTarget(target string) string {
	if target == "" {
		return ""
	}
	return path.Clean(target)
}

// cleanSource normalizes the only short-form source spelling that v2
// produces a different decoded form for vs v3: "./" collapses to "."
// in v2 because filepath.Join in the sub-resolve cleans it, while v3's
// node-level resolver preserves the literal "./" until the format
// parser sees it. Mirror v2 here by rewriting "./" -> "." in place,
// keeping the `.` prefix that format.ParseVolume relies on to flag the
// value as a bind path. Other short-form spellings (`./foo`, `/abs`,
// `name`) are left alone so format.ParseVolume / Windows path
// conversion observe the original characters.
func cleanSource(source string) string {
	if source == "./" {
		return "."
	}
	return source
}

func defaultVolumeBind(data any, p tree.Path, _ bool) (any, error) {
	bind, ok := data.(map[string]any)
	if !ok {
		return data, fmt.Errorf("%s: invalid type %T for service volume bind", p, data)
	}
	if _, ok := bind["create_host_path"]; !ok {
		bind["create_host_path"] = true
	}
	return bind, nil
}
