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

import "fmt"

// SelectModelServices restricts dict["services"] in place to the given root
// service names plus the transitive closure of their depends_on, and returns
// the dict. An empty names slice is a no-op. A root name that doesn't match
// any declared service is an error, reported before any mutation. depends_on
// entries are expected in the canonical map form produced by the loader;
// entries that are not, or whose key doesn't match any declared service (e.g.
// an uninterpolated "${VAR}" under SkipInterpolation), are silently ignored.
func SelectModelServices(dict map[string]any, names []string) (map[string]any, error) {
	if len(names) == 0 {
		return dict, nil
	}

	services, _ := dict["services"].(map[string]any)

	for _, name := range names {
		if _, ok := services[name]; !ok {
			return nil, fmt.Errorf("no such service: %s", name)
		}
	}

	selected := map[string]bool{}
	queue := append([]string{}, names...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		if selected[name] {
			continue
		}
		selected[name] = true

		service, ok := services[name].(map[string]any)
		if !ok {
			continue
		}
		dependsOn, ok := service["depends_on"].(map[string]any)
		if !ok {
			continue
		}
		for dep := range dependsOn {
			if _, ok := services[dep]; ok && !selected[dep] {
				queue = append(queue, dep)
			}
		}
	}

	for name := range services {
		if !selected[name] {
			delete(services, name)
		}
	}

	return dict, nil
}
