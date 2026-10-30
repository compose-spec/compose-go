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
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const endOfSpec = rune(0)

// ParseVolume parses a volume spec without any knowledge of the target platform
func ParseVolume(spec string) (ServiceVolumeConfig, error) {
	volume := ServiceVolumeConfig{}

	switch len(spec) {
	case 0:
		return volume, errors.New("invalid empty volume spec")
	case 1, 2:
		volume.Target = spec
		volume.Type = VolumeTypeVolume
		return volume, nil
	}

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
		case isWindowsDrive(buffer, char):
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

func isWindowsDrive(buffer []rune, char rune) bool {
	return char == ':' && len(buffer) == 1 && unicode.IsLetter(buffer[0])
}

func populateFieldFromBuffer(char rune, buffer []rune, volume *ServiceVolumeConfig) error {
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
			volume.Volume = &ServiceVolumeVolume{NoCopy: true}
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
	PropagationRPrivate,
	PropagationPrivate,
	PropagationRShared,
	PropagationShared,
	PropagationRSlave,
	PropagationSlave,
}

type setBindOptionFunc func(bind *ServiceVolumeBind, option string)

var bindOptions = map[string]setBindOptionFunc{
	PropagationRPrivate: setBindPropagation,
	PropagationPrivate:  setBindPropagation,
	PropagationRShared:  setBindPropagation,
	PropagationShared:   setBindPropagation,
	PropagationRSlave:   setBindPropagation,
	PropagationSlave:    setBindPropagation,
	SELinuxShared:       setBindSELinux,
	SELinuxPrivate:      setBindSELinux,
}

func setBindPropagation(bind *ServiceVolumeBind, option string) {
	bind.Propagation = option
}

func setBindSELinux(bind *ServiceVolumeBind, option string) {
	bind.SELinux = option
}

func isBindOption(option string) bool {
	_, ok := bindOptions[option]

	return ok
}

func setBindOption(volume *ServiceVolumeConfig, option string) {
	if volume.Bind == nil {
		volume.Bind = &ServiceVolumeBind{}
	}

	bindOptions[option](volume.Bind, option)
}

func populateType(volume *ServiceVolumeConfig) {
	if isFilePath(volume.Source) {
		volume.Type = VolumeTypeBind
		if volume.Bind == nil {
			volume.Bind = &ServiceVolumeBind{}
		}
		// For backward compatibility with docker-compose legacy, using short notation involves
		// bind will create missing host path
		volume.Bind.CreateHostPath = true
	} else {
		volume.Type = VolumeTypeVolume
		if volume.Volume == nil {
			volume.Volume = &ServiceVolumeVolume{}
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
	return isWindowsDrive([]rune{first}, rune(source[nextIndex]))
}
