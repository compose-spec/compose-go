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
		c.Error("services.blkio_config")
	}
}

func (c *AllowList) CheckCapAdd(service *types.ServiceConfig) {
	if !c.supported("services.cap_add") && len(service.CapAdd) != 0 {
		service.CapAdd = nil
		c.Error("services.cap_add")
	}
}

func (c *AllowList) CheckCapDrop(service *types.ServiceConfig) {
	if !c.supported("services.cap_drop") && len(service.CapDrop) != 0 {
		service.CapDrop = nil
		c.Error("services.cap_drop")
	}
}

func (c *AllowList) CheckCgroupParent(service *types.ServiceConfig) {
	if !c.supported("services.cgroup_parent") && service.CgroupParent != "" {
		service.CgroupParent = ""
		c.Error("services.cgroup_parent")
	}
}

func (c *AllowList) CheckCPUQuota(service *types.ServiceConfig) {
	if !c.supported("services.cpu_quota") && service.CPUQuota != 0 {
		service.CPUQuota = 0
		c.Error("services.cpu_quota")
	}
}

func (c *AllowList) CheckCPUCount(service *types.ServiceConfig) {
	if !c.supported("services.cpu_count") && service.CPUCount != 0 {
		service.CPUCount = 0
		c.Error("services.cpu_count")
	}
}

func (c *AllowList) CheckCPUPercent(service *types.ServiceConfig) {
	if !c.supported("services.cpu_percent") && service.CPUPercent != 0 {
		service.CPUPercent = 0
		c.Error("services.cpu_percent")
	}
}

func (c *AllowList) CheckCPUPeriod(service *types.ServiceConfig) {
	if !c.supported("services.cpu_period") && service.CPUPeriod != 0 {
		service.CPUPeriod = 0
		c.Error("services.cpu_period")
	}
}

func (c *AllowList) CheckCPURTRuntime(service *types.ServiceConfig) {
	if !c.supported("services.cpu_rt_runtime") && service.CPURTRuntime != 0 {
		service.CPURTRuntime = 0
		c.Error("services.cpu_rt_period")
	}
}

func (c *AllowList) CheckCPURTPeriod(service *types.ServiceConfig) {
	if !c.supported("services.cpu_rt_period") && service.CPURTPeriod != 0 {
		service.CPURTPeriod = 0
		c.Error("services.cpu_rt_period")
	}
}

func (c *AllowList) CheckCPUs(service *types.ServiceConfig) {
	if !c.supported("services.cpus") && service.CPUS != 0 {
		service.CPUS = 0
		c.Error("services.cpus")
	}
}

func (c *AllowList) CheckCPUSet(service *types.ServiceConfig) {
	if !c.supported("services.cpuset") && service.CPUSet != "" {
		service.CPUSet = ""
		c.Error("services.cpuset")
	}
}

func (c *AllowList) CheckCPUShares(service *types.ServiceConfig) {
	if !c.supported("services.cpu_shares") && service.CPUShares != 0 {
		service.CPUShares = 0
		c.Error("services.cpu_shares")
	}
}

func (c *AllowList) CheckCommand(service *types.ServiceConfig) {
	if !c.supported("services.command") && len(service.Command) != 0 {
		service.Command = nil
		c.Error("services.command")
	}
}

func (c *AllowList) CheckConfigs(service *types.ServiceConfig) {
	if len(service.Configs) != 0 {
		if !c.supported("services.configs") {
			service.Configs = nil
			c.Error("services.configs")
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
		c.Error("services.container_name")
	}
}

func (c *AllowList) CheckCredentialSpec(service *types.ServiceConfig) {
	if !c.supported("services.credential_spec") && service.CredentialSpec != nil {
		service.CredentialSpec = nil
		c.Error("services.credential_spec")
	}
}

func (c *AllowList) CheckDependsOn(service *types.ServiceConfig) {
	if !c.supported("services.depends_on") && len(service.DependsOn) != 0 {
		service.DependsOn = nil
		c.Error("services.depends_on")
	}
}

func (c *AllowList) CheckDevices(service *types.ServiceConfig) {
	if !c.supported("services.devices") && len(service.Devices) != 0 {
		service.Devices = nil
		c.Error("services.devices")
	}
}

func (c *AllowList) CheckDNS(service *types.ServiceConfig) {
	if !c.supported("services.dns") && service.DNS != nil {
		service.DNS = nil
		c.Error("services.dns")
	}
}

func (c *AllowList) CheckDNSOpts(service *types.ServiceConfig) {
	if !c.supported("services.dns_opt") && len(service.DNSOpts) != 0 {
		service.DNSOpts = nil
		c.Error("services.dns_opt")
	}
}

func (c *AllowList) CheckDNSSearch(service *types.ServiceConfig) {
	if !c.supported("services.dns_search") && len(service.DNSSearch) != 0 {
		service.DNSSearch = nil
		c.Error("services.dns_search")
	}
}

func (c *AllowList) CheckDomainName(service *types.ServiceConfig) {
	if !c.supported("services.domainname") && service.DomainName != "" {
		service.DomainName = ""
		c.Error("services.domainname")
	}
}

func (c *AllowList) CheckEntrypoint(service *types.ServiceConfig) {
	if !c.supported("services.entrypoint") && len(service.Entrypoint) != 0 {
		service.Entrypoint = nil
		c.Error("services.entrypoint")
	}
}

func (c *AllowList) CheckEnvironment(service *types.ServiceConfig) {
	if !c.supported("services.environment") && len(service.Environment) != 0 {
		service.Environment = nil
		c.Error("services.environment")
	}
}

func (c *AllowList) CheckEnvFile(service *types.ServiceConfig) {
	if !c.supported("services.env_file") && len(service.EnvFile) != 0 {
		service.EnvFile = nil
		c.Error("services.env_file")
	}
}

func (c *AllowList) CheckExpose(service *types.ServiceConfig) {
	if !c.supported("services.expose") && len(service.Expose) != 0 {
		service.Expose = nil
		c.Error("services.expose")
	}
}

func (c *AllowList) CheckExtends(service *types.ServiceConfig) {
	if !c.supported("services.extends") && len(service.Extends) != 0 {
		service.Extends = nil
		c.Error("services.extends")
	}
}

func (c *AllowList) CheckExternalLinks(service *types.ServiceConfig) {
	if !c.supported("services.external_links") && len(service.ExternalLinks) != 0 {
		service.ExternalLinks = nil
		c.Error("services.external_links")
	}
}

func (c *AllowList) CheckExtraHosts(service *types.ServiceConfig) {
	if !c.supported("services.extra_hosts") && len(service.ExtraHosts) != 0 {
		service.ExtraHosts = nil
		c.Error("services.extra_hosts")
	}
}

func (c *AllowList) CheckGroupAdd(service *types.ServiceConfig) {
	if !c.supported("services.group_app") && len(service.GroupAdd) != 0 {
		service.GroupAdd = nil
		c.Error("services.group_app")
	}
}

func (c *AllowList) CheckHostname(service *types.ServiceConfig) {
	if !c.supported("services.hostname") && service.Hostname != "" {
		service.Hostname = ""
		c.Error("services.hostname")
	}
}

func (c *AllowList) CheckHealthCheck(service *types.ServiceConfig) bool {
	if !c.supported("services.healthcheck") {
		service.HealthCheck = nil
		c.Error("services.healthcheck")
		return false
	}
	return true
}

func (c *AllowList) CheckHealthCheckTest(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.test") && len(h.Test) != 0 {
		h.Test = nil
		c.Error("services.healthcheck.test")
	}
}

func (c *AllowList) CheckHealthCheckTimeout(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.timeout") && h.Timeout != nil {
		h.Timeout = nil
		c.Error("services.healthcheck.timeout")
	}
}

func (c *AllowList) CheckHealthCheckInterval(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.interval") && h.Interval != nil {
		h.Interval = nil
		c.Error("services.healthcheck.interval")
	}
}

func (c *AllowList) CheckHealthCheckRetries(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.retries") && h.Retries != nil {
		h.Retries = nil
		c.Error("services.healthcheck.retries")
	}
}

func (c *AllowList) CheckHealthCheckStartPeriod(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.start_period") && h.StartPeriod != nil {
		h.StartPeriod = nil
		c.Error("services.healthcheck.start_period")
	}
}

func (c *AllowList) CheckInit(service *types.ServiceConfig) {
	if !c.supported("services.init") && service.Init != nil {
		service.Init = nil
		c.Error("services.init")
	}
}

func (c *AllowList) CheckIpc(service *types.ServiceConfig) {
	if !c.supported("services.ipc") && service.Ipc != "" {
		service.Ipc = ""
		c.Error("services.ipc")
	}
}

func (c *AllowList) CheckIsolation(service *types.ServiceConfig) {
	if !c.supported("services.isolation") && service.Isolation != "" {
		service.Isolation = ""
		c.Error("services.isolation")
	}
}

func (c *AllowList) CheckLabels(service *types.ServiceConfig) {
	if !c.supported("services.labels") && len(service.Labels) != 0 {
		service.Labels = nil
		c.Error("services.labels")
	}
}

func (c *AllowList) CheckLinks(service *types.ServiceConfig) {
	if !c.supported("services.links") && len(service.Links) != 0 {
		service.Links = nil
		c.Error("services.links")
	}
}

func (c *AllowList) CheckLogging(service *types.ServiceConfig) bool {
	if !c.supported("services.logging") {
		service.Logging = nil
		c.Error("services.logging")
		return false
	}
	return true
}

func (c *AllowList) CheckLoggingDriver(logging *types.LoggingConfig) {
	if !c.supported("services.logging.driver") && logging.Driver != "" {
		logging.Driver = ""
		c.Error("services.logging.driver")
	}
}

func (c *AllowList) CheckLoggingOptions(logging *types.LoggingConfig) {
	if !c.supported("services.logging.options") && len(logging.Options) != 0 {
		logging.Options = nil
		c.Error("services.logging.options")
	}
}

func (c *AllowList) CheckMemLimit(service *types.ServiceConfig) {
	if !c.supported("services.mem_limit") && service.MemLimit != 0 {
		service.MemLimit = 0
		c.Error("services.mem_limit")
	}
}

func (c *AllowList) CheckMemReservation(service *types.ServiceConfig) {
	if !c.supported("services.mem_reservation") && service.MemReservation != 0 {
		service.MemReservation = 0
		c.Error("services.mem_reservation")
	}
}

func (c *AllowList) CheckMemSwapLimit(service *types.ServiceConfig) {
	if !c.supported("services.memswap_limit") && service.MemSwapLimit != 0 {
		service.MemSwapLimit = 0
		c.Error("services.memswap_limit")
	}
}

func (c *AllowList) CheckMemSwappiness(service *types.ServiceConfig) {
	if !c.supported("services.mem_swappiness") && service.MemSwappiness != 0 {
		service.MemSwappiness = 0
		c.Error("services.mem_swappiness")
	}
}

func (c *AllowList) CheckMacAddress(service *types.ServiceConfig) {
	if !c.supported("services.mac_address") && service.MacAddress != "" {
		service.MacAddress = ""
		c.Error("services.mac_address")
	}
}

func (c *AllowList) CheckNet(service *types.ServiceConfig) {
	if !c.supported("services.net") && service.Net != "" {
		service.Net = ""
		c.Error("services.net")
	}
}

func (c *AllowList) CheckNetworkMode(service *types.ServiceConfig) {
	if !c.supported("services.network_mode") && service.NetworkMode != "" {
		service.NetworkMode = ""
		c.Error("services.network_mode")
	}
}

func (c *AllowList) CheckNetworks(service *types.ServiceConfig) bool {
	if !c.supported("services.networks") {
		service.Networks = nil
		c.Error("services.networks")
		return false
	}
	return true
}

func (c *AllowList) CheckNetworkAliases(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.aliases") && len(n.Aliases) != 0 {
		n.Aliases = nil
		c.Error("services.networks.aliases")
	}
}

func (c *AllowList) CheckNetworkIpv4Address(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.ipv4_address") && n.Ipv4Address != "" {
		n.Ipv4Address = ""
		c.Error("services.networks.ipv4_address")
	}
}

func (c *AllowList) CheckNetworkIpv6Address(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.ipv6_address") && n.Ipv6Address != "" {
		n.Ipv6Address = ""
		c.Error("services.networks.ipv6_address")
	}
}

func (c *AllowList) CheckOomKillDisable(service *types.ServiceConfig) {
	if !c.supported("services.oom_kill_disable") && service.OomKillDisable {
		service.OomKillDisable = false
		c.Error("services.oom_kill_disable")
	}
}

func (c *AllowList) CheckOomScoreAdj(service *types.ServiceConfig) {
	if !c.supported("services.oom_score_adj") && service.OomScoreAdj != 0 {
		service.OomScoreAdj = 0
		c.Error("services.oom_score_adj")
	}
}

func (c *AllowList) CheckPid(service *types.ServiceConfig) {
	if !c.supported("services.pid") && service.Pid != "" {
		service.Pid = ""
		c.Error("services.pid")
	}
}

func (c *AllowList) CheckPidLimit(service *types.ServiceConfig) {
	if !c.supported("services.pid_limit") && service.PidLimit != 0 {
		service.PidLimit = 0
		c.Error("services.pid_limit")
	}
}

func (c *AllowList) CheckPlatform(service *types.ServiceConfig) {
	if !c.supported("services.platform") && service.Platform != "" {
		service.Platform = ""
		c.Error("services.platform")
	}
}

func (c *AllowList) CheckPorts(service *types.ServiceConfig) bool {
	if !c.supported("services.ports") {
		service.Ports = nil
		c.Error("services.ports")
		return false
	}
	return true
}

func (c *AllowList) CheckPortsMode(p *types.ServicePortConfig) {
	if !c.supported("services.ports.mode") && p.Mode != "" {
		p.Mode = ""
		c.Error("services.ports.mode")
	}
}

func (c *AllowList) CheckPortsTarget(p *types.ServicePortConfig) {
	if !c.supported("services.ports.target") && p.Target != 0 {
		p.Target = 0
		c.Error("services.ports.target")
	}
}

func (c *AllowList) CheckPortsPublished(p *types.ServicePortConfig) {
	if !c.supported("services.ports.published") && p.Published != 0 {
		p.Published = 0
		c.Error("services.ports.published")
	}
}

func (c *AllowList) CheckPortsProtocol(p *types.ServicePortConfig) {
	if !c.supported("services.ports.protocol") && p.Protocol != "" {
		p.Protocol = ""
		c.Error("services.ports.protocol")
	}
}

func (c *AllowList) CheckPrivileged(service *types.ServiceConfig) {
	if !c.supported("services.privileged") && service.Privileged {
		service.Privileged = false
		c.Error("services.privileged")
	}
}

func (c *AllowList) CheckReadOnly(service *types.ServiceConfig) {
	if !c.supported("services.read_only") && service.ReadOnly {
		service.ReadOnly = false
		c.Error("services.read_only")
	}
}

func (c *AllowList) CheckRestart(service *types.ServiceConfig) {
	if !c.supported("services.restart") && service.Restart != "" {
		service.Restart = ""
		c.Error("services.restart")
	}
}

func (c *AllowList) CheckRuntime(service *types.ServiceConfig) {
	if !c.supported("services.runtime") && service.Runtime != "" {
		service.Runtime = ""
		c.Error("services.runtime")
	}
}

func (c *AllowList) CheckScale(service *types.ServiceConfig) {
	if !c.supported("services.scale") && service.Scale != 0 {
		service.Scale = 0
		c.Error("services.scale")
	}
}

func (c *AllowList) CheckSecrets(service *types.ServiceConfig) {
	if len(service.Secrets) != 0 {
		if !c.supported("services.secrets") {
			service.Secrets = nil
			c.Error("services.secrets")
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
		c.Error(k)
	}
}

func (c *AllowList) CheckFileReferenceTarget(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.target", s)
	if !c.supported(k) && config.Target == "" {
		config.Target = ""
		c.Error(k)
	}
}

func (c *AllowList) CheckFileReferenceUID(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.uid", s)
	if !c.supported(k) && config.UID != "" {
		config.UID = ""
		c.Error(k)
	}
}

func (c *AllowList) CheckFileReferenceGID(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.gid", s)
	if !c.supported(k) && config.GID != "" {
		config.GID = ""
		c.Error(k)
	}
}

func (c *AllowList) CheckFileReferenceMode(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.mode", s)
	if !c.supported(k) && config.Mode != nil {
		config.Mode = nil
		c.Error(k)
	}
}

func (c *AllowList) CheckSecurityOpt(service *types.ServiceConfig) {
	if !c.supported("services.security_opt") && len(service.SecurityOpt) != 0 {
		service.SecurityOpt = nil
		c.Error("services.security_opt")
	}
}

func (c *AllowList) CheckShmSize(service *types.ServiceConfig) {
	if !c.supported("services.shm_size") && service.ShmSize != "" {
		service.ShmSize = ""
		c.Error("services.shm_size")
	}
}

func (c *AllowList) CheckStdinOpen(service *types.ServiceConfig) {
	if !c.supported("services.stdin_open") && service.StdinOpen {
		service.StdinOpen = true
		c.Error("services.stdin_open")
	}
}

func (c *AllowList) CheckStopGracePeriod(service *types.ServiceConfig) {
	if !c.supported("services.stop_grace_period") && service.StopGracePeriod != nil {
		service.StopGracePeriod = nil
		c.Error("services.stop_grace_period")
	}
}

func (c *AllowList) CheckStopSignal(service *types.ServiceConfig) {
	if !c.supported("services.stop_signal") && service.StopSignal != "" {
		service.StopSignal = ""
		c.Error("services.stop_signal")
	}
}

func (c *AllowList) CheckSysctls(service *types.ServiceConfig) {
	if !c.supported("services.sysctls") && len(service.Sysctls) != 0 {
		service.Sysctls = nil
		c.Error("services.sysctls")
	}
}

func (c *AllowList) CheckTmpfs(service *types.ServiceConfig) {
	if !c.supported("services.tmpfs") && len(service.Tmpfs) != 0 {
		service.Tmpfs = nil
		c.Error("services.tmpfs")
	}
}

func (c *AllowList) CheckTty(service *types.ServiceConfig) {
	if !c.supported("services.tty") && service.Tty {
		service.Tty = false
		c.Error("services.tty")
	}
}

func (c *AllowList) CheckUlimits(service *types.ServiceConfig) {
	if !c.supported("services.ulimits") && len(service.Ulimits) != 0 {
		service.Ulimits = nil
		c.Error("services.ulimits")
	}
}

func (c *AllowList) CheckUser(service *types.ServiceConfig) {
	if !c.supported("services.user") && service.User != "" {
		service.User = ""
		c.Error("services.user")
	}
}

func (c *AllowList) CheckUserNSMode(service *types.ServiceConfig) {
	if !c.supported("services.userns_mode") && service.UserNSMode != "" {
		service.UserNSMode = ""
		c.Error("services.userns_mode")
	}
}

func (c *AllowList) CheckUts(service *types.ServiceConfig) {
	if !c.supported("services.build") && service.Uts != "" {
		service.Uts = ""
		c.Error("services.uts")
	}
}

func (c *AllowList) CheckVolumeDriver(service *types.ServiceConfig) {
	if !c.supported("services.volume_driver") && service.VolumeDriver != "" {
		service.VolumeDriver = ""
		c.Error("services.volume_driver")
	}
}

func (c *AllowList) CheckServiceVolumes(service *types.ServiceConfig) bool {
	if !c.supported("services.volumes") {
		service.Volumes = nil
		c.Error("services.volumes")
		return false
	}
	return true
}

func (c *AllowList) CheckVolumesSource(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.source") && config.Source != "" {
		config.Source = ""
		c.Error("services.volumes.source")
	}
}

func (c *AllowList) CheckVolumesTarget(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.target") && config.Target != "" {
		config.Target = ""
		c.Error("services.volumes.target")
	}
}

func (c *AllowList) CheckVolumesReadOnly(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.read_only") && config.ReadOnly {
		config.ReadOnly = false
		c.Error("services.volumes.read_only")
	}
}

func (c *AllowList) CheckVolumesConsistency(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.consistency") && config.Consistency != "" {
		config.Consistency = ""
		c.Error("services.volumes.consistency")
	}
}

func (c *AllowList) CheckVolumesBind(config *types.ServiceVolumeBind) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.bind.propagation") && config.Propagation != "" {
		config.Propagation = ""
		c.Error("services.volumes.bind.propagation")
	}
}

func (c *AllowList) CheckVolumesVolume(config *types.ServiceVolumeVolume) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.nocopy") && config.NoCopy {
		config.NoCopy = false
		c.Error("services.volumes.nocopy")
	}
}

func (c *AllowList) CheckVolumesTmpfs(config *types.ServiceVolumeTmpfs) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.tmpfs.size") && config.Size != 0 {
		config.Size = 0
		c.Error("services.volumes.tmpfs.size")
	}
}

func (c *AllowList) CheckVolumesFrom(service *types.ServiceConfig) {
	if !c.supported("services.volumes_from") && len(service.VolumesFrom) != 0 {
		service.VolumesFrom = nil
		c.Error("services.volumes_from")
	}
}

func (c *AllowList) CheckWorkingDir(service *types.ServiceConfig) {
	if !c.supported("services.working_dir") && service.WorkingDir != "" {
		service.WorkingDir = ""
		c.Error("services.working_dir")
	}
}
