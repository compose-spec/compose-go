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
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/compose-spec/compose-go/v2/utils"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestTraversalWithMultipleParents(t *testing.T) {
	dependent := types.ServiceConfig{
		Name:      "dependent",
		DependsOn: make(types.DependsOnConfig),
	}

	project := types.Project{
		Services: types.Services{"dependent": dependent},
	}

	for i := 1; i <= 100; i++ {
		name := fmt.Sprintf("svc_%d", i)
		dependent.DependsOn[name] = types.ServiceDependency{}

		svc := types.ServiceConfig{Name: name}
		project.Services[name] = svc
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	svc := make(chan string, 10)
	seen := make(map[string]int)
	done := make(chan struct{})
	go func() {
		for service := range svc {
			seen[service]++
		}
		done <- struct{}{}
	}()

	err := InDependencyOrder(ctx, &project, func(ctx context.Context, name string, _ types.ServiceConfig) error {
		svc <- name
		return nil
	})
	require.NoError(t, err, "Error during iteration")
	close(svc)
	<-done

	assert.Equal(t, len(seen), 101)
	for svc, count := range seen {
		assert.Equal(t, 1, count, "service: %s", svc)
	}
}

func TestInDependencyUpCommandOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var order []string
	result, err := CollectInDependencyOrder(ctx, exampleProject(),
		func(ctx context.Context, name string, _ types.ServiceConfig) (string, error) {
			order = append(order, name)
			return name, nil
		}, WithMaxConcurrency(10))
	require.NoError(t, err, "Error during iteration")
	require.Equal(t, []string{"test3", "test2", "test1"}, order)
	assert.DeepEqual(t, result, map[string]string{
		"test1": "test1",
		"test2": "test2",
		"test3": "test3",
	})
}

func TestInDependencyReverseDownCommandOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var order []string
	fn := func(ctx context.Context, name string, _ types.ServiceConfig) error {
		order = append(order, name)
		return nil
	}
	err := InDependencyOrder(ctx, exampleProject(), fn, InReverseOrder)
	require.NoError(t, err, "Error during iteration")
	require.Equal(t, []string{"test1", "test2", "test3"}, order)
}

func TestBuildGraph(t *testing.T) {
	testCases := []struct {
		desc             string
		services         types.Services
		disabled         types.Services
		expectedVertices map[string]*vertex
		expectedError    string
	}{
		{
			desc: "builds graph with single service",
			services: types.Services{
				"test": {
					Name:      "test",
					DependsOn: types.DependsOnConfig{},
				},
			},
			expectedVertices: map[string]*vertex{
				"test": {
					key:      "test",
					service:  &types.ServiceConfig{Name: "test"},
					children: map[string]*vertex{},
					parents:  map[string]*vertex{},
				},
			},
		},
		{
			desc: "builds graph with two separate services",
			services: types.Services{
				"test": {
					Name:      "test",
					DependsOn: types.DependsOnConfig{},
				},
				"another": {
					Name:      "another",
					DependsOn: types.DependsOnConfig{},
				},
			},
			expectedVertices: map[string]*vertex{
				"test": {
					key:      "test",
					service:  &types.ServiceConfig{Name: "test"},
					children: map[string]*vertex{},
					parents:  map[string]*vertex{},
				},
				"another": {
					key:      "another",
					service:  &types.ServiceConfig{Name: "another"},
					children: map[string]*vertex{},
					parents:  map[string]*vertex{},
				},
			},
		},
		{
			desc: "builds graph with a service and a dependency",
			services: types.Services{
				"test": {
					Name: "test",
					DependsOn: types.DependsOnConfig{
						"another": types.ServiceDependency{},
					},
				},
				"another": {
					Name:      "another",
					DependsOn: types.DependsOnConfig{},
				},
			},
			expectedVertices: map[string]*vertex{
				"test": {
					key:     "test",
					service: &types.ServiceConfig{Name: "test"},
					children: map[string]*vertex{
						"another": {},
					},
					parents: map[string]*vertex{},
				},
				"another": {
					key:      "another",
					service:  &types.ServiceConfig{Name: "another"},
					children: map[string]*vertex{},
					parents: map[string]*vertex{
						"test": {},
					},
				},
			},
		},
		{
			desc: "builds graph with a service and optional (missing) dependency",
			services: types.Services{
				"test": {
					Name: "test",
					DependsOn: types.DependsOnConfig{
						"another": types.ServiceDependency{
							Required: false,
						},
					},
				},
			},
			expectedVertices: map[string]*vertex{
				"test": {
					key:      "test",
					service:  &types.ServiceConfig{Name: "test"},
					children: map[string]*vertex{},
					parents:  map[string]*vertex{},
				},
			},
		},
		{
			desc: "builds graph with a service and required (missing) dependency",
			services: types.Services{
				"test": {
					Name: "test",
					DependsOn: types.DependsOnConfig{
						"another": types.ServiceDependency{
							Required: true,
						},
					},
				},
			},
			expectedError: `service "test" depends on unknown service "another"`,
		},
		{
			desc: "builds graph with a service and disabled dependency",
			services: types.Services{
				"test": {
					Name: "test",
					DependsOn: types.DependsOnConfig{
						"another": types.ServiceDependency{
							Required: true,
						},
					},
				},
			},
			disabled: types.Services{
				"another": {
					Name:      "another",
					Profiles:  []string{"test"},
					DependsOn: types.DependsOnConfig{},
				},
			},
			expectedError: `service "another" is required by "test" but is disabled. Can be enabled by profiles [test]`,
		},
		{
			desc: "builds graph with multiple dependency levels",
			services: types.Services{
				"test": {
					Name: "test",
					DependsOn: types.DependsOnConfig{
						"another": types.ServiceDependency{},
					},
				},
				"another": {
					Name: "another",
					DependsOn: types.DependsOnConfig{
						"another_dep": types.ServiceDependency{},
					},
				},
				"another_dep": {
					Name:      "another_dep",
					DependsOn: types.DependsOnConfig{},
				},
			},
			expectedVertices: map[string]*vertex{
				"test": {
					key:     "test",
					service: &types.ServiceConfig{Name: "test"},
					children: map[string]*vertex{
						"another": {},
					},
					parents: map[string]*vertex{},
				},
				"another": {
					key:     "another",
					service: &types.ServiceConfig{Name: "another"},
					children: map[string]*vertex{
						"another_dep": {},
					},
					parents: map[string]*vertex{
						"test": {},
					},
				},
				"another_dep": {
					key:      "another_dep",
					service:  &types.ServiceConfig{Name: "another_dep"},
					children: map[string]*vertex{},
					parents: map[string]*vertex{
						"another": {},
					},
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			project := types.Project{
				Services:         tC.services,
				DisabledServices: tC.disabled,
			}

			graph, err := newGraph(&project)
			if tC.expectedError != "" {
				assert.Error(t, err, tC.expectedError)
				return
			}

			assert.NilError(t, err, fmt.Sprintf("failed to build graph for: %s", tC.desc))
			for k, vertex := range graph.vertices {
				expected, ok := tC.expectedVertices[k]
				assert.Equal(t, true, ok)
				assertVertexEqual(t, *expected, *vertex)
			}
		})
	}
}

func Test_detectCycle(t *testing.T) {
	graph := exampleGraph()
	graph.addEdge("B", "D")
	err := graph.checkCycle()
	assert.Error(t, err, "dependency cycle detected: D -> C -> B")
}

func TestWith_RootNodesAndUp(t *testing.T) {
	graph := exampleGraph()

	tests := []struct {
		name  string
		nodes []string
		want  []string
	}{
		{
			name:  "whole graph",
			nodes: []string{"A", "B"},
			want:  []string{"A", "B", "C", "D", "E", "F", "G"},
		},
		{
			name:  "only leaves",
			nodes: []string{"F", "G"},
			want:  []string{"F", "G"},
		},
		{
			name:  "simple dependent",
			nodes: []string{"D"},
			want:  []string{"D", "F"},
		},
		{
			name:  "diamond dependents",
			nodes: []string{"B"},
			want:  []string{"B", "C", "D", "E", "F"},
		},
		{
			name:  "partial graph",
			nodes: []string{"A"},
			want:  []string{"A", "C", "D", "F", "G"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mx := sync.Mutex{}
			expected := utils.Set[string]{}
			expected.AddAll("C", "G", "D", "F")
			var visited []string

			gt := newTraversal(func(ctx context.Context, name string, service types.ServiceConfig) (any, error) {
				mx.Lock()
				defer mx.Unlock()
				visited = append(visited, name)
				return nil, nil
			})
			WithRootNodesAndDown(tt.nodes)(gt.Options)
			err := walk(context.TODO(), graph, gt)
			assert.NilError(t, err)
			sort.Strings(visited)
			assert.DeepEqual(t, tt.want, visited)
		})
	}
}

func assertVertexEqual(t *testing.T, a, b vertex) {
	assert.Equal(t, a.key, b.key)
	assert.Equal(t, a.service.Name, b.service.Name)
	for c := range a.children {
		_, ok := b.children[c]
		assert.Check(t, ok, "expected children missing %s", c)
	}
	for p := range a.parents {
		_, ok := b.parents[p]
		assert.Check(t, ok, "expected parent missing %s", p)
	}
}

func exampleGraph() *graph {
	graph := &graph{
		vertices: map[string]*vertex{},
	}

	/** graph topology:
	           A   B
		      / \ / \
		     G   C   E
		          \ /
		           D
		           |
		           F
	*/

	graph.addVertex("A", types.ServiceConfig{Name: "A"})
	graph.addVertex("B", types.ServiceConfig{Name: "B"})
	graph.addVertex("C", types.ServiceConfig{Name: "C"})
	graph.addVertex("D", types.ServiceConfig{Name: "D"})
	graph.addVertex("E", types.ServiceConfig{Name: "E"})
	graph.addVertex("F", types.ServiceConfig{Name: "F"})
	graph.addVertex("G", types.ServiceConfig{Name: "G"})

	graph.addEdge("C", "A")
	graph.addEdge("C", "B")
	graph.addEdge("E", "B")
	graph.addEdge("D", "C")
	graph.addEdge("D", "E")
	graph.addEdge("F", "D")
	graph.addEdge("G", "A")
	return graph
}

func exampleProject() *types.Project {
	return &types.Project{
		Services: types.Services{
			"test1": {
				Name: "test1",
				DependsOn: map[string]types.ServiceDependency{
					"test2": {},
				},
			},
			"test2": {
				Name: "test2",
				DependsOn: map[string]types.ServiceDependency{
					"test3": {},
				},
			},
			"test3": {
				Name: "test3",
			},
		},
	}
}
