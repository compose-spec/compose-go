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
	"strings"
	"unicode"
)

func getInlined(arrayName string, mapping Mapping) ([]string, error) {
	raValue, ok := mapping(arrayName)
	if !ok {
		return nil, nil
	}

	trimmed, err := trim(raValue)
	if err != nil {
		return nil, formatInlinedArrayParsingError(err, raValue)
	}

	result, err := parse(trimmed)
	if err != nil {
		return nil, formatInlinedArrayParsingError(err, raValue)
	}

	return result, nil
}

func trim(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 2 || trimmed[0] != '(' || trimmed[len(trimmed)-1] != ')' {
		return "", fmt.Errorf("should be enclosed in parenthesis")
	}
	return strings.TrimSpace(trimmed[1 : len(trimmed)-1]), nil
}

func formatInlinedArrayParsingError(cause error, rawValue string) error {
	return fmt.Errorf("invalid array definition: \"%s\" - %w", rawValue, cause)
}

func parse(value string) ([]string, error) {
	var result []string
	var current strings.Builder
	var inSingleQuote, inDoubleQuote, escaping bool

	for _, char := range value {
		switch {

		case escaping:
			current.WriteRune(char)
			escaping = false

		case char == '(' || char == ')':
			return nil, fmt.Errorf("unescaped character (\"%c\")", char)

		case char == '\\':
			escaping = true

		case char == '"' && !inSingleQuote:
			inDoubleQuote = !inDoubleQuote

		case char == '\'' && !inDoubleQuote:
			inSingleQuote = !inSingleQuote

		case unicode.IsSpace(char) && !inSingleQuote && !inDoubleQuote:
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}

		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	if inSingleQuote || inDoubleQuote {
		return nil, fmt.Errorf("quote not closed")
	}

	if escaping {
		return nil, fmt.Errorf("nothing left to escape")
	}

	return result, nil
}
