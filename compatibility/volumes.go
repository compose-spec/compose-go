package compatibility

import "github.com/compose-spec/compose-go/types"

func (c *WhiteList) CheckVolumeConfig(config *types.VolumeConfig) {
	c.CheckVolumeConfigDriver(config)
	c.CheckVolumeConfigDriverOpts(config)
	c.CheckVolumeConfigExternal(config)
	c.CheckVolumeConfigLabels(config)
}

func (c *WhiteList) CheckVolumeConfigDriver(config *types.VolumeConfig) {
	if !c.supported("volumes.driver") && config.Driver != "" {
		config.Driver = ""
		c.error("volumes.driver")
	}
}

func (c *WhiteList) CheckVolumeConfigDriverOpts(config *types.VolumeConfig) {
	if !c.supported("volumes.driver_opts") && len(config.DriverOpts) != 0 {
		config.DriverOpts = nil
		c.error("volumes.driver_opts")
	}
}

func (c *WhiteList) CheckVolumeConfigExternal(config *types.VolumeConfig) {
	if !c.supported("volumes.external") && config.External.External {
		config.External.External = false
		c.error("volumes.external")
	}
}

func (c *WhiteList) CheckVolumeConfigLabels(config *types.VolumeConfig) {
	if !c.supported("volumes.labels") && len(config.Labels) != 0 {
		config.Labels = nil
		c.error("volumes.labels")
	}
}
