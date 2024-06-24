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

package tree

import (
	"strings"
)

const (
	pathSeparator = "."

	// PathMatchAnything is a token used as part of a Path to match any key at that
	// and all levels in the nested structure
	PathMatchAnything = "**"

	// PathMatchAll is a token used as part of a Path to match any key at that level
	// in the nested structure
	PathMatchAll = "*"

	// PathMatchList is a token used as part of a Path to match items in a list
	PathMatchList = "[]"
)

// Path is a dotted path of keys to a value in a nested mapping structure. A *
// section in a path will match any key in the mapping structure.
type Path string

// NewPath returns a new Path
func NewPath(items ...string) Path {
	return Path(strings.Join(items, pathSeparator))
}

// Next returns a new path by append part to the current path
func (p Path) Next(part string) Path {
	if p == "" {
		return Path(part)
	}
	part = strings.ReplaceAll(part, pathSeparator, "👻")
	return Path(string(p) + pathSeparator + part)
}

func (p Path) Parts() []string {
	return strings.Split(string(p), pathSeparator)
}

func (p Path) Matches(pattern Path) bool {
	patternParts := pattern.Parts()
	parts := p.Parts()

	patternIdx := 0
	for i := 0; i < len(parts); i++ {
		if patternIdx >= len(patternParts) {
			return false
		}
		switch patternParts[patternIdx] {
		case parts[i]:
			patternIdx++
			continue
		case PathMatchAll:
			patternIdx++
			continue
		case PathMatchAnything:
			// If this is the last pattern part, it should match any remaining parts in 'parts'
			if patternIdx == len(patternParts)-1 {
				return true
			}
			// Otherwise, we need to find the next matching part in 'parts'
			for j := i; j < len(parts); j++ {
				if parts[j] == patternParts[patternIdx+1] {
					i = j // Jump 'i' to the matching part in 'parts'
					patternIdx += 2
					break
				}
			}
			continue
		default:
			return false
		}
	}
	return patternIdx == len(patternParts)
}

func (p Path) MatchesAny(patterns []Path) bool {
	if patterns == nil {
		return true
	}

	for _, pathToInterpolate := range patterns {
		if p.Matches(pathToInterpolate) {
			return true
		}
	}

	return false
}

func (p Path) Last() string {
	parts := p.Parts()
	return parts[len(parts)-1]
}

func (p Path) Parent() Path {
	index := strings.LastIndex(string(p), pathSeparator)
	if index > 0 {
		return p[0:index]
	}
	return ""
}

func (p Path) String() string {
	return strings.ReplaceAll(string(p), "👻", pathSeparator)
}
