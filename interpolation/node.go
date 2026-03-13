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

package interpolation

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
)

// InterpolateNode replaces variables in yaml.Node scalar values
func InterpolateNode(node *yaml.Node, opts Options) error {
	if opts.LookupValue == nil {
		opts.LookupValue = os.LookupEnv
	}
	if opts.TypeCastMapping == nil {
		opts.TypeCastMapping = make(map[tree.Path]Cast)
	}
	if opts.Substitute == nil {
		opts.Substitute = template.Substitute
	}
	return recursiveInterpolateNode(node, tree.NewPath(), opts)
}

func recursiveInterpolateNode(node *yaml.Node, path tree.Path, opts Options) error {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			return recursiveInterpolateNode(node.Content[0], path, opts)
		}
		return nil

	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if err := recursiveInterpolateNode(value, path.Next(key.Value), opts); err != nil {
				return err
			}
		}
		return nil

	case yaml.SequenceNode:
		for _, item := range node.Content {
			if err := recursiveInterpolateNode(item, path.Next(tree.PathMatchList), opts); err != nil {
				return err
			}
		}
		return nil

	case yaml.ScalarNode:
		if node.Tag != "!!str" && node.Tag != "" && !strings.Contains(node.Value, "$") {
			return nil
		}
		newValue, err := opts.Substitute(node.Value, template.Mapping(opts.LookupValue))
		if err != nil {
			return newPathError(path, err)
		}
		caster, ok := opts.getCasterForPath(path)
		if !ok {
			if newValue != node.Value {
				node.Value = newValue
			}
			return nil
		}
		casted, err := caster(newValue)
		if err != nil {
			return newPathError(path, fmt.Errorf("failed to cast to expected type: %w", err))
		}
		switch casted.(type) {
		case bool:
			node.Tag = "!!bool"
			node.Value = fmt.Sprint(casted)
		case int, int64:
			node.Tag = "!!int"
			node.Value = fmt.Sprint(casted)
		case float64:
			node.Tag = "!!float"
			node.Value = fmt.Sprint(casted)
		case nil:
			node.Tag = "!!null"
			node.Value = "null"
		case string:
			node.Value = fmt.Sprint(casted)
		default:
			node.Value = fmt.Sprint(casted)
		}
		return nil

	default:
		return nil
	}
}
