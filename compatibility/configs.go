package compatibility

import (
	"fmt"

	"github.com/compose-spec/compose-go/types"
)

func (c *WhiteList) CheckConfigsConfig(config *types.ConfigObjConfig) {
	ref := types.FileObjectConfig(*config)
	c.CheckFileObjectConfig("configs", &ref)
}

func (c *WhiteList) CheckSecretsConfig(config *types.SecretConfig) {
	ref := types.FileObjectConfig(*config)
	c.CheckFileObjectConfig("secrets", &ref)
}

func (c *WhiteList) CheckFileObjectConfig(s string, config *types.FileObjectConfig) {
	c.CheckFileObjectConfigDriver(s, config)
	c.CheckFileObjectConfigDriverOpts(s, config)
	c.CheckFileObjectConfigExternal(s, config)
	c.CheckFileObjectConfigFile(s, config)
	c.CheckFileObjectConfigLabels(s, config)
	c.CheckFileObjectConfigTemplateDriver(s, config)
}

func (c *WhiteList) CheckFileObjectConfigFile(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.file", s)
	if !c.supported(k) && config.File != "" {
		config.File = ""
		c.error(k)
	}
}

func (c *WhiteList) CheckFileObjectConfigExternal(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.external", s)
	if !c.supported(k) && config.External.External {
		config.External.External = false
		c.error(k)
	}
}

func (c *WhiteList) CheckFileObjectConfigLabels(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.labels", s)
	if !c.supported(k) && len(config.Labels) != 0 {
		config.Labels = nil
		c.error(k)
	}
}

func (c *WhiteList) CheckFileObjectConfigDriver(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.driver", s)
	if !c.supported(k) && config.Driver != "" {
		config.Driver = ""
		c.error(k)
	}
}

func (c *WhiteList) CheckFileObjectConfigDriverOpts(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.driver_opts", s)
	if !c.supported(k) && len(config.DriverOpts) != 0 {
		config.DriverOpts = nil
		c.error(k)
	}
}

func (c *WhiteList) CheckFileObjectConfigTemplateDriver(s string, config *types.FileObjectConfig) {
	k := fmt.Sprintf("%s.template_driver", s)
	if !c.supported(k) && config.TemplateDriver != "" {
		config.TemplateDriver = ""
		c.error(k)
	}
}
