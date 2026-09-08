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

package loader

import (
	"strings"

	"github.com/compose-spec/compose-go/v2/tree"
)

// reportUnsupportedAttributes walks the merged model and reports, through
// warn, every attribute matching none of the matcher's declared paths. The
// walk mirrors the path conventions used across compose-go: mapping keys
// append their name, sequence items append [tree.PathMatchList].
//
// The screening runs on what the user actually wrote (the merged,
// canonicalized model, before normalization adds runtime defaults), so a
// reported path points at an attribute of the compose file. Extension keys
// (x-*) are specification-blessed escape hatches and are never reported;
// an unsupported node is reported once, without descending into it.
func reportUnsupportedAttributes(value any, matcher *tree.Matcher, path tree.Path, warn func(tree.Path)) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if strings.HasPrefix(key, "x-") {
				continue
			}
			next := path.Next(key)
			// a matched node is accepted for itself and its children are
			// screened independently; an unmatched node may still hold
			// declared attributes below (MayContain) and then descends
			// silently — anything else is reported once, undescended
			if matcher.Matches(next) || matcher.MayContain(next) {
				reportUnsupportedAttributes(child, matcher, next, warn)
				continue
			}
			warn(next)
		}
	case []any:
		for _, item := range v {
			reportUnsupportedAttributes(item, matcher, path.Next(tree.PathMatchList), warn)
		}
	}
}
