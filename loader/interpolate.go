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

package loader

import (
	interp "github.com/compose-spec/compose-go/interpolation"
)

var interpolateTypeCastMapping = map[interp.Path]string{
	servicePath("configs", interp.PathMatchList, "mode"):             "!!int",
	servicePath("cpu_count"):                                         "!!int",
	servicePath("cpu_percent"):                                       "!!float",
	servicePath("cpu_period"):                                        "!!int",
	servicePath("cpu_quota"):                                         "!!int",
	servicePath("cpu_rt_period"):                                     "!!int",
	servicePath("cpu_rt_runtime"):                                    "!!int",
	servicePath("cpus"):                                              "!!float",
	servicePath("cpu_shares"):                                        "!!int",
	servicePath("init"):                                              "!!bool",
	servicePath("deploy", "replicas"):                                "!!int",
	servicePath("deploy", "update_config", "parallelism"):            "!!int",
	servicePath("deploy", "update_config", "max_failure_ratio"):      "!!float",
	servicePath("deploy", "rollback_config", "parallelism"):          "!!int",
	servicePath("deploy", "rollback_config", "max_failure_ratio"):    "!!float",
	servicePath("deploy", "restart_policy", "max_attempts"):          "!!int",
	servicePath("deploy", "placement", "max_replicas_per_node"):      "!!int",
	servicePath("healthcheck", "retries"):                            "!!int",
	servicePath("healthcheck", "disable"):                            "!!bool",
	servicePath("mem_limit"):                                         "!!int",
	servicePath("mem_reservation"):                                   "!!int",
	servicePath("memswap_limit"):                                     "!!int",
	servicePath("mem_swappiness"):                                    "!!int",
	servicePath("oom_kill_disable"):                                  "!!bool",
	servicePath("oom_score_adj"):                                     "!!int",
	servicePath("pids_limit"):                                        "!!int",
	servicePath("ports", interp.PathMatchList, "target"):             "!!int",
	servicePath("privileged"):                                        "!!bool",
	servicePath("read_only"):                                         "!!bool",
	servicePath("scale"):                                             "!!int",
	servicePath("secrets", interp.PathMatchList, "mode"):             "!!int",
	servicePath("shm_size"):                                          "!!str",
	servicePath("stdin_open"):                                        "!!bool",
	servicePath("stop_grace_period"):                                 "!!str",
	servicePath("tty"):                                               "!!bool",
	servicePath("ulimits", interp.PathMatchAll):                      "!!int",
	servicePath("ulimits", interp.PathMatchAll, "hard"):              "!!int",
	servicePath("ulimits", interp.PathMatchAll, "soft"):              "!!int",
	servicePath("volumes", interp.PathMatchList, "read_only"):        "!!bool",
	servicePath("volumes", interp.PathMatchList, "volume", "nocopy"): "!!bool",
	servicePath("volumes", interp.PathMatchList, "tmpfs", "size"):    "!!int",
	iPath("networks", interp.PathMatchAll, "external"):               "!!bool",
	iPath("networks", interp.PathMatchAll, "internal"):               "!!bool",
	iPath("networks", interp.PathMatchAll, "attachable"):             "!!bool",
	iPath("networks", interp.PathMatchAll, "enable_ipv6"):            "!!bool",
	iPath("volumes", interp.PathMatchAll, "external"):                "!!bool",
	iPath("secrets", interp.PathMatchAll, "external"):                "!!bool",
	iPath("configs", interp.PathMatchAll, "external"):                "!!bool",
}

func iPath(parts ...string) interp.Path {
	return interp.NewPath(parts...)
}

func servicePath(parts ...string) interp.Path {
	return iPath(append([]string{"services", interp.PathMatchAll}, parts...)...)
}
