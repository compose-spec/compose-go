package compatibility

import "github.com/compose-spec/compose-go/types"

func (c *WhiteList) CheckBuild(service *types.ServiceConfig) {
	if service.Build != nil {
		if !c.supported("services.build") {
			service.Build = nil
			c.error("services.build")
			return
		}
		c.CheckBuildArgs(service.Build)
		c.CheckBuildLabels(service.Build)
		c.CheckBuildCacheFrom(service.Build)
		c.CheckBuildNetwork(service.Build)
		c.CheckBuildTarget(service.Build)
	}
}

func (c *WhiteList) CheckBuildArgs(build *types.BuildConfig) {
	if !c.supported("services.build.args") && len(build.Args) != 0 {
		build.Args = nil
		c.error("services.build.args")
	}
}

func (c *WhiteList) CheckBuildLabels(build *types.BuildConfig) {
	if !c.supported("services.build.labels") && len(build.Labels) != 0 {
		build.Labels = nil
		c.error("services.build.labels")
	}
}

func (c *WhiteList) CheckBuildCacheFrom(build *types.BuildConfig) {
	if !c.supported("services.build.cache_from") && len(build.CacheFrom) != 0 {
		build.CacheFrom = nil
		c.error("services.build.cache_from")
	}
}

func (c *WhiteList) CheckBuildNetwork(build *types.BuildConfig) {
	if !c.supported("services.build.network") && build.Network != "" {
		build.Network = ""
		c.error("services.build.network")
	}
}

func (c *WhiteList) CheckBuildTarget(build *types.BuildConfig) {
	if !c.supported("services.build.target") && build.Target != "" {
		build.Target = ""
		c.error("services.build.target")
	}
}
