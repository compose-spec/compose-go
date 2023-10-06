package transform

import (
	"fmt"

	"github.com/compose-spec/compose-go/tree"
	"github.com/compose-spec/compose-go/types"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

func trasformPorts(data interface{}, p tree.Path) (interface{}, error) {
	switch entries := data.(type) {
	case []interface{}:
		// We process the list instead of individual items here.
		// The reason is that one entry might be mapped to multiple ServicePortConfig.
		// Therefore we take an input of a list and return an output of a list.
		var ports []interface{}
		for _, entry := range entries {
			switch value := entry.(type) {
			case int:
				parsed, err := types.ParsePortConfig(fmt.Sprint(value))
				if err != nil {
					return data, err
				}
				for _, v := range parsed {
					m := map[string]interface{}{}
					err := mapstructure.Decode(v, &m)
					if err != nil {
						return nil, err
					}
					ports = append(ports, m)
				}
			case string:
				parsed, err := types.ParsePortConfig(value)
				if err != nil {
					return data, err
				}
				for _, v := range parsed {
					m := map[string]interface{}{}
					err := mapstructure.Decode(v, &m)
					if err != nil {
						return nil, err
					}
					ports = append(ports, m)
				}
			case map[string]interface{}:
				ports = append(ports, value)
			default:
				return data, errors.Errorf("invalid type %T for port", value)
			}
		}
		return ports, nil
	default:
		return data, errors.Errorf("invalid type %T for port", entries)
	}
}
