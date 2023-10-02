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

package merge

import (
	"sort"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/tree"
	"github.com/compose-spec/compose-go/utils"
)

type indexer func(interface{}) (string, error)

// mergeSpecials defines the custom rules applied by compose when merging yaml trees
var unique = map[tree.Path]indexer{}

func init() {
	unique["services.*.environment"] = environmentIndexer
	unique["services.*.volumes"] = volumeIndexer
}

// EnforceUnicity removes redefinition of elements declared in a sequence
func EnforceUnicity(value interface{}, p tree.Path) (interface{}, error) {
	switch v := value.(type) {
	case map[string]interface{}:
		for k, e := range v {
			u, err := EnforceUnicity(e, p.Next(k))
			if err != nil {
				return nil, err
			}
			v[k] = u
		}
		return v, nil
	case []interface{}:
		uniq := make(map[string]interface{}, len(v))
		for pattern, indexer := range unique {
			if p.Matches(pattern) {
				for _, entry := range v {
					key, err := indexer(entry)
					if err != nil {
						return nil, err
					}
					uniq[key] = entry
				}
				keys := utils.MapKeys(uniq)
				sort.Strings(keys)

				seq := make([]interface{}, len(uniq))
				i := 0
				for _, k := range keys {
					seq[i] = uniq[k]
					i++
				}
				return seq, nil
			}
		}
	}
	return value, nil
}

func environmentIndexer(y interface{}) (string, error) {
	value := y.(string)
	key, _, found := strings.Cut(value, "=")
	if !found {
		return value, nil
	}
	return key, nil
}

func volumeIndexer(y interface{}) (string, error) {
	switch value := y.(type) {
	case map[string]interface{}:
		return value["target"].(string), nil
	case string:
		volume, err := loader.ParseVolume(value)
		if err != nil {
			return "", err
		}
		return volume.Target, nil
	}
	return "", nil
}
