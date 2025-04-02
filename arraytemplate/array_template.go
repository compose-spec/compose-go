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

package arraytemplate

import (
	"fmt"
	"regexp"
)

type Mapping func(string) (string, bool)

const (
	maxArraySize = 100

	substitution = "[_a-zA-Z][_a-zA-Z0-9]*"
)

var (
	rawPattern = fmt.Sprintf(
		"^\\$(?:(%s)\\[\\*]|{(%s)\\[\\*]})$",
		substitution,
		substitution,
	)

	ArraySubstitutionPattern = regexp.MustCompile(rawPattern)
)

func Substitute(template string, mapping Mapping) ([]string, error) {
	arrayName, err := findArrayName(template)
	if err != nil {
		return nil, err
	}
	inlined, err := getInlined(arrayName, mapping)
	if err != nil {
		return nil, fmt.Errorf("could not substitute array template \"%s\":\n%w", template, err)
	}

	if inlined != nil {
		return inlined, nil
	}

	return getIndexed(arrayName, mapping), nil
}

func findArrayName(template string) (string, error) {
	matches := ArraySubstitutionPattern.FindStringSubmatch(template)

	if len(matches) < 1 {
		return "", fmt.Errorf("not a valid array template: \"%s\"", template)
	}

	var arrayName string
	for _, match := range matches[1:] {
		if match != "" {
			arrayName = match
			break
		}
	}

	if arrayName == "" {
		return "", fmt.Errorf("this message suggest an internal error and should never occur; if you see this error, please report it: \"%s\"", template)
	}

	return arrayName, nil
}
