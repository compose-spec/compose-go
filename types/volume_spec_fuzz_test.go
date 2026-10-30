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

package types

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzParseVolume locks the short syntax of service `volumes`
// (https://github.com/compose-spec/compose-spec/blob/main/05-services.md#short-syntax-5):
// whatever the text, ParseVolume never panics, is deterministic, and types a
// volume it accepts as a bind mount or a named volume. A volume with a source
// and a target (neither a single letter), once rendered by String() (which the type documents as
// parseable), parses back to the same volume.
func FuzzParseVolume(f *testing.F) {
	for _, seed := range []string{
		"", "a", "ab", "/data", "data:/data", "data:/data:ro", "data:/data:rw", "./src:/app", "../src:/app:ro",
		"~/src:/app", "/host:/container:z", "/host:/container:Z", "/host:/container:ro,z", "/host:/container:rshared",
		"data:/data:nocopy", "data:/data:ro,nocopy", "/host:/container:unknown", "a:b:c:d", "::", ":a", "a:", "a::b",
		`C:\Users\data:/data`, `C:\Users\data:/data:ro`, `\\.\pipe\docker_engine:\\.\pipe\docker_engine`,
		"${DATA}:/data", "${DATA:-/srv}:/data:ro", "${A:-${B:-x}}:/data", "${", "$", "a:${B", "é:ü:ro",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, spec string) {
		volume, err := ParseVolume(spec)
		again, errAgain := ParseVolume(spec)
		if !reflect.DeepEqual(volume, again) || (err == nil) != (errAgain == nil) {
			t.Fatalf("ParseVolume(%q) is not deterministic: (%+v, %v) then (%+v, %v)", spec, volume, err, again, errAgain)
		}
		if err != nil {
			return
		}
		if volume.Type != VolumeTypeBind && volume.Type != VolumeTypeVolume {
			t.Fatalf("ParseVolume(%q) has the unexpected type %q", spec, volume.Type)
		}
		if volume.Source == "" || volume.Target == "" || utf8.RuneCountInString(volume.Source) == 1 || utf8.RuneCountInString(volume.Target) == 1 ||
			strings.ContainsRune(spec, endOfSpec) || strings.HasSuffix(volume.Source, ":") || strings.HasSuffix(volume.Target, ":") {
			// ParseVolume knows nothing of the platform: a single letter
			// followed by a colon is always taken for a Windows drive, so
			// "a:b" is one path and no longer a source and a target. Such
			// ambiguous volumes cannot be rendered back, nor can a spec
			// holding the NUL character the parser marks its end with.
			return
		}
		back, err := ParseVolume(volume.String())
		if err != nil || !reflect.DeepEqual(volume, back) {
			t.Fatalf("ParseVolume(%q) = %+v but its String() %q parses to (%+v, %v)", spec, volume, volume.String(), back, err)
		}
	})
}
