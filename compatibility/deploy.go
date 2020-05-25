package compatibility

import (
	"fmt"

	"github.com/compose-spec/compose-go/types"
)

func (c *WhiteList) CheckDeploy(service *types.ServiceConfig) {
	if service.Deploy != nil {
		if !c.supported("deploy") {
			service.Deploy = nil
			c.error("deploy")
			return
		}
		c.CheckDeployEndpointMode(service.Deploy)
		c.CheckDeployLabels(service.Deploy)
		c.CheckDeployMode(service.Deploy)
		c.CheckDeployPlacement(service.Deploy)
		c.CheckDeployReplicas(service.Deploy)
		c.CheckDeployResources(ResourceLimits, service.Deploy)
		c.CheckDeployResources(ResourceReservations, service.Deploy)
		c.CheckDeployRestartPolicy(service.Deploy)
		c.CheckDeployRollbackConfig(service.Deploy)
		c.CheckDeployUpdateConfig(service.Deploy)
	}
}

func (c *WhiteList) CheckDeployMode(config *types.DeployConfig) {
	if !c.supported("services.deploy.mode") && config.Mode != "" {
		config.Mode = ""
		c.error("services.deploy.mode")
	}
}
func (c *WhiteList) CheckDeployReplicas(config *types.DeployConfig) {
	if !c.supported("services.deploy.replicas") && config.Replicas != nil {
		config.Replicas = nil
		c.error("services.deploy.replicas")
	}
}
func (c *WhiteList) CheckDeployLabels(config *types.DeployConfig) {
	if !c.supported("services.deploy.labels") && len(config.Labels) != 0 {
		config.Labels = nil
		c.error("services.deploy.labels")
	}
}

const (
	UpdateConfigUpdate   = "update_config"
	UpdateConfigRollback = "rolback_config"
)

func (c *WhiteList) CheckDeployUpdateConfig(config *types.DeployConfig) {
	if config.UpdateConfig != nil {
		if !c.supported("services.deploy.update_config") {
			config.UpdateConfig = nil
			c.error("services.deploy.update_config")
			return
		}
		c.CheckUpdateConfig(UpdateConfigUpdate, config)
	}
}

func (c *WhiteList) CheckDeployRollbackConfig(config *types.DeployConfig) {
	if config.RollbackConfig != nil {
		if !c.supported("services.deploy.rollback_config") {
			config.RollbackConfig = nil
			c.error("services.deploy.rollback_config")
			return
		}
		c.CheckUpdateConfig(UpdateConfigRollback, config)
	}
}

func (c *WhiteList) CheckUpdateConfig(update string, config *types.DeployConfig) {
	c.CheckUpdateConfigDelay(update, config.UpdateConfig)
	c.CheckUpdateConfigFailureAction(update, config.UpdateConfig)
	c.CheckUpdateConfigMaxFailureRatio(update, config.UpdateConfig)
	c.CheckUpdateConfigMonitor(update, config.UpdateConfig)
	c.CheckUpdateConfigOrder(update, config.UpdateConfig)
	c.CheckUpdateConfigParallelism(update, config.UpdateConfig)
}

func (c *WhiteList) CheckUpdateConfigParallelism(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.parallelism", s)
	if !c.supported(k) && config.Parallelism != nil {
		config.Parallelism = nil
		c.error(k)
	}
}
func (c *WhiteList) CheckUpdateConfigDelay(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.delay", s)
	if !c.supported(k) && config.Delay != 0 {
		config.Delay = 0
		c.error(k)
	}
}
func (c *WhiteList) CheckUpdateConfigFailureAction(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.failure_action", s)
	if !c.supported(k) && config.FailureAction != "" {
		config.FailureAction = ""
		c.error(k)
	}
}
func (c *WhiteList) CheckUpdateConfigMonitor(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.monitor", s)
	if !c.supported(k) && config.Monitor != 0 {
		config.Monitor = 0
		c.error(k)
	}
}
func (c *WhiteList) CheckUpdateConfigMaxFailureRatio(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.max_failure_ratio", s)
	if !c.supported(k) && config.MaxFailureRatio != 0 {
		config.MaxFailureRatio = 0
		c.error(k)
	}
}
func (c *WhiteList) CheckUpdateConfigOrder(s string, config *types.UpdateConfig) {
	k := fmt.Sprintf("services.deploy.%s.order", s)
	if !c.supported(k) && config.Order != "" {
		config.Order = ""
		c.error(k)
	}
}

const (
	ResourceLimits       = "limits"
	ResourceReservations = "reservations"
)

func (c *WhiteList) CheckDeployResources(s string, config *types.DeployConfig) {
	c.CheckDeployResourcesNanoCPUs(s, config.Resources.Limits)
	c.CheckDeployResourcesMemoryBytes(s, config.Resources.Limits)
	c.CheckDeployResourcesGenericResources(s, config.Resources.Limits)
}

func (c *WhiteList) CheckDeployResourcesNanoCPUs(s string, r *types.Resource) {
	k := fmt.Sprintf("services.deploy.resources.%s.cpus", s)
	if !c.supported(k) && r.NanoCPUs != "" {
		r.NanoCPUs = ""
		c.error(k)
	}
}
func (c *WhiteList) CheckDeployResourcesMemoryBytes(s string, r *types.Resource) {
	k := fmt.Sprintf("services.deploy.resources.%s.memory", s)
	if !c.supported(k) && r.MemoryBytes != 0 {
		r.MemoryBytes = 0
		c.error(k)
	}
}
func (c *WhiteList) CheckDeployResourcesGenericResources(s string, r *types.Resource) {
	k := fmt.Sprintf("services.deploy.resources.%s.generic_resources", s)
	if !c.supported(k) && len(r.GenericResources) != 0 {
		r.GenericResources = nil
		c.error(k)
	}
}

func (c *WhiteList) CheckDeployRestartPolicy(config *types.DeployConfig) {
	if config.RestartPolicy != nil {
		if !c.supported("services.deploy.restart_policy") {
			config.RestartPolicy = nil
			c.error("services.deploy.restart_policy")
			return
		}
		c.CheckRestartPolicyCondition(config.RestartPolicy)
		c.CheckRestartPolicyDelay(config.RestartPolicy)
		c.CheckRestartPolicyMaxAttempts(config.RestartPolicy)
		c.CheckRestartPolicyWindow(config.RestartPolicy)
	}
}

func (c *WhiteList) CheckRestartPolicyCondition(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.condition") && p.Condition != "" {
		p.Condition = ""
		c.error("services.deploy.restart_policy.condition")
	}
}
func (c *WhiteList) CheckRestartPolicyDelay(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.delay") && p.Delay != nil {
		p.Delay = nil
		c.error("services.deploy.restart_policy.delay")
	}
}
func (c *WhiteList) CheckRestartPolicyMaxAttempts(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.max_attempts") && p.MaxAttempts != nil {
		p.MaxAttempts = nil
		c.error("services.deploy.restart_policy.max_attempts")
	}
}
func (c *WhiteList) CheckRestartPolicyWindow(p *types.RestartPolicy) {
	if !c.supported("services.deploy.restart_policy.window") && p.Window != nil {
		p.Window = nil
		c.error("services.deploy.restart_policy.window")
	}
}

func (c *WhiteList) CheckDeployPlacement(config *types.DeployConfig) {
	c.CheckPlacementConstraints(&config.Placement)
	c.CheckPlacementMaxReplicas(&config.Placement)
	c.CheckPlacementPreferences(&config.Placement)
}

func (c *WhiteList) CheckPlacementConstraints(p *types.Placement) {
	if !c.supported("services.deploy.placement.constraints") && len(p.Constraints) != 0 {
		p.Constraints = nil
		c.error("services.deploy.restart_policy.constraints")
	}
}

func (c *WhiteList) CheckPlacementPreferences(p *types.Placement) {
	if !c.supported("services.deploy.placement.preferences") && p.Preferences != nil {
		p.Preferences = nil
		c.error("services.deploy.restart_policy.preferences")
	}
}

func (c *WhiteList) CheckPlacementMaxReplicas(p *types.Placement) {
	if !c.supported("services.deploy.placement.max_replicas_per_node") && p.MaxReplicas != 0 {
		p.MaxReplicas = 0
		c.error("services.deploy.restart_policy.max_replicas_per_node")
	}
}

func (c *WhiteList) CheckDeployEndpointMode(config *types.DeployConfig) {
	if !c.supported("services.deploy.endpoint_mode") && config.EndpointMode != "" {
		config.EndpointMode = ""
		c.error("services.deploy.endpoint_mode")
	}
}
