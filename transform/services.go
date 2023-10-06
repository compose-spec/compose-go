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

package transform

import (
	"github.com/compose-spec/compose-go/tree"
)

func makeServicesSlice(data interface{}, p tree.Path) (interface{}, error) {
	services := data.(map[string]interface{})
	servicesAsSlice := make([]interface{}, len(services))
	i := 0
	for name, it := range services {
		config := it.(map[string]interface{})
		config["name"] = name
		if _, ok := config["scale"]; !ok {
			config["scale"] = 1 // TODO(ndeloof) we should make Scale a *int
		}
		canonical, err := transform(config, p.Next(name))
		if err != nil {
			return nil, err
		}
		servicesAsSlice[i] = canonical
		i++
	}
	return servicesAsSlice, nil
}

func transformServiceNetworks(data interface{}, p tree.Path) (interface{}, error) {
	if slice, ok := data.([]interface{}); ok {
		networks := make(map[string]interface{}, len(slice))
		for _, net := range slice {
			networks[net.(string)] = nil
		}
		return networks, nil
	}
	return data, nil
}
