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

package compatibility

import "github.com/compose-spec/compose-go/types"

func (c *AllowList) CheckNetworkConfig(network *types.NetworkConfig) {
	c.CheckNetworkConfigDriver(network)
	c.CheckNetworkConfigDriverOpts(network)
	c.CheckNetworkConfigIpam(network)
	c.CheckNetworkConfigExternal(network)
	c.CheckNetworkConfigInternal(network)
	c.CheckNetworkConfigAttachable(network)
	c.CheckNetworkConfigLabels(network)
}

func (c *AllowList) CheckNetworkConfigDriver(config *types.NetworkConfig) {
	if !c.supported("networks.driver") && config.Driver != "" {
		config.Driver = ""
		c.Error("networks.driver")
	}
}

func (c *AllowList) CheckNetworkConfigDriverOpts(config *types.NetworkConfig) {
	if !c.supported("networks.driver_opts") && len(config.DriverOpts) != 0 {
		config.DriverOpts = nil
		c.Error("networks.driver_opts")
	}
}

func (c *AllowList) CheckNetworkConfigIpam(config *types.NetworkConfig) {
	c.CheckNetworkConfigIpamDriver(&config.Ipam)
	if len(config.Ipam.Config) != 0 {
		if !c.supported("networks.ipam.config") {
			c.Error("networks.ipam.config")
			return
		}
		for _, p := range config.Ipam.Config {
			c.CheckNetworkConfigIpamSubnet(p)
		}
	}
}

func (c *AllowList) CheckNetworkConfigIpamDriver(config *types.IPAMConfig) {
	if !c.supported("networks.ipam.driver") && config.Driver != "" {
		config.Driver = ""
		c.Error("networks.ipam.driver")
	}
}

func (c *AllowList) CheckNetworkConfigIpamSubnet(config *types.IPAMPool) {
	if !c.supported("networks.ipam.config.subnet") && config.Subnet != "" {
		config.Subnet = ""
		c.Error("networks.ipam.config.subnet")
	}

}

func (c *AllowList) CheckNetworkConfigExternal(config *types.NetworkConfig) {
	if !c.supported("networks.external") && config.External.External {
		config.External.External = false
		c.Error("networks.external")
	}
}

func (c *AllowList) CheckNetworkConfigInternal(config *types.NetworkConfig) {
	if !c.supported("networks.internal") && config.Internal {
		config.Internal = false
		c.Error("networks.internal")
	}
}

func (c *AllowList) CheckNetworkConfigAttachable(config *types.NetworkConfig) {
	if !c.supported("networks.attachable") && config.Attachable {
		config.Attachable = false
		c.Error("networks.attachable")
	}
}

func (c *AllowList) CheckNetworkConfigLabels(config *types.NetworkConfig) {
	if !c.supported("networks.labels") && len(config.Labels) != 0 {
		config.Labels = nil
		c.Error("networks.labels")
	}
}
