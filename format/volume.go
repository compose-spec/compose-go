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

package format

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/compose-spec/compose-go/v2/types"
)

const endOfSpec = rune(0)

// ParseVolume parses a volume spec without any knowledge of the target platform.
// A single letter followed by a colon and a path separator is taken for a
// Windows drive (`C:\data:/data`, `C:/data:/data`), unless the spec reads as a
// named volume with a single letter name (`a:/data`, `a:/data:ro`).
func ParseVolume(spec string) (types.ServiceVolumeConfig, error) {
	volume := types.ServiceVolumeConfig{}

	switch len(spec) {
	case 0:
		return volume, errors.New("invalid empty volume spec")
	case 1, 2:
		volume.Target = spec
		volume.Type = types.VolumeTypeVolume
		return volume, nil
	}

	namedLetter := startsWithNamedVolumeLetter(spec)
	var buffer []rune
	var inVarSubstitution int // Track nesting depth of ${...}
	for i, char := range spec + string(endOfSpec) {
		// Check if we're entering a variable substitution
		if char == '$' && i+1 < len(spec) && rune(spec[i+1]) == '{' {
			inVarSubstitution++
			buffer = append(buffer, char)
			continue
		}

		// Check if we're exiting a variable substitution
		if char == '}' && inVarSubstitution > 0 {
			inVarSubstitution--
			buffer = append(buffer, char)
			continue
		}

		switch {
		case isWindowsDrive(buffer, char, spec[min(i+1, len(spec)):]) && (volume.Source != "" || !namedLetter):
			buffer = append(buffer, char)
		case (char == ':' || char == endOfSpec) && inVarSubstitution == 0:
			if err := populateFieldFromBuffer(char, buffer, &volume); err != nil {
				populateType(&volume)
				return volume, fmt.Errorf("invalid spec: %s: %w", spec, err)
			}
			buffer = nil
		default:
			buffer = append(buffer, char)
		}
	}

	populateType(&volume)
	return volume, nil
}

// isWindowsDrive tells whether char, the colon following buffer, ends a Windows
// drive: a single letter, and a path separator right after the colon (rest is
// what follows it). Without the separator, `a:b` is a source and a target.
func isWindowsDrive(buffer []rune, char rune, rest string) bool {
	return char == ':' && len(buffer) == 1 && unicode.IsLetter(buffer[0]) && rest != "" && (rest[0] == '/' || rest[0] == '\\')
}

// startsWithNamedVolumeLetter tells whether spec, which starts with a single
// letter and a colon followed by a slash, is a volume named after this letter
// (`a:/data`, `a:/data:ro`) rather than a path on a Windows drive
// (`C:/data:/data`). Both read the same until the last section: one made of
// options belongs to a named volume, a target (an absolute path, or a variable
// not yet resolved, which options never look like) means the spec is a bind
// mount from the drive, as does a spec with more sections
// (`C:/data:d:/data:ro`).
func startsWithNamedVolumeLetter(spec string) bool {
	first, size := utf8.DecodeRuneInString(spec)
	if !unicode.IsLetter(first) || !strings.HasPrefix(spec[size:], ":/") {
		return false
	}
	sections := splitSections(spec[size+1:])
	switch len(sections) {
	case 1:
		return true
	case 2:
		// a target means a bind mount from a drive, options a named volume
		// (an unresolved variable is more likely a target than a mode)
		return !strings.HasPrefix(sections[1], "/") && !strings.HasPrefix(sections[1], `\`) && !strings.HasPrefix(sections[1], "$")
	default:
		return false
	}
}

// splitSections splits spec on its colons, but those of a variable substitution.
func splitSections(spec string) []string {
	var sections []string
	depth, start := 0, 0
	for i := 0; i < len(spec); i++ {
		switch {
		case strings.HasPrefix(spec[i:], "${"):
			depth++
			i++
		case spec[i] == '}' && depth > 0:
			depth--
		case spec[i] == ':' && depth == 0:
			sections = append(sections, spec[start:i])
			start = i + 1
		}
	}
	return append(sections, spec[start:])
}

func populateFieldFromBuffer(char rune, buffer []rune, volume *types.ServiceVolumeConfig) error {
	strBuffer := string(buffer)
	switch {
	case len(buffer) == 0:
		return errors.New("empty section between colons")
	// Anonymous volume
	case volume.Source == "" && char == endOfSpec:
		volume.Target = strBuffer
		return nil
	case volume.Source == "":
		volume.Source = strBuffer
		return nil
	case volume.Target == "":
		volume.Target = strBuffer
		return nil
	case char == ':':
		return errors.New("too many colons")
	}
	for _, option := range strings.Split(strBuffer, ",") {
		switch option {
		case "ro":
			volume.ReadOnly = true
		case "rw":
			volume.ReadOnly = false
		case "nocopy":
			volume.Volume = &types.ServiceVolumeVolume{NoCopy: true}
		default:
			if isBindOption(option) {
				setBindOption(volume, option)
			}
			// ignore unknown options FIXME why not report an error here?
		}
	}
	return nil
}

var Propagations = []string{
	types.PropagationRPrivate,
	types.PropagationPrivate,
	types.PropagationRShared,
	types.PropagationShared,
	types.PropagationRSlave,
	types.PropagationSlave,
}

type setBindOptionFunc func(bind *types.ServiceVolumeBind, option string)

var bindOptions = map[string]setBindOptionFunc{
	types.PropagationRPrivate: setBindPropagation,
	types.PropagationPrivate:  setBindPropagation,
	types.PropagationRShared:  setBindPropagation,
	types.PropagationShared:   setBindPropagation,
	types.PropagationRSlave:   setBindPropagation,
	types.PropagationSlave:    setBindPropagation,
	types.SELinuxShared:       setBindSELinux,
	types.SELinuxPrivate:      setBindSELinux,
}

func setBindPropagation(bind *types.ServiceVolumeBind, option string) {
	bind.Propagation = option
}

func setBindSELinux(bind *types.ServiceVolumeBind, option string) {
	bind.SELinux = option
}

func isBindOption(option string) bool {
	_, ok := bindOptions[option]

	return ok
}

func setBindOption(volume *types.ServiceVolumeConfig, option string) {
	if volume.Bind == nil {
		volume.Bind = &types.ServiceVolumeBind{}
	}

	bindOptions[option](volume.Bind, option)
}

func populateType(volume *types.ServiceVolumeConfig) {
	if isFilePath(volume.Source) {
		volume.Type = types.VolumeTypeBind
		if volume.Bind == nil {
			volume.Bind = &types.ServiceVolumeBind{}
		}
		// For backward compatibility with docker-compose legacy, using short notation involves
		// bind will create missing host path
		volume.Bind.CreateHostPath = true
	} else {
		volume.Type = types.VolumeTypeVolume
		if volume.Volume == nil {
			volume.Volume = &types.ServiceVolumeVolume{}
		}
	}
}

func isFilePath(source string) bool {
	if source == "" {
		return false
	}
	switch source[0] {
	case '.', '/', '~':
		return true
	}

	// windows named pipes
	if strings.HasPrefix(source, `\\`) {
		return true
	}

	first, nextIndex := utf8.DecodeRuneInString(source)
	if len(source) <= nextIndex {
		return false
	}
	return isWindowsDrive([]rune{first}, rune(source[nextIndex]), source[nextIndex+1:])
}
