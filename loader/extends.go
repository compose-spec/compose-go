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
	"context"
	"fmt"

	"github.com/compose-spec/compose-go/override"
	"github.com/compose-spec/compose-go/types"
)

func ApplyExtends(ctx context.Context, dict map[string]interface{}, opts *Options) error {
	services := dict["services"].(map[string]interface{})
	for name, s := range services {
		service := s.(map[string]interface{})
		x, ok := service["extends"]
		if !ok {
			continue
		}
		extends := x.(map[string]interface{})
		var base interface{}
		ref := extends["service"].(string)
		if file, ok := extends["file"]; ok {
			path := file.(string)
			for _, loader := range opts.ResourceLoaders {
				if loader.Accept(path) {
					local, err := loader.Load(ctx, path)
					if err != nil {
						return err
					}
					source, err := loadYamlModel(ctx, []types.ConfigFile{
						{Filename: local},
					}, opts)
					if err != nil {
						return err
					}
					services := source["services"].([]interface{})
					for _, s := range services {
						service := s.(map[string]interface{})
						if service["name"] == ref {
							base = service
							break
						}
					}
					if base == nil {
						return fmt.Errorf("cannot extend service %q in %s: service not found", name, path)
					}
				}
			}
			if base == nil {
				return fmt.Errorf("cannot read %s", path)
			}
		} else {
			base, ok = services[ref]
			if !ok {
				return fmt.Errorf("cannot extend service %q in %s: service not found", name, "filename") //TODO track filename
			}
		}
		merged, err := override.ExtendService(deepClone(base).(map[string]interface{}), service)
		if err != nil {
			return err
		}
		services[name] = merged
	}
	dict["services"] = services
	return nil
}

func deepClone(value interface{}) interface{} {
	switch v := value.(type) {
	case []interface{}:
		cp := make([]interface{}, len(v))
		for i, e := range v {
			cp[i] = deepClone(e)
		}
		return cp
	case map[string]interface{}:
		cp := make(map[string]interface{}, len(v))
		for k, e := range v {
			cp[k] = deepClone(e)
		}
		return cp
	default:
		return value
	}
}
