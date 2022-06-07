package loader

import (
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestNoCycle(t *testing.T) {
	var serviceConfigs = []types.ServiceConfig{
		{
			Name: "test1",
			DependsOn: map[string]types.ServiceDependency{
				"test2": {},
			},
		},
		{
			Name: "test2",
			DependsOn: map[string]types.ServiceDependency{
				"test3": {},
			},
		},
		{
			Name: "test3",
		},
	}

	g := NewGraph(serviceConfigs)
	hasCycle, err := g.HasCycles()

	assert.Equal(t, hasCycle, false)
	assert.NilError(t, err)
}

func TestContainCycle(t *testing.T) {
	var serviceConfigs = []types.ServiceConfig{
		{
			Name: "test1",
			DependsOn: map[string]types.ServiceDependency{
				"test2": {},
			},
		},
		{
			Name: "test2",
			DependsOn: map[string]types.ServiceDependency{
				"test3": {},
			},
		},
		{
			Name: "test3",
			DependsOn: map[string]types.ServiceDependency{
				"test2": {},
			},
		},
	}

	g := NewGraph(serviceConfigs)
	hasCycle, err := g.HasCycles()

	assert.Equal(t, hasCycle, true)
	assert.ErrorContains(t, err, "cycle found")
}
