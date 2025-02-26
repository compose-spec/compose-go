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
	matches := ArraySubstitutionPattern.FindStringSubmatch(template)
	prefix, err := findPrefix(matches)

	if err != nil {
		return nil, fmt.Errorf("invalid array template: %s", template)
	}

	arr := make([]string, 0, maxArraySize)
	for i := 0; i < maxArraySize; i++ {
		key := fmt.Sprintf("%s[%d]", prefix, i)
		value, ok := mapping(key)
		if !ok {
			break
		}
		arr = append(arr, value)
	}

	return arr, nil
}

func findPrefix(matches []string) (string, error) {
	for _, match := range matches[1:] {
		if match != "" {
			return match, nil
		}
	}

	return "", fmt.Errorf("could not find a match")
}
