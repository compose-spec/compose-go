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

	"github.com/compose-spec/compose-go/types"
)

func (c *AllowList) CheckBlkioConfig(service *types.ServiceConfig) {
	if !c.supported("services.blkio_config") && service.BlkioConfig != "" {
		service.BlkioConfig = ""
		c.error("services.blkio_config")
	}
}

func (c *AllowList) CheckCapAdd(service *types.ServiceConfig) {
	if !c.supported("services.cap_add") && len(service.CapAdd) != 0 {
		service.CapAdd = nil
		c.error("services.cap_add")
	}
}

func (c *AllowList) CheckCapDrop(service *types.ServiceConfig) {
	if !c.supported("services.cap_drop") && len(service.CapDrop) != 0 {
		service.CapDrop = nil
		c.error("services.cap_drop")
	}
}

func (c *AllowList) CheckCgroupParent(service *types.ServiceConfig) {
	if !c.supported("services.cgroup_parent") && service.CgroupParent != "" {
		service.CgroupParent = ""
		c.error("services.cgroup_parent")
	}
}

func (c *AllowList) CheckCPUQuota(service *types.ServiceConfig) {
	if !c.supported("services.cpu_quota") && service.CPUQuota != 0 {
		service.CPUQuota = 0
		c.error("services.cpu_quota")
	}
}

func (c *AllowList) CheckCPUCount(service *types.ServiceConfig) {
	if !c.supported("services.cpu_count") && service.CPUCount != 0 {
		service.CPUCount = 0
		c.error("services.cpu_count")
	}
}

func (c *AllowList) CheckCPUPercent(service *types.ServiceConfig) {
	if !c.supported("services.cpu_percent") && service.CPUPercent != 0 {
		service.CPUPercent = 0
		c.error("services.cpu_percent")
	}
}

func (c *AllowList) CheckCPUPeriod(service *types.ServiceConfig) {
	if !c.supported("services.cpu_period") && service.CPUPeriod != 0 {
		service.CPUPeriod = 0
		c.error("services.cpu_period")
	}
}

func (c *AllowList) CheckCPURTRuntime(service *types.ServiceConfig) {
	if !c.supported("services.cpu_rt_runtime") && service.CPURTRuntime != 0 {
		service.CPURTRuntime = 0
		c.error("services.cpu_rt_period")
	}
}

func (c *AllowList) CheckCPURTPeriod(service *types.ServiceConfig) {
	if !c.supported("services.cpu_rt_period") && service.CPURTPeriod != 0 {
		service.CPURTPeriod = 0
		c.error("services.cpu_rt_period")
	}
}

func (c *AllowList) CheckCPUs(service *types.ServiceConfig) {
	if !c.supported("services.cpus") && service.CPUS != 0 {
		service.CPUS = 0
		c.error("services.cpus")
	}
}

func (c *AllowList) CheckCPUSet(service *types.ServiceConfig) {
	if !c.supported("services.cpuset") && service.CPUSet != "" {
		service.CPUSet = ""
		c.error("services.cpuset")
	}
}

func (c *AllowList) CheckCPUShares(service *types.ServiceConfig) {
	if !c.supported("services.cpu_shares") && service.CPUShares != 0 {
		service.CPUShares = 0
		c.error("services.cpu_shares")
	}
}

func (c *AllowList) CheckCommand(service *types.ServiceConfig) {
	if !c.supported("services.command") && len(service.Command) != 0 {
		service.Command = nil
		c.error("services.command")
	}
}

func (c *AllowList) CheckConfigs(service *types.ServiceConfig) {
	if len(service.Configs) != 0 {
		if !c.supported("services.configs") {
			service.Configs = nil
			c.error("services.configs")
			return
		}
		for i, s := range service.Secrets {
			ref := types.FileReferenceConfig(s)
			c.CheckFileReference("configs", &ref)
			service.Secrets[i] = s
		}
	}
}

func (c *AllowList) CheckContainerName(service *types.ServiceConfig) {
	if !c.supported("services.container_name") && service.ContainerName != "" {
		service.ContainerName = ""
		c.error("services.container_name")
	}
}

func (c *AllowList) CheckCredentialSpec(service *types.ServiceConfig) {
	if !c.supported("services.credential_spec") && service.CredentialSpec != nil {
		service.CredentialSpec = nil
		c.error("services.credential_spec")
	}
}

func (c *AllowList) CheckDependsOn(service *types.ServiceConfig) {
	if !c.supported("services.depends_on") && len(service.DependsOn) != 0 {
		service.DependsOn = nil
		c.error("services.depends_on")
	}
}

func (c *AllowList) CheckDevices(service *types.ServiceConfig) {
	if !c.supported("services.devices") && len(service.Devices) != 0 {
		service.Devices = nil
		c.error("services.devices")
	}
}

func (c *AllowList) CheckDNS(service *types.ServiceConfig) {
	if !c.supported("services.dns") && service.DNS != nil {
		service.DNS = nil
		c.error("services.dns")
	}
}

func (c *AllowList) CheckDNSOpts(service *types.ServiceConfig) {
	if !c.supported("services.dns_opt") && len(service.DNSOpts) != 0 {
		service.DNSOpts = nil
		c.error("services.dns_opt")
	}
}

func (c *AllowList) CheckDNSSearch(service *types.ServiceConfig) {
	if !c.supported("services.dns_search") && len(service.DNSSearch) != 0 {
		service.DNSSearch = nil
		c.error("services.dns_search")
	}
}

func (c *AllowList) CheckDomainName(service *types.ServiceConfig) {
	if !c.supported("services.domainname") && service.DomainName != "" {
		service.DomainName = ""
		c.error("services.domainname")
	}
}

func (c *AllowList) CheckEntrypoint(service *types.ServiceConfig) {
	if !c.supported("services.entrypoint") && len(service.Entrypoint) != 0 {
		service.Entrypoint = nil
		c.error("services.entrypoint")
	}
}

func (c *AllowList) CheckEnvironment(service *types.ServiceConfig) {
	if !c.supported("services.environment") && len(service.Environment) != 0 {
		service.Environment = nil
		c.error("services.environment")
	}
}

func (c *AllowList) CheckEnvFile(service *types.ServiceConfig) {
	if !c.supported("services.env_file") && len(service.EnvFile) != 0 {
		service.EnvFile = nil
		c.error("services.env_file")
	}
}

func (c *AllowList) CheckExpose(service *types.ServiceConfig) {
	if !c.supported("services.expose") && len(service.Expose) != 0 {
		service.Expose = nil
		c.error("services.expose")
	}
}

func (c *AllowList) CheckExtends(service *types.ServiceConfig) {
	if !c.supported("services.extends") && len(service.Extends) != 0 {
		service.Extends = nil
		c.error("services.extends")
	}
}

func (c *AllowList) CheckExternalLinks(service *types.ServiceConfig) {
	if !c.supported("services.external_links") && len(service.ExternalLinks) != 0 {
		service.ExternalLinks = nil
		c.error("services.external_links")
	}
}

func (c *AllowList) CheckExtraHosts(service *types.ServiceConfig) {
	if !c.supported("services.extra_hosts") && len(service.ExtraHosts) != 0 {
		service.ExtraHosts = nil
		c.error("services.extra_hosts")
	}
}

func (c *AllowList) CheckGroupAdd(service *types.ServiceConfig) {
	if !c.supported("services.group_app") && len(service.GroupAdd) != 0 {
		service.GroupAdd = nil
		c.error("services.group_app")
	}
}

func (c *AllowList) CheckHostname(service *types.ServiceConfig) {
	if !c.supported("services.hostname") && service.Hostname != "" {
		service.Hostname = ""
		c.error("services.hostname")
	}
}

func (c *AllowList) CheckHealthCheck(service *types.ServiceConfig) bool {
	if !c.supported("services.healthcheck") {
		service.HealthCheck = nil
		c.error("services.healthcheck")
		return false
	}
	return true
}

func (c *AllowList) CheckHealthCheckTest(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.test") && len(h.Test) != 0 {
		h.Test = nil
		c.error("services.healthcheck.test")
	}
}

func (c *AllowList) CheckHealthCheckTimeout(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.timeout") && h.Timeout != nil {
		h.Timeout = nil
		c.error("services.healthcheck.timeout")
	}
}

func (c *AllowList) CheckHealthCheckInterval(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.interval") && h.Interval != nil {
		h.Interval = nil
		c.error("services.healthcheck.interval")
	}
}

func (c *AllowList) CheckHealthCheckRetries(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.retries") && h.Retries != nil {
		h.Retries = nil
		c.error("services.healthcheck.retries")
	}
}

func (c *AllowList) CheckHealthCheckStartPeriod(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.start_period") && h.StartPeriod != nil {
		h.StartPeriod = nil
		c.error("services.healthcheck.start_period")
	}
}

func (c *AllowList) CheckInit(service *types.ServiceConfig) {
	if !c.supported("services.init") && service.Init != nil {
		service.Init = nil
		c.error("services.init")
	}
}

func (c *AllowList) CheckIpc(service *types.ServiceConfig) {
	if !c.supported("services.ipc") && service.Ipc != "" {
		service.Ipc = ""
		c.error("services.ipc")
	}
}

func (c *AllowList) CheckIsolation(service *types.ServiceConfig) {
	if !c.supported("services.isolation") && service.Isolation != "" {
		service.Isolation = ""
		c.error("services.isolation")
	}
}

func (c *AllowList) CheckLabels(service *types.ServiceConfig) {
	if !c.supported("services.labels") && len(service.Labels) != 0 {
		service.Labels = nil
		c.error("services.labels")
	}
}

func (c *AllowList) CheckLinks(service *types.ServiceConfig) {
	if !c.supported("services.links") && len(service.Links) != 0 {
		service.Links = nil
		c.error("services.links")
	}
}

func (c *AllowList) CheckLogging(service *types.ServiceConfig) bool {
	if !c.supported("services.logging") {
		service.Logging = nil
		c.error("services.logging")
		return false
	}
	return true
}

func (c *AllowList) CheckLoggingDriver(logging *types.LoggingConfig) {
	if !c.supported("services.logging.driver") && logging.Driver != "" {
		logging.Driver = ""
		c.error("services.logging.driver")
	}
}

func (c *AllowList) CheckLoggingOptions(logging *types.LoggingConfig) {
	if !c.supported("services.logging.options") && len(logging.Options) != 0 {
		logging.Options = nil
		c.error("services.logging.options")
	}
}

func (c *AllowList) CheckMemLimit(service *types.ServiceConfig) {
	if !c.supported("services.mem_limit") && service.MemLimit != 0 {
		service.MemLimit = 0
		c.error("services.mem_limit")
	}
}

func (c *AllowList) CheckMemReservation(service *types.ServiceConfig) {
	if !c.supported("services.mem_reservation") && service.MemReservation != 0 {
		service.MemReservation = 0
		c.error("services.mem_reservation")
	}
}

func (c *AllowList) CheckMemSwapLimit(service *types.ServiceConfig) {
	if !c.supported("services.memswap_limit") && service.MemSwapLimit != 0 {
		service.MemSwapLimit = 0
		c.error("services.memswap_limit")
	}
}

func (c *AllowList) CheckMemSwappiness(service *types.ServiceConfig) {
	if !c.supported("services.mem_swappiness") && service.MemSwappiness != 0 {
		service.MemSwappiness = 0
		c.error("services.mem_swappiness")
	}
}

func (c *AllowList) CheckMacAddress(service *types.ServiceConfig) {
	if !c.supported("services.mac_address") && service.MacAddress != "" {
		service.MacAddress = ""
		c.error("services.mac_address")
	}
}

func (c *AllowList) CheckNet(service *types.ServiceConfig) {
	if !c.supported("services.net") && service.Net != "" {
		service.Net = ""
		c.error("services.net")
	}
}

func (c *AllowList) CheckNetworkMode(service *types.ServiceConfig) {
	if !c.supported("services.network_mode") && service.NetworkMode != "" {
		service.NetworkMode = ""
		c.error("services.network_mode")
	}
}

func (c *AllowList) CheckNetworks(service *types.ServiceConfig) bool {
	if !c.supported("services.networks") {
		service.Networks = nil
		c.error("services.networks")
		return false
	}
	return true
}

func (c *AllowList) CheckNetworkAliases(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.aliases") && len(n.Aliases) != 0 {
		n.Aliases = nil
		c.error("services.networks.aliases")
	}
}

func (c *AllowList) CheckNetworkIpv4Address(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.ipv4_address") && n.Ipv4Address != "" {
		n.Ipv4Address = ""
		c.error("services.networks.ipv4_address")
	}
}

func (c *AllowList) CheckNetworkIpv6Address(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.ipv6_address") && n.Ipv6Address != "" {
		n.Ipv6Address = ""
		c.error("services.networks.ipv6_address")
	}
}

func (c *AllowList) CheckOomKillDisable(service *types.ServiceConfig) {
	if !c.supported("services.oom_kill_disable") && service.OomKillDisable {
		service.OomKillDisable = false
		c.error("services.oom_kill_disable")
	}
}

func (c *AllowList) CheckOomScoreAdj(service *types.ServiceConfig) {
	if !c.supported("services.oom_score_adj") && service.OomScoreAdj != 0 {
		service.OomScoreAdj = 0
		c.error("services.oom_score_adj")
	}
}

func (c *AllowList) CheckPid(service *types.ServiceConfig) {
	if !c.supported("services.pid") && service.Pid != "" {
		service.Pid = ""
		c.error("services.pid")
	}
}

func (c *AllowList) CheckPidLimit(service *types.ServiceConfig) {
	if !c.supported("services.pid_limit") && service.PidLimit != 0 {
		service.PidLimit = 0
		c.error("services.pid_limit")
	}
}

func (c *AllowList) CheckPlatform(service *types.ServiceConfig) {
	if !c.supported("services.platform") && service.Platform != "" {
		service.Platform = ""
		c.error("services.platform")
	}
}

func (c *AllowList) CheckPorts(service *types.ServiceConfig) bool {
	if !c.supported("services.ports") {
		service.Ports = nil
		c.error("services.ports")
		return false
	}
	return true
}

func (c *AllowList) CheckPortsMode(p *types.ServicePortConfig) {
	if !c.supported("services.ports.mode") && p.Mode != "" {
		p.Mode = ""
		c.error("services.ports.mode")
	}
}

func (c *AllowList) CheckPortsTarget(p *types.ServicePortConfig) {
	if !c.supported("services.ports.target") && p.Target != 0 {
		p.Target = 0
		c.error("services.ports.target")
	}
}

func (c *AllowList) CheckPortsPublished(p *types.ServicePortConfig) {
	if !c.supported("services.ports.published") && p.Published != 0 {
		p.Published = 0
		c.error("services.ports.published")
	}
}

func (c *AllowList) CheckPortsProtocol(p *types.ServicePortConfig) {
	if !c.supported("services.ports.protocol") && p.Protocol != "" {
		p.Protocol = ""
		c.error("services.ports.protocol")
	}
}

func (c *AllowList) CheckPrivileged(service *types.ServiceConfig) {
	if !c.supported("services.privileged") && service.Privileged {
		service.Privileged = false
		c.error("services.privileged")
	}
}

func (c *AllowList) CheckReadOnly(service *types.ServiceConfig) {
	if !c.supported("services.read_only") && service.ReadOnly {
		service.ReadOnly = false
		c.error("services.read_only")
	}
}

func (c *AllowList) CheckRestart(service *types.ServiceConfig) {
	if !c.supported("services.restart") && service.Restart != "" {
		service.Restart = ""
		c.error("services.restart")
	}
}

func (c *AllowList) CheckRuntime(service *types.ServiceConfig) {
	if !c.supported("services.runtime") && service.Runtime != "" {
		service.Runtime = ""
		c.error("services.runtime")
	}
}

func (c *AllowList) CheckScale(service *types.ServiceConfig) {
	if !c.supported("services.scale") && service.Scale != 0 {
		service.Scale = 0
		c.error("services.scale")
	}
}

func (c *AllowList) CheckSecrets(service *types.ServiceConfig) {
	if len(service.Secrets) != 0 {
		if !c.supported("services.secrets") {
			service.Secrets = nil
			c.error("services.secrets")
		}
		for i, s := range service.Secrets {
			ref := types.FileReferenceConfig(s)
			c.CheckFileReference("services.secrets", &ref)
			service.Secrets[i] = s
		}
	}
}

func (c *AllowList) CheckFileReference(s string, config *types.FileReferenceConfig) {
	c.CheckFileReferenceSource(s, config)
	c.CheckFileReferenceTarget(s, config)
	c.CheckFileReferenceGID(s, config)
	c.CheckFileReferenceUID(s, config)
	c.CheckFileReferenceMode(s, config)
}

func (c *AllowList) CheckFileReferenceSource(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.source", s)
	if !c.supported(k) && config.Source != "" {
		config.Source = ""
		c.error(k)
	}
}

func (c *AllowList) CheckFileReferenceTarget(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.target", s)
	if !c.supported(k) && config.Target == "" {
		config.Target = ""
		c.error(k)
	}
}

func (c *AllowList) CheckFileReferenceUID(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.uid", s)
	if !c.supported(k) && config.UID != "" {
		config.UID = ""
		c.error(k)
	}
}

func (c *AllowList) CheckFileReferenceGID(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.gid", s)
	if !c.supported(k) && config.GID != "" {
		config.GID = ""
		c.error(k)
	}
}

func (c *AllowList) CheckFileReferenceMode(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.mode", s)
	if !c.supported(k) && config.Mode != nil {
		config.Mode = nil
		c.error(k)
	}
}

func (c *AllowList) CheckSecurityOpt(service *types.ServiceConfig) {
	if !c.supported("services.security_opt") && len(service.SecurityOpt) != 0 {
		service.SecurityOpt = nil
		c.error("services.security_opt")
	}
}

func (c *AllowList) CheckShmSize(service *types.ServiceConfig) {
	if !c.supported("services.shm_size") && service.ShmSize != "" {
		service.ShmSize = ""
		c.error("services.shm_size")
	}
}

func (c *AllowList) CheckStdinOpen(service *types.ServiceConfig) {
	if !c.supported("services.stdin_open") && service.StdinOpen {
		service.StdinOpen = true
		c.error("services.stdin_open")
	}
}

func (c *AllowList) CheckStopGracePeriod(service *types.ServiceConfig) {
	if !c.supported("services.stop_grace_period") && service.StopGracePeriod != nil {
		service.StopGracePeriod = nil
		c.error("services.stop_grace_period")
	}
}

func (c *AllowList) CheckStopSignal(service *types.ServiceConfig) {
	if !c.supported("services.stop_signal") && service.StopSignal != "" {
		service.StopSignal = ""
		c.error("services.stop_signal")
	}
}

func (c *AllowList) CheckSysctls(service *types.ServiceConfig) {
	if !c.supported("services.sysctls") && len(service.Sysctls) != 0 {
		service.Sysctls = nil
		c.error("services.sysctls")
	}
}

func (c *AllowList) CheckTmpfs(service *types.ServiceConfig) {
	if !c.supported("services.tmpfs") && len(service.Tmpfs) != 0 {
		service.Tmpfs = nil
		c.error("services.tmpfs")
	}
}

func (c *AllowList) CheckTty(service *types.ServiceConfig) {
	if !c.supported("services.tty") && service.Tty {
		service.Tty = false
		c.error("services.tty")
	}
}

func (c *AllowList) CheckUlimits(service *types.ServiceConfig) {
	if !c.supported("services.ulimits") && len(service.Ulimits) != 0 {
		service.Ulimits = nil
		c.error("services.ulimits")
	}
}

func (c *AllowList) CheckUser(service *types.ServiceConfig) {
	if !c.supported("services.user") && service.User != "" {
		service.User = ""
		c.error("services.user")
	}
}

func (c *AllowList) CheckUserNSMode(service *types.ServiceConfig) {
	if !c.supported("services.userns_mode") && service.UserNSMode != "" {
		service.UserNSMode = ""
		c.error("services.userns_mode")
	}
}

func (c *AllowList) CheckUts(service *types.ServiceConfig) {
	if !c.supported("services.build") && service.Uts != "" {
		service.Uts = ""
		c.error("services.uts")
	}
}

func (c *AllowList) CheckVolumeDriver(service *types.ServiceConfig) {
	if !c.supported("services.volume_driver") && service.VolumeDriver != "" {
		service.VolumeDriver = ""
		c.error("services.volume_driver")
	}
}

func (c *AllowList) CheckServiceVolumes(service *types.ServiceConfig) bool {
	if !c.supported("services.volumes") {
		service.Volumes = nil
		c.error("services.volumes")
		return false
	}
	return true
}

func (c *AllowList) CheckVolumesSource(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.source") && config.Source != "" {
		config.Source = ""
		c.error("services.volumes.source")
	}
}

func (c *AllowList) CheckVolumesTarget(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.target") && config.Target != "" {
		config.Target = ""
		c.error("services.volumes.target")
	}
}

func (c *AllowList) CheckVolumesReadOnly(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.read_only") && config.ReadOnly {
		config.ReadOnly = false
		c.error("services.volumes.read_only")
	}
}

func (c *AllowList) CheckVolumesConsistency(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.consistency") && config.Consistency != "" {
		config.Consistency = ""
		c.error("services.volumes.consistency")
	}
}

func (c *AllowList) CheckVolumesBind(config *types.ServiceVolumeBind) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.bind.propagation") && config.Propagation != "" {
		config.Propagation = ""
		c.error("services.volumes.bind.propagation")
	}
}

func (c *AllowList) CheckVolumesVolume(config *types.ServiceVolumeVolume) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.nocopy") && config.NoCopy {
		config.NoCopy = false
		c.error("services.volumes.nocopy")
	}
}

func (c *AllowList) CheckVolumesTmpfs(config *types.ServiceVolumeTmpfs) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.tmpfs.size") && config.Size != 0 {
		config.Size = 0
		c.error("services.volumes.tmpfs.size")
	}
}

func (c *AllowList) CheckVolumesFrom(service *types.ServiceConfig) {
	if !c.supported("services.volumes_from") && len(service.VolumesFrom) != 0 {
		service.VolumesFrom = nil
		c.error("services.volumes_from")
	}
}

func (c *AllowList) CheckWorkingDir(service *types.ServiceConfig) {
	if !c.supported("services.working_dir") && service.WorkingDir != "" {
		service.WorkingDir = ""
		c.error("services.working_dir")
	}
}
