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

import (
	"fmt"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/types"
	"github.com/pkg/errors"
)

// WhiteList implement Checker interface by rejecting all attributes but those listed in a whitelist.
type WhiteList struct {
	Supported []string
	errors    []error
}

func (c *WhiteList) Check(project *types.Project) {
	for i, service := range project.Services {
		c.CheckService(&service)
		project.Services[i] = service
	}
}

func (c *WhiteList) Errors() []error {
	return c.errors
}

func (c *WhiteList) CheckService(service *types.ServiceConfig) {
	c.CheckBuild(service)
	c.CheckCapAdd(service)
	c.CheckCapDrop(service)
	c.CheckCgroupParent(service)
	c.CheckCPUQuota(service)
	c.CheckCPUSet(service)
	c.CheckCPUShares(service)
	c.CheckCommand(service)
	c.CheckConfigs(service)
	c.CheckContainerName(service)
	c.CheckCredentialSpec(service)
	c.CheckDependsOn(service)
	c.CheckDeploy(service)
	c.CheckDevices(service)
	c.CheckDNS(service)
	c.CheckDNSOpts(service)
	c.CheckDNSSearch(service)
	c.CheckDomainName(service)
	c.CheckEntrypoint(service)
	c.CheckEnvironment(service)
	c.CheckEnvFile(service)
	c.CheckExpose(service)
	c.CheckExtends(service)
	c.CheckExternalLinks(service)
	c.CheckExtraHosts(service)
	c.CheckGroupAdd(service)
	c.CheckHostname(service)
	c.CheckHealthCheck(service)
	c.CheckInit(service)
	c.CheckIpc(service)
	c.CheckIsolation(service)
	c.CheckLabels(service)
	c.CheckLinks(service)
	c.CheckLogging(service)
	c.CheckMemLimit(service)
	c.CheckMemReservation(service)
	c.CheckMemSwapLimit(service)
	c.CheckMemSwappiness(service)
	c.CheckMacAddress(service)
	c.CheckNet(service)
	c.CheckNetworkMode(service)
	c.CheckNetworks(service)
	c.CheckOomKillDisable(service)
	c.CheckOomScoreAdj(service)
	c.CheckPid(service)
	c.CheckPorts(service)
	c.CheckPrivileged(service)
	c.CheckReadOnly(service)
	c.CheckRestart(service)
	c.CheckSecrets(service)
	c.CheckSecurityOpt(service)
	c.CheckShmSize(service)
	c.CheckStdinOpen(service)
	c.CheckStopGracePeriod(service)
	c.CheckStopSignal(service)
	c.CheckSysctls(service)
	c.CheckTmpfs(service)
	c.CheckTty(service)
	c.CheckUlimits(service)
	c.CheckUser(service)
	c.CheckUserNSMode(service)
	c.CheckUts(service)
	c.CheckVolumeDriver(service)
	c.CheckVolumes(service)
	c.CheckVolumesFrom(service)
	c.CheckWorkingDir(service)
}

func (c *WhiteList) CheckBuild(service *types.ServiceConfig) {
	if !c.supported("build") && service.Build != nil {
		service.Build = nil
		c.error("build")
	}
}

func (c *WhiteList) CheckCapAdd(service *types.ServiceConfig) {
	if !c.supported("cap_add") && len(service.CapAdd) != 0 {
		service.CapAdd = nil
		c.error("cap_add")
	}
}

func (c *WhiteList) CheckCapDrop(service *types.ServiceConfig) {
	if !c.supported("cap_drop") && len(service.CapDrop) != 0 {
		service.CapDrop = nil
		c.error("cap_drop")
	}
}

func (c *WhiteList) CheckCgroupParent(service *types.ServiceConfig) {
	if !c.supported("cgroup_parent") && service.CgroupParent != "" {
		service.CgroupParent = ""
		c.error("cgroup_parent")
	}
}

func (c *WhiteList) CheckCPUQuota(service *types.ServiceConfig) {
	if !c.supported("cpu_quota") && service.CPUQuota != 0 {
		service.CPUQuota = 0
		c.error("cpu_quota")
	}
}

func (c *WhiteList) CheckCPUSet(service *types.ServiceConfig) {
	if !c.supported("cpuset") && service.CPUSet != "" {
		service.CPUSet = ""
		c.error("cpuset")
	}
}

func (c *WhiteList) CheckCPUShares(service *types.ServiceConfig) {
	if !c.supported("cpu_shares") && service.CPUShares != 0 {
		service.CPUShares = 0
		c.error("cpu_shares")
	}
}

func (c *WhiteList) CheckCommand(service *types.ServiceConfig) {
	if !c.supported("command") && len(service.Command) != 0 {
		service.Command = nil
		c.error("command")
	}
}

func (c *WhiteList) CheckConfigs(service *types.ServiceConfig) {
	if !c.supported("configs") && len(service.Configs) != 0 {
		service.Configs = nil
		c.error("configs")
	}
}

func (c *WhiteList) CheckContainerName(service *types.ServiceConfig) {
	if !c.supported("container_name") && service.ContainerName != "" {
		service.ContainerName = ""
		c.error("container_name")
	}
}

func (c *WhiteList) CheckCredentialSpec(service *types.ServiceConfig) {
	if !c.supported("credential_spec") && service.CredentialSpec != nil {
		service.CredentialSpec = nil
		c.error("credential_spec")
	}
}

func (c *WhiteList) CheckDependsOn(service *types.ServiceConfig) {
	if !c.supported("depends_on") && len(service.DependsOn) != 0 {
		service.DependsOn = nil
		c.error("depends_on")
	}
}

func (c *WhiteList) CheckDeploy(service *types.ServiceConfig) {
	if !c.supported("deploy") && service.Deploy != nil {
		service.Deploy = nil
		c.error("deploy")
	}
}

func (c *WhiteList) CheckDevices(service *types.ServiceConfig) {
	if !c.supported("devices") && len(service.Devices) != 0 {
		service.Devices = nil
		c.error("devices")
	}
}

func (c *WhiteList) CheckDNS(service *types.ServiceConfig) {
	if !c.supported("dns") && service.DNS != nil {
		service.DNS = nil
		c.error("dns")
	}
}

func (c *WhiteList) CheckDNSOpts(service *types.ServiceConfig) {
	if !c.supported("dns_opt") && len(service.DNSOpts) != 0 {
		service.DNSOpts = nil
		c.error("dns_opt")
	}
}

func (c *WhiteList) CheckDNSSearch(service *types.ServiceConfig) {
	if !c.supported("dns_search") && len(service.DNSSearch) != 0 {
		service.DNSSearch = nil
		c.error("dns_search")
	}
}

func (c *WhiteList) CheckDomainName(service *types.ServiceConfig) {
	if !c.supported("domainname") && service.DomainName != "" {
		service.DomainName = ""
		c.error("domainname")
	}
}

func (c *WhiteList) CheckEntrypoint(service *types.ServiceConfig) {
	if !c.supported("entrypoint") && len(service.Entrypoint) != 0 {
		service.Entrypoint = nil
		c.error("entrypoint")
	}
}

func (c *WhiteList) CheckEnvironment(service *types.ServiceConfig) {
	if !c.supported("environment") && len(service.Environment) != 0 {
		service.Environment = nil
		c.error("environment")
	}
}

func (c *WhiteList) CheckEnvFile(service *types.ServiceConfig) {
	if !c.supported("env_file") && len(service.EnvFile) != 0 {
		service.EnvFile = nil
		c.error("env_file")
	}
}

func (c *WhiteList) CheckExpose(service *types.ServiceConfig) {
	if !c.supported("expose") && len(service.Expose) != 0 {
		service.Expose = nil
		c.error("expose")
	}
}

func (c *WhiteList) CheckExtends(service *types.ServiceConfig) {
	if !c.supported("extends") && len(service.Extends) != 0 {
		service.Extends = nil
		c.error("extends")
	}
}

func (c *WhiteList) CheckExternalLinks(service *types.ServiceConfig) {
	if !c.supported("external_links") && len(service.ExternalLinks) != 0 {
		service.ExternalLinks = nil
		c.error("external_links")
	}
}

func (c *WhiteList) CheckExtraHosts(service *types.ServiceConfig) {
	if !c.supported("extra_hosts") && len(service.ExtraHosts) != 0 {
		service.ExtraHosts = nil
		c.error("extra_hosts")
	}
}

func (c *WhiteList) CheckGroupAdd(service *types.ServiceConfig) {
	if !c.supported("group_app") && len(service.GroupAdd) != 0 {
		service.GroupAdd = nil
		c.error("group_app")
	}
}

func (c *WhiteList) CheckHostname(service *types.ServiceConfig) {
	if !c.supported("hostname") && service.Hostname != "" {
		service.Hostname = ""
		c.error("hostname")
	}
}

func (c *WhiteList) CheckHealthCheck(service *types.ServiceConfig) {
	if !c.supported("healthcheck") && service.HealthCheck != nil {
		service.HealthCheck = nil
		c.error("healthcheck")
	}
}

func (c *WhiteList) CheckInit(service *types.ServiceConfig) {
	if !c.supported("init") && service.Init != nil {
		service.Init = nil
		c.error("init")
	}
}

func (c *WhiteList) CheckIpc(service *types.ServiceConfig) {
	if !c.supported("ipc") && service.Ipc != "" {
		service.Ipc = ""
		c.error("ipc")
	}
}

func (c *WhiteList) CheckIsolation(service *types.ServiceConfig) {
	if !c.supported("isolation") && service.Isolation != "" {
		service.Isolation = ""
		c.error("isolation")
	}
}

func (c *WhiteList) CheckLabels(service *types.ServiceConfig) {
	if !c.supported("labels") && len(service.Labels) != 0 {
		service.Labels = nil
		c.error("labels")
	}
}

func (c *WhiteList) CheckLinks(service *types.ServiceConfig) {
	if !c.supported("links") && len(service.Links) != 0 {
		service.Links = nil
		c.error("links")
	}
}

func (c *WhiteList) CheckLogging(service *types.ServiceConfig) {
	if !c.supported("logging") && service.Logging != nil {
		service.Logging = nil
		c.error("logging")
	}
}

func (c *WhiteList) CheckMemLimit(service *types.ServiceConfig) {
	if !c.supported("mem_limit") && service.MemLimit != 0 {
		service.MemLimit = 0
		c.error("mem_limit")
	}
}

func (c *WhiteList) CheckMemReservation(service *types.ServiceConfig) {
	if !c.supported("mem_reservation") && service.MemReservation != 0 {
		service.MemReservation = 0
		c.error("mem_reservation")
	}
}

func (c *WhiteList) CheckMemSwapLimit(service *types.ServiceConfig) {
	if !c.supported("memswap_limit") && service.MemSwapLimit != 0 {
		service.MemSwapLimit = 0
		c.error("memswap_limit")
	}
}

func (c *WhiteList) CheckMemSwappiness(service *types.ServiceConfig) {
	if !c.supported("mem_swappiness") && service.MemSwappiness != 0 {
		service.MemSwappiness = 0
		c.error("mem_swappiness")
	}
}

func (c *WhiteList) CheckMacAddress(service *types.ServiceConfig) {
	if !c.supported("mac_address") && service.MacAddress != "" {
		service.MacAddress = ""
		c.error("mac_address")
	}
}

func (c *WhiteList) CheckNet(service *types.ServiceConfig) {
	if !c.supported("net") && service.Net != "" {
		service.Net = ""
		c.error("net")
	}
}

func (c *WhiteList) CheckNetworkMode(service *types.ServiceConfig) {
	if !c.supported("network_mode") && service.NetworkMode != "" {
		service.NetworkMode = ""
		c.error("network_mode")
	}
}

func (c *WhiteList) CheckNetworks(service *types.ServiceConfig) {
	if !c.supported("networks") && len(service.Networks) != 0 {
		service.Networks = nil
		c.error("networks")
	}
}

func (c *WhiteList) CheckOomKillDisable(service *types.ServiceConfig) {
	if !c.supported("oom_kill_disable") && service.OomKillDisable {
		service.OomKillDisable = false
		c.error("oom_kill_disable")
	}
}

func (c *WhiteList) CheckOomScoreAdj(service *types.ServiceConfig) {
	if !c.supported("oom_score_adj") && service.OomScoreAdj != 0 {
		service.OomScoreAdj = 0
		c.error("oom_score_adj")
	}
}

func (c *WhiteList) CheckPid(service *types.ServiceConfig) {
	if !c.supported("pid") && service.Pid != "" {
		service.Pid = ""
		c.error("pid")
	}
}

func (c *WhiteList) CheckPorts(service *types.ServiceConfig) {
	if !c.supported("ports") && len(service.Ports) != 0 {
		service.Ports = nil
		c.error("ports")
	}
}

func (c *WhiteList) CheckPrivileged(service *types.ServiceConfig) {
	if !c.supported("privileged") && service.Privileged {
		service.Privileged = false
		c.error("privileged")
	}
}

func (c *WhiteList) CheckReadOnly(service *types.ServiceConfig) {
	if !c.supported("read_only") && service.ReadOnly {
		service.ReadOnly = false
		c.error("read_only")
	}
}

func (c *WhiteList) CheckRestart(service *types.ServiceConfig) {
	if !c.supported("restart") && service.Restart != "" {
		service.Restart = ""
		c.error("restart")
	}
}

func (c *WhiteList) CheckSecrets(service *types.ServiceConfig) {
	if !c.supported("secrets") && len(service.Secrets) != 0 {
		service.Secrets = nil
		c.error("secrets")
	}
}

func (c *WhiteList) CheckSecurityOpt(service *types.ServiceConfig) {
	if !c.supported("security_opt") && len(service.SecurityOpt) != 0 {
		service.SecurityOpt = nil
		c.error("security_opt")
	}
}

func (c *WhiteList) CheckShmSize(service *types.ServiceConfig) {
	if !c.supported("shm_size") && service.ShmSize != "" {
		service.ShmSize = ""
		c.error("shm_size")
	}
}

func (c *WhiteList) CheckStdinOpen(service *types.ServiceConfig) {
	if !c.supported("stdin_open") && service.StdinOpen {
		service.StdinOpen = true
		c.error("stdin_open")
	}
}

func (c *WhiteList) CheckStopGracePeriod(service *types.ServiceConfig) {
	if !c.supported("stop_grace_period") && service.StopGracePeriod != nil {
		service.StopGracePeriod = nil
		c.error("stop_grace_period")
	}
}

func (c *WhiteList) CheckStopSignal(service *types.ServiceConfig) {
	if !c.supported("stop_signal") && service.StopSignal != "" {
		service.StopSignal = ""
		c.error("stop_signal")
	}
}

func (c *WhiteList) CheckSysctls(service *types.ServiceConfig) {
	if !c.supported("sysctls") && len(service.Sysctls) != 0 {
		service.Sysctls = nil
		c.error("sysctls")
	}
}

func (c *WhiteList) CheckTmpfs(service *types.ServiceConfig) {
	if !c.supported("tmpfs") && len(service.Tmpfs) != 0 {
		service.Tmpfs = nil
		c.error("tmpfs")
	}
}

func (c *WhiteList) CheckTty(service *types.ServiceConfig) {
	if !c.supported("tty") && service.Tty {
		service.Tty = false
		c.error("tty")
	}
}

func (c *WhiteList) CheckUlimits(service *types.ServiceConfig) {
	if !c.supported("ulimits") && len(service.Ulimits) != 0 {
		service.Ulimits = nil
		c.error("ulimits")
	}
}

func (c *WhiteList) CheckUser(service *types.ServiceConfig) {
	if !c.supported("user") && service.User != "" {
		service.User = ""
		c.error("user")
	}
}

func (c *WhiteList) CheckUserNSMode(service *types.ServiceConfig) {
	if !c.supported("userns_mode") && service.UserNSMode != "" {
		service.UserNSMode = ""
		c.error("userns_mode")
	}
}

func (c *WhiteList) CheckUts(service *types.ServiceConfig) {
	if !c.supported("build") && service.Uts != "" {
		service.Uts = ""
		c.error("uts")
	}
}

func (c *WhiteList) CheckVolumeDriver(service *types.ServiceConfig) {
	if !c.supported("volume_driver") && service.VolumeDriver != "" {
		service.VolumeDriver = ""
		c.error("volume_driver")
	}
}

func (c *WhiteList) CheckVolumes(service *types.ServiceConfig) {
	if !c.supported("volumes") && len(service.Volumes) != 0 {
		service.Volumes = nil
		c.error("volumes")
	}
}

func (c *WhiteList) CheckVolumesFrom(service *types.ServiceConfig) {
	if !c.supported("volumes_from") && len(service.VolumesFrom) != 0 {
		service.VolumesFrom = nil
		c.error("volumes_from")
	}
}

func (c *WhiteList) CheckWorkingDir(service *types.ServiceConfig) {
	if !c.supported("working_dir") && service.WorkingDir != "" {
		service.WorkingDir = ""
		c.error("working_dir")
	}
}

func (c *WhiteList) supported(attribute string) bool {
	for _, s := range c.Supported {
		if s == attribute {
			return true
		}
	}
	return false
}

func (c *WhiteList) error(message string, args ...interface{}) {
	c.errors = append(c.errors, errors.Wrap(errdefs.ErrUnsupported, fmt.Sprintf(message, args...)))
}
