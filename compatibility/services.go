package compatibility

import (
	"fmt"

	"github.com/compose-spec/compose-go/types"
)

func (c *WhiteList) CheckServiceConfig(service *types.ServiceConfig) {
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

func (c *WhiteList) CheckCapAdd(service *types.ServiceConfig) {
	if !c.supported("services.cap_add") && len(service.CapAdd) != 0 {
		service.CapAdd = nil
		c.error("services.cap_add")
	}
}

func (c *WhiteList) CheckCapDrop(service *types.ServiceConfig) {
	if !c.supported("services.cap_drop") && len(service.CapDrop) != 0 {
		service.CapDrop = nil
		c.error("services.cap_drop")
	}
}

func (c *WhiteList) CheckCgroupParent(service *types.ServiceConfig) {
	if !c.supported("services.cgroup_parent") && service.CgroupParent != "" {
		service.CgroupParent = ""
		c.error("services.cgroup_parent")
	}
}

func (c *WhiteList) CheckCPUQuota(service *types.ServiceConfig) {
	if !c.supported("services.cpu_quota") && service.CPUQuota != 0 {
		service.CPUQuota = 0
		c.error("services.cpu_quota")
	}
}

func (c *WhiteList) CheckCPUSet(service *types.ServiceConfig) {
	if !c.supported("services.cpuset") && service.CPUSet != "" {
		service.CPUSet = ""
		c.error("services.cpuset")
	}
}

func (c *WhiteList) CheckCPUShares(service *types.ServiceConfig) {
	if !c.supported("services.cpu_shares") && service.CPUShares != 0 {
		service.CPUShares = 0
		c.error("services.cpu_shares")
	}
}

func (c *WhiteList) CheckCommand(service *types.ServiceConfig) {
	if !c.supported("services.command") && len(service.Command) != 0 {
		service.Command = nil
		c.error("services.command")
	}
}

func (c *WhiteList) CheckConfigs(service *types.ServiceConfig) {
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

func (c *WhiteList) CheckContainerName(service *types.ServiceConfig) {
	if !c.supported("services.container_name") && service.ContainerName != "" {
		service.ContainerName = ""
		c.error("services.container_name")
	}
}

func (c *WhiteList) CheckCredentialSpec(service *types.ServiceConfig) {
	if !c.supported("services.credential_spec") && service.CredentialSpec != nil {
		service.CredentialSpec = nil
		c.error("services.credential_spec")
	}
}

func (c *WhiteList) CheckDependsOn(service *types.ServiceConfig) {
	if !c.supported("services.depends_on") && len(service.DependsOn) != 0 {
		service.DependsOn = nil
		c.error("services.depends_on")
	}
}

func (c *WhiteList) CheckDevices(service *types.ServiceConfig) {
	if !c.supported("services.devices") && len(service.Devices) != 0 {
		service.Devices = nil
		c.error("services.devices")
	}
}

func (c *WhiteList) CheckDNS(service *types.ServiceConfig) {
	if !c.supported("services.dns") && service.DNS != nil {
		service.DNS = nil
		c.error("services.dns")
	}
}

func (c *WhiteList) CheckDNSOpts(service *types.ServiceConfig) {
	if !c.supported("services.dns_opt") && len(service.DNSOpts) != 0 {
		service.DNSOpts = nil
		c.error("services.dns_opt")
	}
}

func (c *WhiteList) CheckDNSSearch(service *types.ServiceConfig) {
	if !c.supported("services.dns_search") && len(service.DNSSearch) != 0 {
		service.DNSSearch = nil
		c.error("services.dns_search")
	}
}

func (c *WhiteList) CheckDomainName(service *types.ServiceConfig) {
	if !c.supported("services.domainname") && service.DomainName != "" {
		service.DomainName = ""
		c.error("services.domainname")
	}
}

func (c *WhiteList) CheckEntrypoint(service *types.ServiceConfig) {
	if !c.supported("services.entrypoint") && len(service.Entrypoint) != 0 {
		service.Entrypoint = nil
		c.error("services.entrypoint")
	}
}

func (c *WhiteList) CheckEnvironment(service *types.ServiceConfig) {
	if !c.supported("services.environment") && len(service.Environment) != 0 {
		service.Environment = nil
		c.error("services.environment")
	}
}

func (c *WhiteList) CheckEnvFile(service *types.ServiceConfig) {
	if !c.supported("services.env_file") && len(service.EnvFile) != 0 {
		service.EnvFile = nil
		c.error("services.env_file")
	}
}

func (c *WhiteList) CheckExpose(service *types.ServiceConfig) {
	if !c.supported("services.expose") && len(service.Expose) != 0 {
		service.Expose = nil
		c.error("services.expose")
	}
}

func (c *WhiteList) CheckExtends(service *types.ServiceConfig) {
	if !c.supported("services.extends") && len(service.Extends) != 0 {
		service.Extends = nil
		c.error("services.extends")
	}
}

func (c *WhiteList) CheckExternalLinks(service *types.ServiceConfig) {
	if !c.supported("services.external_links") && len(service.ExternalLinks) != 0 {
		service.ExternalLinks = nil
		c.error("services.external_links")
	}
}

func (c *WhiteList) CheckExtraHosts(service *types.ServiceConfig) {
	if !c.supported("services.extra_hosts") && len(service.ExtraHosts) != 0 {
		service.ExtraHosts = nil
		c.error("services.extra_hosts")
	}
}

func (c *WhiteList) CheckGroupAdd(service *types.ServiceConfig) {
	if !c.supported("services.group_app") && len(service.GroupAdd) != 0 {
		service.GroupAdd = nil
		c.error("services.group_app")
	}
}

func (c *WhiteList) CheckHostname(service *types.ServiceConfig) {
	if !c.supported("services.hostname") && service.Hostname != "" {
		service.Hostname = ""
		c.error("services.hostname")
	}
}

func (c *WhiteList) CheckHealthCheck(service *types.ServiceConfig) {
	if service.HealthCheck != nil {
		if !c.supported("services.healthcheck") {
			service.HealthCheck = nil
			c.error("services.healthcheck")
			return
		}
		c.CheckHealthCheckInterval(service.HealthCheck)
		c.CheckHealthCheckRetries(service.HealthCheck)
		c.CheckHealthCheckStartPeriod(service.HealthCheck)
		c.CheckHealthCheckTest(service.HealthCheck)
		c.CheckHealthCheckTimeout(service.HealthCheck)
	}
}

func (c *WhiteList) CheckHealthCheckTest(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.test") && len(h.Test) != 0 {
		h.Test = nil
		c.error("services.healthcheck.test")
	}
}

func (c *WhiteList) CheckHealthCheckTimeout(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.timeout") && h.Timeout != nil {
		h.Timeout = nil
		c.error("services.healthcheck.timeout")
	}
}

func (c *WhiteList) CheckHealthCheckInterval(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.interval") && h.Interval != nil {
		h.Interval = nil
		c.error("services.healthcheck.interval")
	}
}

func (c *WhiteList) CheckHealthCheckRetries(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.retries") && h.Retries != nil {
		h.Retries = nil
		c.error("services.healthcheck.retries")
	}
}

func (c *WhiteList) CheckHealthCheckStartPeriod(h *types.HealthCheckConfig) {
	if !c.supported("services.healthcheck.start_period") && h.StartPeriod != nil {
		h.StartPeriod = nil
		c.error("services.healthcheck.start_period")
	}
}

func (c *WhiteList) CheckInit(service *types.ServiceConfig) {
	if !c.supported("services.init") && service.Init != nil {
		service.Init = nil
		c.error("services.init")
	}
}

func (c *WhiteList) CheckIpc(service *types.ServiceConfig) {
	if !c.supported("services.ipc") && service.Ipc != "" {
		service.Ipc = ""
		c.error("services.ipc")
	}
}

func (c *WhiteList) CheckIsolation(service *types.ServiceConfig) {
	if !c.supported("services.isolation") && service.Isolation != "" {
		service.Isolation = ""
		c.error("services.isolation")
	}
}

func (c *WhiteList) CheckLabels(service *types.ServiceConfig) {
	if !c.supported("services.labels") && len(service.Labels) != 0 {
		service.Labels = nil
		c.error("services.labels")
	}
}

func (c *WhiteList) CheckLinks(service *types.ServiceConfig) {
	if !c.supported("services.links") && len(service.Links) != 0 {
		service.Links = nil
		c.error("services.links")
	}
}

func (c *WhiteList) CheckLogging(service *types.ServiceConfig) {
	if service.Logging != nil {
		if !c.supported("services.logging") {
			service.Logging = nil
			c.error("services.logging")
			return
		}
		c.CheckLoggingDriver(service.Logging)
		c.CheckLoggingOptions(service.Logging)
	}
}

func (c *WhiteList) CheckLoggingDriver(logging *types.LoggingConfig) {
	if !c.supported("services.logging.driver") && logging.Driver != "" {
		logging.Driver = ""
		c.error("services.logging.driver")
	}
}

func (c *WhiteList) CheckLoggingOptions(logging *types.LoggingConfig) {
	if !c.supported("services.logging.options") && len(logging.Options) != 0 {
		logging.Options = nil
		c.error("services.logging.options")
	}
}

func (c *WhiteList) CheckMemLimit(service *types.ServiceConfig) {
	if !c.supported("services.mem_limit") && service.MemLimit != 0 {
		service.MemLimit = 0
		c.error("services.mem_limit")
	}
}

func (c *WhiteList) CheckMemReservation(service *types.ServiceConfig) {
	if !c.supported("services.mem_reservation") && service.MemReservation != 0 {
		service.MemReservation = 0
		c.error("services.mem_reservation")
	}
}

func (c *WhiteList) CheckMemSwapLimit(service *types.ServiceConfig) {
	if !c.supported("services.memswap_limit") && service.MemSwapLimit != 0 {
		service.MemSwapLimit = 0
		c.error("services.memswap_limit")
	}
}

func (c *WhiteList) CheckMemSwappiness(service *types.ServiceConfig) {
	if !c.supported("services.mem_swappiness") && service.MemSwappiness != 0 {
		service.MemSwappiness = 0
		c.error("services.mem_swappiness")
	}
}

func (c *WhiteList) CheckMacAddress(service *types.ServiceConfig) {
	if !c.supported("services.mac_address") && service.MacAddress != "" {
		service.MacAddress = ""
		c.error("services.mac_address")
	}
}

func (c *WhiteList) CheckNet(service *types.ServiceConfig) {
	if !c.supported("services.net") && service.Net != "" {
		service.Net = ""
		c.error("services.net")
	}
}

func (c *WhiteList) CheckNetworkMode(service *types.ServiceConfig) {
	if !c.supported("services.network_mode") && service.NetworkMode != "" {
		service.NetworkMode = ""
		c.error("services.network_mode")
	}
}

func (c *WhiteList) CheckNetworks(service *types.ServiceConfig) {
	if len(service.Networks) != 0 {
		if !c.supported("services.networks") {
			service.Networks = nil
			c.error("services.networks")
		}
		for _, n := range service.Networks {
			if n != nil {
				c.CheckNetworkAliases(n)
				c.CheckNetworkIpv4Address(n)
				c.CheckNetworkIpv6Address(n)
			}
		}
	}
}

func (c *WhiteList) CheckNetworkAliases(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.aliases") && len(n.Aliases) != 0 {
		n.Aliases = nil
		c.error("services.networks.aliases")
	}
}

func (c *WhiteList) CheckNetworkIpv4Address(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.ipv4_address") && n.Ipv4Address != "" {
		n.Ipv4Address = ""
		c.error("services.networks.ipv4_address")
	}
}

func (c *WhiteList) CheckNetworkIpv6Address(n *types.ServiceNetworkConfig) {
	if !c.supported("services.networks.ipv6_address") && n.Ipv6Address != "" {
		n.Ipv6Address = ""
		c.error("services.networks.ipv6_address")
	}
}

func (c *WhiteList) CheckOomKillDisable(service *types.ServiceConfig) {
	if !c.supported("services.oom_kill_disable") && service.OomKillDisable {
		service.OomKillDisable = false
		c.error("services.oom_kill_disable")
	}
}

func (c *WhiteList) CheckOomScoreAdj(service *types.ServiceConfig) {
	if !c.supported("services.oom_score_adj") && service.OomScoreAdj != 0 {
		service.OomScoreAdj = 0
		c.error("services.oom_score_adj")
	}
}

func (c *WhiteList) CheckPid(service *types.ServiceConfig) {
	if !c.supported("services.pid") && service.Pid != "" {
		service.Pid = ""
		c.error("services.pid")
	}
}

func (c *WhiteList) CheckPorts(service *types.ServiceConfig) {
	if len(service.Ports) != 0 {
		if !c.supported("services.ports") {
			service.Ports = nil
			c.error("services.ports")
		}
		for i, p := range service.Ports {
			c.CheckPortsMode(&p)
			c.CheckPortsTarget(&p)
			c.CheckPortsProtocol(&p)
			c.CheckPortsProtocol(&p)
			service.Ports[i] = p
		}
	}
}

func (c *WhiteList) CheckPortsMode(p *types.ServicePortConfig) {
	if !c.supported("services.ports.mode") && p.Mode != "" {
		p.Mode = ""
		c.error("services.ports.mode")
	}
}

func (c *WhiteList) CheckPortsTarget(p *types.ServicePortConfig) {
	if !c.supported("services.ports.target") && p.Target != 0 {
		p.Target = 0
		c.error("services.ports.target")
	}
}

func (c *WhiteList) CheckPortsPublished(p *types.ServicePortConfig) {
	if !c.supported("services.ports.published") && p.Published != 0 {
		p.Published = 0
		c.error("services.ports.published")
	}
}

func (c *WhiteList) CheckPortsProtocol(p *types.ServicePortConfig) {
	if !c.supported("services.ports.protocol") && p.Protocol != "" {
		p.Protocol = ""
		c.error("services.ports.protocol")
	}
}

func (c *WhiteList) CheckPrivileged(service *types.ServiceConfig) {
	if !c.supported("services.privileged") && service.Privileged {
		service.Privileged = false
		c.error("services.privileged")
	}
}

func (c *WhiteList) CheckReadOnly(service *types.ServiceConfig) {
	if !c.supported("services.read_only") && service.ReadOnly {
		service.ReadOnly = false
		c.error("services.read_only")
	}
}

func (c *WhiteList) CheckRestart(service *types.ServiceConfig) {
	if !c.supported("services.restart") && service.Restart != "" {
		service.Restart = ""
		c.error("services.restart")
	}
}

func (c *WhiteList) CheckSecrets(service *types.ServiceConfig) {
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

func (c *WhiteList) CheckFileReference(s string, config *types.FileReferenceConfig) {
	c.CheckFileReferenceSource(s, config)
	c.CheckFileReferenceTarget(s, config)
	c.CheckFileReferenceGID(s, config)
	c.CheckFileReferenceUID(s, config)
	c.CheckFileReferenceMode(s, config)
}

func (c *WhiteList) CheckFileReferenceSource(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.source", s)
	if !c.supported(k) && config.Source != "" {
		config.Source = ""
		c.error(k)
	}
}

func (c *WhiteList) CheckFileReferenceTarget(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.target", s)
	if !c.supported(k) && config.Target == "" {
		config.Target = ""
		c.error(k)
	}
}

func (c *WhiteList) CheckFileReferenceUID(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.uid", s)
	if !c.supported(k) && config.UID != "" {
		config.UID = ""
		c.error(k)
	}
}

func (c *WhiteList) CheckFileReferenceGID(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.gid", s)
	if !c.supported(k) && config.GID != "" {
		config.GID = ""
		c.error(k)
	}
}

func (c *WhiteList) CheckFileReferenceMode(s string, config *types.FileReferenceConfig) {
	k := fmt.Sprintf("%s.mode", s)
	if !c.supported(k) && config.Mode != nil {
		config.Mode = nil
		c.error(k)
	}
}

func (c *WhiteList) CheckSecurityOpt(service *types.ServiceConfig) {
	if !c.supported("services.security_opt") && len(service.SecurityOpt) != 0 {
		service.SecurityOpt = nil
		c.error("services.security_opt")
	}
}

func (c *WhiteList) CheckShmSize(service *types.ServiceConfig) {
	if !c.supported("services.shm_size") && service.ShmSize != "" {
		service.ShmSize = ""
		c.error("services.shm_size")
	}
}

func (c *WhiteList) CheckStdinOpen(service *types.ServiceConfig) {
	if !c.supported("services.stdin_open") && service.StdinOpen {
		service.StdinOpen = true
		c.error("services.stdin_open")
	}
}

func (c *WhiteList) CheckStopGracePeriod(service *types.ServiceConfig) {
	if !c.supported("services.stop_grace_period") && service.StopGracePeriod != nil {
		service.StopGracePeriod = nil
		c.error("services.stop_grace_period")
	}
}

func (c *WhiteList) CheckStopSignal(service *types.ServiceConfig) {
	if !c.supported("services.stop_signal") && service.StopSignal != "" {
		service.StopSignal = ""
		c.error("services.stop_signal")
	}
}

func (c *WhiteList) CheckSysctls(service *types.ServiceConfig) {
	if !c.supported("services.sysctls") && len(service.Sysctls) != 0 {
		service.Sysctls = nil
		c.error("services.sysctls")
	}
}

func (c *WhiteList) CheckTmpfs(service *types.ServiceConfig) {
	if !c.supported("services.tmpfs") && len(service.Tmpfs) != 0 {
		service.Tmpfs = nil
		c.error("services.tmpfs")
	}
}

func (c *WhiteList) CheckTty(service *types.ServiceConfig) {
	if !c.supported("services.tty") && service.Tty {
		service.Tty = false
		c.error("services.tty")
	}
}

func (c *WhiteList) CheckUlimits(service *types.ServiceConfig) {
	if !c.supported("services.ulimits") && len(service.Ulimits) != 0 {
		service.Ulimits = nil
		c.error("services.ulimits")
	}
}

func (c *WhiteList) CheckUser(service *types.ServiceConfig) {
	if !c.supported("services.user") && service.User != "" {
		service.User = ""
		c.error("services.user")
	}
}

func (c *WhiteList) CheckUserNSMode(service *types.ServiceConfig) {
	if !c.supported("services.userns_mode") && service.UserNSMode != "" {
		service.UserNSMode = ""
		c.error("services.userns_mode")
	}
}

func (c *WhiteList) CheckUts(service *types.ServiceConfig) {
	if !c.supported("services.build") && service.Uts != "" {
		service.Uts = ""
		c.error("services.uts")
	}
}

func (c *WhiteList) CheckVolumeDriver(service *types.ServiceConfig) {
	if !c.supported("services.volume_driver") && service.VolumeDriver != "" {
		service.VolumeDriver = ""
		c.error("services.volume_driver")
	}
}

func (c *WhiteList) CheckVolumes(service *types.ServiceConfig) {
	if len(service.Volumes) != 0 {
		if !c.supported("services.volumes") {
			service.Volumes = nil
			c.error("services.volumes")
		}
		for i, v := range service.Volumes {
			c.CheckVolumesSource(&v)
			c.CheckVolumesTarget(&v)
			c.CheckVolumesReadOnly(&v)
			switch v.Type {
			case types.VolumeTypeBind:
				c.CheckVolumesBind(v.Bind)
			case types.VolumeTypeVolume:
				c.CheckVolumesVolume(v.Volume)
			case types.VolumeTypeTmpfs:
				c.CheckVolumesTmpfs(v.Tmpfs)
			}
			service.Volumes[i] = v
		}
	}
}

func (c *WhiteList) CheckVolumesSource(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.source") && config.Source != "" {
		config.Source = ""
		c.error("services.volumes.source")
	}
}

func (c *WhiteList) CheckVolumesTarget(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.target") && config.Target != "" {
		config.Target = ""
		c.error("services.volumes.target")
	}
}

func (c *WhiteList) CheckVolumesReadOnly(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.read_only") && config.ReadOnly {
		config.ReadOnly = false
		c.error("services.volumes.read_only")
	}
}

func (c *WhiteList) CheckVolumesConsistency(config *types.ServiceVolumeConfig) {
	if !c.supported("services.volumes.consistency") && config.Consistency != "" {
		config.Consistency = ""
		c.error("services.volumes.consistency")
	}
}

func (c *WhiteList) CheckVolumesBind(config *types.ServiceVolumeBind) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.bind.propagation") && config.Propagation != "" {
		config.Propagation = ""
		c.error("services.volumes.bind.propagation")
	}
}

func (c *WhiteList) CheckVolumesVolume(config *types.ServiceVolumeVolume) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.nocopy") && config.NoCopy {
		config.NoCopy = false
		c.error("services.volumes.nocopy")
	}
}

func (c *WhiteList) CheckVolumesTmpfs(config *types.ServiceVolumeTmpfs) {
	if config == nil {
		return
	}
	if !c.supported("services.volumes.tmpfs.size") && config.Size != 0 {
		config.Size = 0
		c.error("services.volumes.tmpfs.size")
	}
}

func (c *WhiteList) CheckVolumesFrom(service *types.ServiceConfig) {
	if !c.supported("services.volumes_from") && len(service.VolumesFrom) != 0 {
		service.VolumesFrom = nil
		c.error("services.volumes_from")
	}
}

func (c *WhiteList) CheckWorkingDir(service *types.ServiceConfig) {
	if !c.supported("services.working_dir") && service.WorkingDir != "" {
		service.WorkingDir = ""
		c.error("services.working_dir")
	}
}
