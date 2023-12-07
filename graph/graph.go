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

package graph

import (
	"fmt"
	"strings"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/compose-spec/compose-go/v2/utils"
	"golang.org/x/exp/slices"
)

// graph represents project as service dependencies
type graph struct {
	vertices map[string]*vertex
}

// vertex represents a service in the dependencies structure
type vertex struct {
	key      string
	service  *types.ServiceConfig
	children map[string]*vertex
	parents  map[string]*vertex
}

// newGraph creates a service graph from project
func newGraph(project *types.Project) (*graph, error) {
	g := &graph{
		vertices: map[string]*vertex{},
	}

	for name, s := range project.Services {
		g.addVertex(name, s)
	}

	for name, s := range project.Services {
		src := g.vertices[name]
		for dep, condition := range s.DependsOn {
			dest, ok := g.vertices[dep]
			if !ok {
				if condition.Required {
					if ds, exists := project.DisabledServices[dep]; exists {
						return nil, fmt.Errorf("service %q is required by %q but is disabled. Can be enabled by profiles %s", dep, name, ds.Profiles)
					}
					return nil, fmt.Errorf("service %q depends on unknown service %q", name, dep)
				}
				delete(s.DependsOn, name)
				project.Services[name] = s
				continue
			}
			src.children[dep] = dest
			dest.parents[name] = src
		}
	}

	err := g.checkCycle()
	return g, err
}

func (g *graph) addVertex(name string, service types.ServiceConfig) {
	g.vertices[name] = &vertex{
		key:      name,
		service:  &service,
		parents:  map[string]*vertex{},
		children: map[string]*vertex{},
	}
}

func (g *graph) addEdge(src, dest string) {
	g.vertices[src].children[dest] = g.vertices[dest]
	g.vertices[dest].parents[src] = g.vertices[src]
}

func (g *graph) roots() []*vertex {
	var res []*vertex
	for _, v := range g.vertices {
		if len(v.parents) == 0 {
			res = append(res, v)
		}
	}
	return res
}

func (g *graph) leaves() []*vertex {
	var res []*vertex
	for _, v := range g.vertices {
		if len(v.children) == 0 {
			res = append(res, v)
		}
	}

	return res
}

func (g *graph) checkCycle() error {
	// iterate on verticles in a name-order to render a predicable error message
	// this is required by tests and enforce command reproducibility by user, which otherwise could be confusing
	names := utils.MapKeys(g.vertices)
	for _, name := range names {
		err := searchCycle([]string{name}, g.vertices[name])
		if err != nil {
			return err
		}
	}
	return nil
}

func searchCycle(path []string, v *vertex) error {
	names := utils.MapKeys(v.children)
	for _, name := range names {
		if i := slices.Index(path, name); i > 0 {
			return fmt.Errorf("dependency cycle detected: %s", strings.Join(path[i:], " -> "))
		}
		ch := v.children[name]
		err := searchCycle(append(path, name), ch)
		if err != nil {
			return err
		}
	}
	return nil
}

// descendents return all descendents for a vertex, might contain duplicates
func (v *vertex) descendents() []string {
	var vx []string
	for _, n := range v.children {
		vx = append(vx, n.key)
		vx = append(vx, n.descendents()...)
	}
	return vx
}
