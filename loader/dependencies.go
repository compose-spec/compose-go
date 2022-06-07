/*
   Copyright 2020 Docker Compose CLI authors

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
	"fmt"
	"github.com/compose-spec/compose-go/types"
	"github.com/compose-spec/compose-go/utils"
	"strings"
	"sync"
)

// Graph represents project as service dependencies
type Graph struct {
	Vertices map[string]*Vertex
	lock     sync.RWMutex
}

// Vertex represents a service in the dependencies structure
type Vertex struct {
	Children map[string]*Vertex
}

// GetChildren returns a slice with the child vertexes of the a Vertex
func (v *Vertex) GetChildren() []*Vertex {
	var res []*Vertex
	for _, p := range v.Children {
		res = append(res, p)
	}
	return res
}

// NewGraph returns the dependency graph of the services
func NewGraph(services types.Services) *Graph {
	graph := &Graph{
		lock:     sync.RWMutex{},
		Vertices: map[string]*Vertex{},
	}

	for _, s := range services {
		graph.AddVertex(s.Name)
	}

	for _, s := range services {
		for _, name := range s.GetDependencies() {
			_ = graph.AddEdge(s.Name, name)
		}
	}

	return graph
}

// NewVertex is the constructor function for the Vertex
func NewVertex() *Vertex {
	return &Vertex{
		Children: map[string]*Vertex{},
	}
}

// AddVertex adds a vertex to the Graph
func (g *Graph) AddVertex(service string) {
	g.lock.Lock()
	defer g.lock.Unlock()

	v := NewVertex()
	g.Vertices[service] = v
}

// AddEdge adds a relationship of dependency between vertexes `source` and `destination`
func (g *Graph) AddEdge(source string, destination string) error {
	g.lock.Lock()
	defer g.lock.Unlock()

	sourceVertex := g.Vertices[source]
	destinationVertex := g.Vertices[destination]

	if sourceVertex == nil {
		return fmt.Errorf("could not find %s", source)
	}
	if destinationVertex == nil {
		return fmt.Errorf("could not find %s", destination)
	}

	// If they are already connected
	if _, ok := sourceVertex.Children[destination]; ok {
		return nil
	}

	sourceVertex.Children[destination] = destinationVertex

	return nil
}

// HasCycles detects cycles in the graph
func (g *Graph) HasCycles() (bool, error) {
	discovered := []string{}
	finished := []string{}

	for service := range g.Vertices {
		path := []string{service}
		if !utils.StringContains(discovered, service) && !utils.StringContains(finished, service) {
			var err error
			discovered, finished, err = g.visit(service, path, discovered, finished)

			if err != nil {
				return true, err
			}
		}
	}

	return false, nil
}

func (g *Graph) visit(service string, path []string, discovered []string, finished []string) ([]string, []string, error) {
	discovered = append(discovered, service)

	for child := range g.Vertices[service].Children {
		path := append(path, child)
		if utils.StringContains(discovered, child) {
			return nil, nil, fmt.Errorf("cycle found: %s", strings.Join(path, " -> "))
		}

		if !utils.StringContains(finished, child) {
			if _, _, err := g.visit(child, path, discovered, finished); err != nil {
				return nil, nil, err
			}
		}
	}

	discovered = remove(discovered, service)
	finished = append(finished, service)
	return discovered, finished, nil
}

func remove(slice []string, item string) []string {
	var s []string
	for _, i := range slice {
		if i != item {
			s = append(s, i)
		}
	}
	return s
}
