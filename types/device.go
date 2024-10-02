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

package types

import (
	"fmt"
	"strconv"
	"strings"
)

type DeviceRequest struct {
	Capabilities []string    `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Driver       string      `yaml:"driver,omitempty" json:"driver,omitempty"`
	Count        DeviceCount `yaml:"count,omitempty" json:"count,omitempty"`
	IDs          []string    `yaml:"device_ids,omitempty" json:"device_ids,omitempty"`
	Options      Mapping     `yaml:"options,omitempty" json:"options,omitempty"`
}

type DeviceCount int64

func (c *DeviceCount) DecodeMapstructure(value interface{}) error {
	switch v := value.(type) {
	case int:
		*c = DeviceCount(v)
	case string:
		if strings.ToLower(v) == "all" {
			*c = -1
			return nil
		}
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid value %q, the only value allowed is 'all' or a number", v)
		}
		*c = DeviceCount(i)
	default:
		return fmt.Errorf("invalid type %T for device count", v)
	}
	return nil
}

func (d *DeviceRequest) DecodeMapstructure(value interface{}) error {
	v, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid device request type %T", value)
	}
	if _, okCaps := v["capabilities"]; !okCaps {
		return fmt.Errorf(`"capabilities" attribute is mandatory for device request definition`)
	}
	if _, okCount := v["count"]; okCount {
		if _, okDeviceIds := v["device_ids"]; okDeviceIds {
			return fmt.Errorf(`invalid "count" and "device_ids" are attributes are exclusive`)
		}
	}
	d.Count = DeviceCount(-1)

	capabilities := v["capabilities"]
	caps := StringList{}
	if err := caps.DecodeMapstructure(capabilities); err != nil {
		return err
	}
	d.Capabilities = caps
	if driver, ok := v["driver"]; ok {
		if val, ok := driver.(string); ok {
			d.Driver = val
		} else {
			return fmt.Errorf("invalid type for driver value: %T", driver)
		}
	}
	if count, ok := v["count"]; ok {
		if err := d.Count.DecodeMapstructure(count); err != nil {
			return err
		}
	}
	if deviceIDs, ok := v["device_ids"]; ok {
		ids := StringList{}
		if err := ids.DecodeMapstructure(deviceIDs); err != nil {
			return err
		}
		d.IDs = ids
		d.Count = DeviceCount(len(ids))
	}

	d.Options = Mapping{}
	if options, ok := v["options"].(map[string]any); ok {
		for k, v := range options {
			d.Options[k] = fmt.Sprint(v)
		}
	}
	return nil

}
