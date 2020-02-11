package types

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_WithServices(t *testing.T) {
	c := Config{
		Services: append(Services{},
			ServiceConfig{
				Name:      "service_1",
				DependsOn: []string{"service_3"},
			}, ServiceConfig{
				Name: "service_2",
			}, ServiceConfig{
				Name:  "service_3",
				Links: []string{"service_2"},
			}),
	}
	order := []string{}
	fn := func(service ServiceConfig) error {
		order = append(order, service.Name)
		return nil
	}

	err := c.WithServices(nil, fn)
	assert.NilError(t, err)
	assert.DeepEqual(t, order, []string{"service_2", "service_3", "service_1"})
}
