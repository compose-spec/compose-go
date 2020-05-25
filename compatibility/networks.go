package compatibility

import "github.com/compose-spec/compose-go/types"

func (c *WhiteList) CheckNetworkConfig(network *types.NetworkConfig) {
	c.CheckNetworkConfigDriver(network)
	c.CheckNetworkConfigDriverOpts(network)
	c.CheckNetworkConfigIpam(network)
	c.CheckNetworkConfigExternal(network)
	c.CheckNetworkConfigInternal(network)
	c.CheckNetworkConfigAttachable(network)
	c.CheckNetworkConfigLabels(network)
}

func (c *WhiteList) CheckNetworkConfigDriver(config *types.NetworkConfig) {
	if !c.supported("_networks.driver") && config.Driver != "" {
		config.Driver = ""
		c.error("_networks.driver")
	}
}

func (c *WhiteList) CheckNetworkConfigDriverOpts(config *types.NetworkConfig) {
	if !c.supported("_networks.driver_opts") && len(config.DriverOpts) != 0 {
		config.DriverOpts = nil
		c.error("_networks.driver_opts")
	}
}

func (c *WhiteList) CheckNetworkConfigIpam(config *types.NetworkConfig) {
	c.CheckNetworkConfigIpamDriver(&config.Ipam)
	if len(config.Ipam.Config) != 0 {
		if !c.supported("_networks.ipam.config") {
			c.error("_networks.ipam.config")
			return
		}
		for _, p := range config.Ipam.Config {
			c.CheckNetworkConfigIpamSubnet(p)
		}
	}
}

func (c *WhiteList) CheckNetworkConfigIpamDriver(config *types.IPAMConfig) {
	if !c.supported("_networks.ipam.driver") && config.Driver != "" {
		config.Driver = ""
		c.error("_networks.ipam.driver")
	}
}

func (c *WhiteList) CheckNetworkConfigIpamSubnet(config *types.IPAMPool) {
	if !c.supported("_networks.ipam.config.subnet") && config.Subnet != "" {
		config.Subnet = ""
		c.error("_networks.ipam.config.subnet")
	}

}

func (c *WhiteList) CheckNetworkConfigExternal(config *types.NetworkConfig) {
	if !c.supported("_networks.external") && config.External.External {
		config.External.External = false
		c.error("_networks.external")
	}
}

func (c *WhiteList) CheckNetworkConfigInternal(config *types.NetworkConfig) {
	if !c.supported("_networks.internal") && config.Internal {
		config.Internal = false
		c.error("_networks.internal")
	}
}

func (c *WhiteList) CheckNetworkConfigAttachable(config *types.NetworkConfig) {
	if !c.supported("_networks.attachable") && config.Attachable {
		config.Attachable = false
		c.error("_networks.attachable")
	}
}

func (c *WhiteList) CheckNetworkConfigLabels(config *types.NetworkConfig) {
	if !c.supported("_networks.labels") && len(config.Labels) != 0 {
		config.Labels = nil
		c.error("_networks.labels")
	}
}
