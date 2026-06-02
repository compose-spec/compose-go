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
	"strings"

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

// cleanSource normalizes the short-form source value (`./` -> `.`,
// `./foo` -> `foo`). Uses path.Clean (forward slashes only) so the
// result matches what filepath.Join would produce on Linux and stays
// stable across host OSes -- the compose-spec semantics treat the
// host portion of the short form as POSIX-style. Absolute paths
// (including Windows-style C:\) and bare relative paths without a
// leading `.` are left untouched.
func cleanSource(source string) string {
	if source == "" {
		return source
	}
	if !strings.HasPrefix(source, "./") && source != "." {
		return source
	}
	return path.Clean(source)
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
