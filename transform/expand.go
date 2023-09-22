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
	"fmt"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/tree"
	"github.com/compose-spec/compose-go/types"
	"github.com/docker/go-units"
	"github.com/mattn/go-shellwords"
	"github.com/pkg/errors"
)

type transformFunc func(data interface{}) (interface{}, error)

var (
	// TODO(ndeloof) Labels, Mapping, MappingWithEqual and HostsList could implement yaml.Unmarhsaller
	transformLabels           = transformMappingOrListFunc("=", false)
	transformHostList         = transformMappingOrListFunc(":", false)
	transformMapping          = transformMappingOrListFunc("=", false)
	transformMappingWithEqual = transformMappingOrListFunc("=", true)
)

// transformers list the places compose spec allows a short syntax we have to expand
var transformers = map[tree.Path]transformFunc{
	"services.*.annotations":                                   transformMapping,
	"services.*.blkio_config.device_read_bps.*.rate":           transformSize,
	"services.*.blkio_config.device_read_iops.*.rate":          transformSize,
	"services.*.blkio_config.device_write_bps.*.rate":          transformSize,
	"services.*.blkio_config.device_write_iops.*.rate":         transformSize,
	"services.*.build":                                         transformBuildConfig,
	"services.*.build.additional_contexts":                     transformMapping,
	"services.*.build.args":                                    transformMappingWithEqual,
	"services.*.build.cache_from":                              transformStringList,
	"services.*.build.cache_to":                                transformStringList,
	"services.*.build.extra_hosts":                             transformHostList,
	"services.*.build.labels":                                  transformLabels,
	"services.*.build.tags":                                    transformStringList,
	"services.*.build.platforms":                               transformStringList,
	"services.*.build.secrets.*":                               transformFileReferenceConfig,
	"services.*.build.ssh":                                     transformSSHConfig,
	"services.*.command":                                       transformShellCommand,
	"services.*.configs.*":                                     transformFileReferenceConfig,
	"services.*.depends_on":                                    transformDependsOnConfig,
	"services.*.deploy.labels":                                 transformLabels,
	"services.*.deploy.resources.limits.memory":                transformSize,
	"services.*.deploy.resources.reservations.memory":          transformSize,
	"services.*.deploy.resources.reservations.devices.*.count": transformServiceDeviceRequestCount,
	"services.*.dns":                                           transformStringList,
	"services.*.dns_search":                                    transformStringList,
	"services.*.entrypoint":                                    transformShellCommand,
	"services.*.env_file":                                      transformStringList,
	"services.*.environment":                                   transformMappingWithEqual,
	"services.*.expose":                                        transformStringOrNumberList,
	"services.*.extends":                                       transformExtends,
	"services.*.extra_hosts":                                   transformHostList,
	"services.*.healthcheck.test":                              transformHealthCheckTest,
	"services.*.labels":                                        transformLabels,
	"services.*.mem_limit":                                     transformSize,
	"services.*.mem_reservation":                               transformSize,
	"services.*.mem_swap_limit":                                transformSize,
	"services.*.mem_swapiness":                                 transformSize,
	"services.*.networks":                                      transformServiceNetworks,
	"services.*.ports":                                         transformServicePorts,
	"services.*.secrets.*":                                     transformFileReferenceConfig,
	"services.*.shm_size":                                      transformSize,
	"services.*.sysctls":                                       transformMapping,
	"services.*.tmpfs":                                         transformStringList,
	"services.*.ulimits.*":                                     transformUlimits,
	"services.*.volumes.*":                                     transformServiceVolume,
	"services.*.volumes.*.tmpfs.size":                          transformSize,
	"include.*":                                                transformIncludeConfig,
	"include.*.path":                                           transformStringList,
	"include.*.env_file":                                       transformStringList,
	"configs.*.external":                                       transformExternal,
	"configs.*.labels":                                         transformLabels,
	"networks.*.driver_opts.*":                                 transformDriverOpt,
	"networks.*.external":                                      transformExternal,
	"networks.*.labels":                                        transformLabels,
	"volumes.*.driver_opts.*":                                  transformDriverOpt,
	"volumes.*.external":                                       transformExternal,
	"volumes.*.labels":                                         transformLabels,
	"secrets.*.external":                                       transformExternal,
	"secrets.*.labels":                                         transformLabels,
}

func ExpandShortSyntax(composefile map[string]interface{}) (map[string]interface{}, error) {
	m, err := transform(composefile, tree.NewPath())
	if err != nil {
		return nil, err
	}
	return m.(map[string]interface{}), nil
}

func transform(e interface{}, p tree.Path) (interface{}, error) {
	for pattern, transformer := range transformers {
		if p.Matches(pattern) {
			t, err := transformer(e)
			if err != nil {
				return nil, err
			}
			e = t
		}
	}
	switch value := e.(type) {
	case map[string]interface{}:
		for k, v := range value {
			t, err := transform(v, p.Next(k))
			if err != nil {
				return nil, err
			}
			value[k] = t
		}
		return value, nil
	case []interface{}:
		for i, e := range value {
			t, err := transform(e, p.Next("[]"))
			if err != nil {
				return nil, err
			}
			value[i] = t
		}
		return value, nil
	default:
		return e, nil
	}
}

func transformServiceVolume(data interface{}) (interface{}, error) {
	if value, ok := data.(string); ok {
		volume, err := parseVolume(value)
		if err != nil {
			return nil, err
		}
		vol := map[string]interface{}{
			"type":      volume.Type,
			"source":    volume.Source,
			"target":    volume.Target,
			"read_only": volume.ReadOnly,
		}
		if volume.Volume != nil {
			vol["volume"] = map[string]interface{}{
				"nocopy": volume.Volume.NoCopy,
			}
		}
		if volume.Bind != nil {
			vol["bind"] = map[string]interface{}{
				"create_host_path": volume.Bind.CreateHostPath,
				"propagation":      volume.Bind.Propagation,
				"selinux":          volume.Bind.SELinux,
			}
		}
		return omitEmpty(vol), nil
	}
	return data, nil
}

// transformServicePorts process the list instead of individual ports as a port-range definition will result in multiple
// items, so we flatten this into a single sequence
func transformServicePorts(data interface{}) (interface{}, error) {
	switch entries := data.(type) {
	case []interface{}:
		var ports []interface{}
		for _, entry := range entries {
			switch value := entry.(type) {
			case int, string:
				parsed, err := types.ParsePortConfig(fmt.Sprint(value))
				if err != nil {
					return data, err
				}
				for _, v := range parsed {
					ports = append(ports, map[string]interface{}{
						"mode":      v.Mode,
						"host_ip":   v.HostIP,
						"target":    int(v.Target),
						"published": v.Published,
						"protocol":  v.Protocol,
					})
				}
			case map[string]interface{}:
				published := value["published"]
				if v, ok := published.(int); ok {
					value["published"] = strconv.Itoa(v)
				}
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

func transformServiceNetworks(data interface{}) (interface{}, error) {
	if list, ok := data.([]interface{}); ok {
		mapValue := map[interface{}]interface{}{}
		for _, name := range list {
			mapValue[name] = nil
		}
		return mapValue, nil
	}
	return data, nil
}

func transformExternal(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case bool:
		return map[string]interface{}{"external": value}, nil
	case map[string]interface{}:
		return map[string]interface{}{"external": true, "name": value["name"]}, nil
	default:
		return data, errors.Errorf("invalid type %T for external", value)
	}
}

func transformHealthCheckTest(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return append([]string{"CMD-SHELL"}, value), nil
	case []interface{}:
		return value, nil
	default:
		return value, errors.Errorf("invalid type %T for healthcheck.test", value)
	}
}

func transformShellCommand(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		args, err := shellwords.Parse(value)
		res := make([]interface{}, len(args))
		for i, arg := range args {
			res[i] = arg
		}
		return res, err
	case []interface{}:
		return value, nil
	default:
		// ShellCommand do NOT have omitempty tag, to distinguish unset vs empty
		if data == nil {
			return nil, nil
		}
		return data, errors.Errorf("invalid type %T for shell command", value)
	}
}

func transformDependsOnConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case []interface{}:
		transformed := map[string]interface{}{}
		for _, serviceIntf := range value {
			service, ok := serviceIntf.(string)
			if !ok {
				return data, errors.Errorf("invalid type %T for service depends_on element, expected string", value)
			}
			transformed[service] = map[string]interface{}{
				"condition": types.ServiceConditionStarted,
				"required":  true,
			}
		}
		return transformed, nil
	case map[string]interface{}:
		transformed := map[string]interface{}{}
		for service, val := range value {
			dependsConfigIntf, ok := val.(map[string]interface{})
			if !ok {
				return data, errors.Errorf("invalid type %T for service depends_on element", value)
			}
			if _, ok := dependsConfigIntf["required"]; !ok {
				dependsConfigIntf["required"] = true
			}
			transformed[service] = dependsConfigIntf
		}
		return transformed, nil
	default:
		return data, errors.Errorf("invalid type %T for service depends_on", value)
	}
}

func transformFileReferenceConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return map[string]interface{}{"source": value}, nil
	case map[string]interface{}:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for secret", value)
	}
}

func transformBuildConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return map[string]interface{}{"context": value}, nil
	case map[string]interface{}:
		return data, nil
	default:
		return data, errors.Errorf("invalid type %T for service build", value)
	}
}

func transformExtends(data interface{}) (interface{}, error) {
	switch data.(type) {
	case string:
		return map[string]interface{}{"service": data}, nil
	case map[string]interface{}:
		return data, nil
	default:
		return data, errors.Errorf("invalid type %T for extends", data)
	}
}

func transformStringList(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return []string{value}, nil
	case []interface{}:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for string list", value)
	}
}

// TODO(ndeloof) StringOrNumberList could implement yaml.Unmarhsaler
func transformStringOrNumberList(data interface{}) (interface{}, error) {
	list := data.([]interface{})
	result := make([]string, len(list))
	for i, item := range list {
		result[i] = fmt.Sprint(item)
	}
	return result, nil
}

// TODO(ndeloof) UlimitsConfig could implement yaml.Unmarhsaler
func transformUlimits(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case int:
		return map[string]interface{}{"single": value}, nil
	case map[string]interface{}:
		return data, nil
	default:
		return data, errors.Errorf("invalid type %T for ulimits", value)
	}
}

// TODO(ndeloof) UnitBytes could implement yaml.Unmarhsaler
func transformSize(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case int:
		return value, nil
	case int64, types.UnitBytes:
		return value, nil
	case string:
		return units.RAMInBytes(value)
	default:
		return value, errors.Errorf("invalid type for size %T", value)
	}
}

// TODO(ndeloof) create a DriverOpts type and implement yaml.Unmarshaler
func transformDriverOpt(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case int:
		return strconv.Itoa(value), nil
	case string:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for driver_opts value", value)
	}
}

func transformServiceDeviceRequestCount(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case int:
		return value, nil
	case string:
		if strings.ToLower(value) == "all" {
			return -1, nil
		}
		i, err := strconv.Atoi(value)
		if err == nil {
			return i, nil
		}
		return data, errors.Errorf("invalid string value for 'count' (the only value allowed is 'all' or a number)")
	default:
		return data, errors.Errorf("invalid type %T for device count", value)
	}
}

// TODO(ndeloof) SSHConfig could implement yaml.Unmarshal
func transformSSHConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case map[string]interface{}:
		var result []interface{}
		for key, val := range value {
			if val == nil {
				val = ""
			}
			result = append(result, map[string]interface{}{"id": key, "path": val.(string)})
		}
		return result, nil
	case []interface{}:
		var result []interface{}
		for _, v := range value {
			key, val := transformValueToMapEntry(v.(string), "=", false)
			result = append(result, map[string]interface{}{"id": key, "path": val.(string)})
		}
		return result, nil
	case string:
		return ParseShortSSHSyntax(value)
	}
	return nil, errors.Errorf("expected a sting, map or a list, got %T: %#v", data, data)
}

// ParseShortSSHSyntax parse short syntax for SSH authentications
func ParseShortSSHSyntax(value string) ([]types.SSHKey, error) {
	if value == "" {
		value = "default"
	}
	key, val := transformValueToMapEntry(value, "=", false)
	result := []types.SSHKey{{ID: key, Path: val.(string)}}
	return result, nil
}

func transformValueToMapEntry(value string, separator string, allowNil bool) (string, interface{}) {
	parts := strings.SplitN(value, separator, 2)
	key := parts[0]
	switch {
	case len(parts) == 1 && allowNil:
		return key, nil
	case len(parts) == 1 && !allowNil:
		return key, ""
	default:
		return key, parts[1]
	}
}

func transformIncludeConfig(data interface{}) (interface{}, error) {
	switch value := data.(type) {
	case string:
		return map[string]interface{}{"path": value}, nil
	case map[string]interface{}:
		return value, nil
	default:
		return data, errors.Errorf("invalid type %T for `include` configuration", value)
	}
}

func transformMappingOrListFunc(sep string, allowNil bool) transformFunc {
	return func(data interface{}) (interface{}, error) {
		switch value := data.(type) {
		case map[string]interface{}:
			result := make(map[string]interface{})
			for key, v := range value {
				result[key] = toString(v, allowNil)
			}
			return result, nil
		case []interface{}:
			result := make(map[string]interface{})
			for _, v := range value {
				key, val := transformValueToMapEntry(v.(string), sep, allowNil)
				result[key] = val
			}
			return result, nil
		}
		return nil, errors.Errorf("expected a map or a list, got %T: %#v", data, data)
	}
}

func toString(value interface{}, allowNil bool) interface{} {
	switch {
	case value != nil:
		return fmt.Sprint(value)
	case allowNil:
		return nil
	default:
		return ""
	}
}

func omitEmpty(m map[string]interface{}) interface{} {
	for k, v := range m {
		switch e := v.(type) {
		case string:
			if e == "" {
				delete(m, k)
			}
		case int, int32, int64:
			if e == 0 {
				delete(m, k)
			}
		case map[string]interface{}:
			m[k] = omitEmpty(e)
		}
	}
	return m
}
