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

package schema

import (
	"bytes"
	"errors"
	"regexp"
	"slices"
	"strings"

	// Enable support for embedded static resources
	_ "embed"
	"encoding/json"

	"github.com/compose-spec/compose-go/v2/utils"
	"github.com/santhosh-tekuri/jsonschema"
)

type ComposeSchema struct {
	schema *jsonschema.Schema
}

// Schema is the compose-spec JSON schema
//
//go:embed compose-spec.json
var Schema string

func CreateComposeSchema() (ComposeSchema, error) {
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft7

	err := c.AddResource("compose-spec", strings.NewReader(Schema))
	if err != nil {
		return ComposeSchema{}, err
	}
	sch, err := c.Compile("compose-spec")
	if err != nil {
		return ComposeSchema{}, err
	}
	return ComposeSchema{schema: sch}, nil
}

func (c *ComposeSchema) Validate(config map[string]interface{}) error {
	v, err := json.Marshal(config)
	if err != nil {
		return err
	}
	err = c.schema.Validate(bytes.NewReader(v))
	if err != nil {
		return formatError(err)
	}
	return nil
}

func formatError(result error) error {
	message := result.Error()
	allErrors := strings.Split(message, "\n")

	// Iterate over errors and...
	infos := []map[string]string{}
	for _, e := range allErrors {
		if !ignoreError(e) {
			info := getErrorInfo(e) // ...extract metadata from each line
			infos = append(infos, info)
		}
	}
	// Sort so that the first element is the deepest leaf
	slices.SortStableFunc(infos, func(a, b map[string]string) int {
		depthA := depth(a["Path"])
		depthB := depth(b["Path"])
		if depthB == depthA {
			return depth(b["Definition"]) - depth(a["Definition"])
		}
		return depthB - depthA
	})
	description := getDescription(infos[0]["Message"], infos[0]["Path"])
	return errors.New(description)
}

func depth(s string) int {
	return len(strings.Split(s, "/"))
}

func getDescription(message, prefix string) string {
	var res string
	switch {
	case strings.Contains(message, "must be"):
		res = strings.Replace(message, "must", "Must", 1)
		res = strings.ReplaceAll(res, "but found", "")
		switch {
		case strings.Contains(message, ">="):
			res = strings.ReplaceAll(res, ">=", "greater than or equal to")
		case strings.Contains(message, "<="):
			res = strings.ReplaceAll(res, "<=", "lesser than or equal to")
		case strings.Contains(message, ">"):
			res = strings.ReplaceAll(res, ">", "greater than")
		case strings.Contains(message, "<"):
			res = strings.ReplaceAll(res, "<", "lesser than")
		case strings.Contains(message, "="):
			res = strings.ReplaceAll(res, "=", "equal to")
		}
	case strings.Contains(message, "expected"):
		res = utils.TrimRightFrom(message, ", ")
		res = strings.Replace(res, "expected", "must be a", 1)

		res = strings.ReplaceAll(res, "object", "mapping")
		res = strings.ReplaceAll(res, "array", "list")

		or := strings.Count(res, "or")
		res = strings.Replace(res, " or", ",", or-1)

	case strings.Contains(message, "additionalProperties"):
		res = strings.ReplaceAll(message, "additionalProperties \"", "Additional property ")
		res = strings.ReplaceAll(res, "\" not allowed", " is not allowed")

	case strings.Contains(message, "missing properties"):
		res = strings.ReplaceAll(message, "\"", "")
		res = strings.ReplaceAll(res, "missing properties: ", "")
		res += " is required"
	}

	if prefix == "#" {
		prefix = ""
	} else {
		prefix = utils.TrimLeftFrom(prefix, "#/")
		prefix = strings.ReplaceAll(prefix, "/", ".")
	}
	if res == "" {
		// This error was not contemplated
		res = message
	}
	return prefix + " " + res
}

// Creates a map with the tree <Path> from yaml of the keyword
// <Definition> rule from compose-spec.json that caused the fail
// and the error <Message>
func getErrorInfo(message string) map[string]string {
	message = strings.TrimLeft(message, " ")

	var compRegEx = regexp.MustCompile(`I\[(?P<Path>#\/?(\w*\/*\W*)+)\]\ S\[(?P<Definition>#\/?(\w*\/*\W*)+)\]\ (?P<Message>(\w+\W*)+)`)
	match := compRegEx.FindStringSubmatch(message)

	info := make(map[string]string)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) && name != "" {
			info[name] = match[i]
		}
	}
	return info
}

func ignoreError(message string) bool {
	return strings.Contains(message, "doesn't validate with") || strings.Contains(message, "oneOf failed")
}
