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
	"errors"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v2/errdefs"
	"github.com/compose-spec/compose-go/v2/types"
)

func TestValidateAnonymousVolume(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"myservice": {
				Name: "myservice",
				ContainerSpec: types.ContainerSpec{
					Image: "my/service",
					Volumes: []types.ServiceVolumeConfig{
						{
							Type:   types.VolumeTypeVolume,
							Target: "/use/local",
						},
					},
				},
			},
		},
	}
	err := checkConsistency(project)
	assert.NilError(t, err)
}

func TestValidateNamedVolume(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"myservice": {
				Name: "myservice",
				ContainerSpec: types.ContainerSpec{
					Image: "my/service",
					Volumes: []types.ServiceVolumeConfig{
						{
							Type:   types.VolumeTypeVolume,
							Source: "myVolume",
							Target: "/use/local",
						},
					},
				},
			},
		},
	}
	err := checkConsistency(project)
	assert.Error(t, err, `service "myservice" refers to undefined volume myVolume: invalid compose project`)

	project.Volumes = types.Volumes(map[string]types.VolumeConfig{
		"myVolume": {
			Name: "myVolume",
		},
	})
	err = checkConsistency(project)
	assert.NilError(t, err)
}

func TestValidateNoBuildNoImage(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"myservice": {
				Name: "myservice",
			},
		},
	}
	err := checkConsistency(project)
	assert.Error(t, err, `service "myservice" has neither an image nor a build context specified: invalid compose project`)
}

func TestValidateNetworkMode(t *testing.T) {
	t.Run("network_mode service fail", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services{
				"myservice1": {
					Name:          "myservice1",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
				},
				"myservice2": {
					Name: "myservice2",
					ContainerSpec: types.ContainerSpec{
						Image:       "scratch",
						NetworkMode: "service:myservice1",
					},
				},
			},
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})

	t.Run("network_mode service fail", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services{
				"myservice1": {
					Name:          "myservice1",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
				},
				"myservice2": {
					Name: "myservice2",
					ContainerSpec: types.ContainerSpec{
						Image:       "scratch",
						NetworkMode: "service:nonexistentservice",
					},
				},
			},
		}
		// the message names the service which declares the network_mode, then its target
		err := checkConsistency(project)
		assert.Error(t, err, `service "myservice2" declares unknown service "nonexistentservice" for network_mode: invalid compose project`)
	})

	t.Run("network_mode container", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services{
				"myservice1": {
					Name:          "myservice1",
					ContainerName: "mycontainer_name",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
				},
				"myservice2": {
					Name: "myservice2",
					ContainerSpec: types.ContainerSpec{
						Image:       "scratch",
						NetworkMode: "container:mycontainer_name",
					},
				},
			},
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})

	t.Run("network_mode & networks can't both be defined", func(t *testing.T) {
		project := &types.Project{
			Networks: types.Networks{"mynetwork": types.NetworkConfig{}},
			Services: types.Services{
				"myservice1": {
					Name: "myservice1",
					ContainerSpec: types.ContainerSpec{
						Image:       "scratch",
						NetworkMode: "host",
						Networks: map[string]*types.ServiceNetworkConfig{
							"mynetwork": {},
						},
					},
				},
			},
		}
		err := checkConsistency(project)
		assert.Error(t, err, "service myservice1 declares mutually exclusive `network_mode` and `networks`: invalid compose project")
	})
}

func TestValidateSecret(t *testing.T) {
	t.Run("secret set by file", func(t *testing.T) {
		project := &types.Project{
			Secrets: types.Secrets{
				"foo": types.SecretConfig{
					File: ".secret",
				},
			},
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})
	t.Run("secret set by environment", func(t *testing.T) {
		project := &types.Project{
			Secrets: types.Secrets{
				"foo": types.SecretConfig{
					Environment: "TOKEN",
				},
			},
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})
	t.Run("external secret", func(t *testing.T) {
		project := &types.Project{
			Secrets: types.Secrets{
				"foo": types.SecretConfig{
					External: true,
				},
			},
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})
	t.Run("unset secret type", func(t *testing.T) {
		project := &types.Project{
			Secrets: types.Secrets{
				"foo": types.SecretConfig{},
			},
		}
		err := checkConsistency(project)
		assert.Error(t, err, "secret \"foo\" must declare either `file` or `environment`: invalid compose project")
	})

	t.Run("service secret exist", func(t *testing.T) {
		project := &types.Project{
			Secrets: types.Secrets{
				"foo": types.SecretConfig{
					External: true,
				},
			},
			Services: types.Services{
				"myservice": {
					Name: "myservice",
					ContainerSpec: types.ContainerSpec{
						Image: "scratch",
						Secrets: []types.ServiceSecretConfig{
							{
								Source: "foo",
							},
						},
					},
				},
			},
		}
		err := checkConsistency(project)
		assert.NilError(t, err)
	})

	t.Run("service secret undefined", func(t *testing.T) {
		project := &types.Project{
			Services: types.Services{
				"myservice": {
					Name: "myservice",
					ContainerSpec: types.ContainerSpec{
						Image: "scratch",
						Secrets: []types.ServiceSecretConfig{
							{
								Source: "foo",
							},
						},
					},
				},
			},
		}
		err := checkConsistency(project)
		assert.Error(t, err, `service "myservice" refers to undefined secret foo: invalid compose project`)
	})
}

func TestValidateDependsOn(t *testing.T) {
	project := types.Project{
		Services: types.Services{
			"myservice": {
				Name:          "myservice",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
				WorkloadSpec: types.WorkloadSpec{DependsOn: map[string]types.ServiceDependency{
					"missingservice": {},
				}},
			},
		},
	}
	err := checkConsistency(&project)
	assert.Error(t, err, `service "myservice" depends on undefined service "missingservice": invalid compose project`)
}

func TestValidateServiceDependsOnJob(t *testing.T) {
	project := types.Project{
		Services: types.Services{
			"myservice": {
				Name:          "myservice",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
				WorkloadSpec: types.WorkloadSpec{DependsOn: map[string]types.ServiceDependency{
					"myjob": {},
				}},
			},
		},
		Jobs: types.Jobs{
			"myjob": {
				Name: "myjob",
				ContainerSpec: types.ContainerSpec{
					Image: "scratch",
				},
			},
		},
	}
	err := checkConsistency(&project)
	assert.Error(t, err, `service "myservice" cannot depend on job "myjob": services can only depend on other services: invalid compose project`)
}

func TestValidateJobDependsOn(t *testing.T) {
	project := types.Project{
		Services: types.Services{
			"myservice": {
				Name: "myservice",
				ContainerSpec: types.ContainerSpec{
					Image: "scratch",
				},
			},
		},
		Jobs: types.Jobs{
			// a job can depend on a service and on another job
			"myjob": {
				Name:          "myjob",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
				WorkloadSpec: types.WorkloadSpec{DependsOn: map[string]types.ServiceDependency{
					"myservice": {},
					"otherjob":  {},
				}},
			},
			"otherjob": {
				Name: "otherjob",
				ContainerSpec: types.ContainerSpec{
					Image: "scratch",
				},
			},
		},
	}
	assert.NilError(t, checkConsistency(&project))

	project.Jobs["myjob"].DependsOn["missing"] = types.ServiceDependency{}
	err := checkConsistency(&project)
	assert.Error(t, err, `job "myjob" depends on undefined service or job "missing": invalid compose project`)
}

func TestValidateServiceJobNameUniqueness(t *testing.T) {
	project := types.Project{
		Services: types.Services{
			"foo": {
				Name: "foo",
				ContainerSpec: types.ContainerSpec{
					Image: "scratch",
				},
			},
		},
		Jobs: types.Jobs{
			"foo": {
				Name: "foo",
				ContainerSpec: types.ContainerSpec{
					Image: "scratch",
				},
			},
		},
	}
	err := checkConsistency(&project)
	assert.Error(t, err, `"foo" is declared both as a service and a job: service and job names must be unique: invalid compose project`)

	// a name owned by a profile-disabled service is not available for a job either
	project.DisabledServices = types.Services{"foo": project.Services["foo"]}
	delete(project.Services, "foo")
	err = checkConsistency(&project)
	assert.Error(t, err, `"foo" is declared both as a service and a job: service and job names must be unique: invalid compose project`)
}

func TestValidateContainerName(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"myservice": {
				Name:          "myservice",
				ContainerName: "mycontainer",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
			},
			"myservice2": {
				Name:          "myservice2",
				ContainerName: "mycontainer",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
			},
		},
	}
	err := project.CheckContainerNameUnicity()
	assert.Assert(t, strings.Contains(err.Error(), `container name "mycontainer" is already in use by`))
}

func TestValidateWatch(t *testing.T) {
	t.Run("watch valid configuration", func(t *testing.T) {
		project := types.Project{
			Services: types.Services{
				"myservice": {
					Name:          "myservice",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					Develop: &types.DevelopConfig{
						Watch: []types.Trigger{
							{
								Action: types.WatchActionSync,
								Path:   "/app",
								Target: "/container/app",
							},
						},
					},
				},
			},
		}
		err := checkConsistency(&project)
		assert.NilError(t, err)
	})

	t.Run("watch missing target for sync action", func(t *testing.T) {
		project := types.Project{
			Services: types.Services{
				"myservice": {
					Name:          "myservice",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					Develop: &types.DevelopConfig{
						Watch: []types.Trigger{
							{
								Action: types.WatchActionSync,
								Path:   "/app",
							},
						},
					},
				},
			},
		}
		err := checkConsistency(&project)
		assert.Error(t, err, "services.myservice.develop.watch: target is required for non-rebuild actions: invalid compose project")
	})

	t.Run("watch missing target for sync+restart action", func(t *testing.T) {
		project := types.Project{
			Services: types.Services{
				"myservice": {
					Name:          "myservice",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					Develop: &types.DevelopConfig{
						Watch: []types.Trigger{
							{
								Action: types.WatchActionSyncRestart,
								Path:   "/app",
							},
						},
					},
				},
			},
		}
		err := checkConsistency(&project)
		assert.Error(t, err, "services.myservice.develop.watch: target is required for non-rebuild actions: invalid compose project")
	})

	t.Run("watch config valid with missing target for rebuild action", func(t *testing.T) {
		project := types.Project{
			Services: types.Services{
				"myservice": {
					Name:          "myservice",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					Develop: &types.DevelopConfig{
						Watch: []types.Trigger{
							{
								Action: types.WatchActionRebuild,
								Path:   "/app",
							},
						},
					},
				},
			},
		}
		err := checkConsistency(&project)
		assert.NilError(t, err)
	})

	t.Run("depends on disabled service", func(t *testing.T) {
		project := types.Project{
			Services: types.Services{
				"myservice": {
					Name:          "myservice",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					WorkloadSpec: types.WorkloadSpec{DependsOn: map[string]types.ServiceDependency{
						"other": {
							Required: false,
						},
					}},
				},
			},
			DisabledServices: types.Services{
				"other": {
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
				},
			},
		}
		err := checkConsistency(&project)
		assert.NilError(t, err)
	})

	t.Run("depends on unknown service", func(t *testing.T) {
		project := types.Project{
			Services: types.Services{
				"myservice": {
					Name:          "myservice",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					WorkloadSpec: types.WorkloadSpec{DependsOn: map[string]types.ServiceDependency{
						"other": {
							Required: false,
						},
					}},
				},
			},
		}
		err := checkConsistency(&project)
		assert.ErrorContains(t, err, "depends on undefined service")
	})
}

func TestValidateMountConflict(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"myservice": {
				Name: "myservice",
				ContainerSpec: types.ContainerSpec{
					Image: "scratch",
					Tmpfs: []string{
						"/foo",
						"/conflict:size=64m",
					},
					Volumes: []types.ServiceVolumeConfig{
						{
							Type:   "bind",
							Target: "/bar",
							Source: ".",
						},
						{
							Type:   "bind",
							Target: "/conflict",
							Source: ".",
						},
					},
				},
			},
		},
	}
	err := checkConsistency(project)
	assert.Error(t, err, "services.myservice.volumes[1]: target /conflict already mounted as services.myservice.tmpfs[1]: invalid compose project")
}

func TestValidateNegativeScale(t *testing.T) {
	project := &types.Project{
		Services: types.Services{
			"myservice": {
				Name:          "myservice",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
				Scale:         ptr(-1),
			},
		},
	}
	err := checkConsistency(project)
	assert.Error(t, err, "services.myservice.scale: must be greater than or equal to 0: invalid compose project")

	project = &types.Project{
		Services: types.Services{
			"myservice": {
				Name:          "myservice",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
				Deploy: &types.DeployConfig{
					Replicas: ptr(-1),
				},
			},
		},
	}
	err = checkConsistency(project)
	assert.Error(t, err, "services.myservice.deploy.replicas: must be greater than or equal to 0: invalid compose project")
}

// webService returns a project's services made of one valid service named
// "web", which mutate (when set) turns into the case under test.
func webService(mutate func(s *types.ServiceConfig)) types.Services {
	s := types.ServiceConfig{
		Name:          "web",
		ContainerSpec: types.ContainerSpec{Image: "scratch"},
	}
	if mutate != nil {
		mutate(&s)
	}
	return types.Services{"web": s}
}

// TestCheckConsistencyServiceChecks locks every consistency check applied to
// a single service: each case is either the project rejected with the exact
// error, or the nearby project that the same check must keep accepting.
func TestCheckConsistencyServiceChecks(t *testing.T) {
	invalid := ": invalid compose project"
	for _, tc := range []struct {
		name    string
		project types.Project
		err     string // exact error, empty when the project is consistent
	}{
		// a service needs something to run
		{
			name:    "build without image is enough",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) { s.Image = ""; s.Build = &types.BuildConfig{Context: "."} })},
		},
		{
			name:    "provider without image is enough",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) { s.Image = ""; s.Provider = &types.ServiceProviderConfig{Type: "model"} })},
		},

		// build: dockerfile and dockerfile_inline are mutually exclusive
		{
			name: "dockerfile and dockerfile_inline",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Build = &types.BuildConfig{Dockerfile: "Dockerfile", DockerfileInline: "FROM scratch"}
			})},
			err: `service "web" declares mutually exclusive dockerfile and dockerfile_inline` + invalid,
		},
		{
			name:    "dockerfile alone",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) { s.Build = &types.BuildConfig{Dockerfile: "Dockerfile"} })},
		},
		{
			name:    "dockerfile_inline alone",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) { s.Build = &types.BuildConfig{DockerfileInline: "FROM scratch"} })},
		},

		// build: an additional context can point to another buildable service
		{
			name: "additional context on an unknown service",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Build = &types.BuildConfig{AdditionalContexts: types.Mapping{"base": "service:ghost"}}
			})},
			err: `service "web" declares unknown service "ghost" as additional contexts base` + invalid,
		},
		{
			name: "additional context on a non-buildable service",
			project: types.Project{Services: types.Services{
				"web": {Name: "web", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{
					Build: &types.BuildConfig{AdditionalContexts: types.Mapping{"base": "service:db"}},
				}},
				"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}},
			}},
			err: `service "web" declares non-buildable service "db" as additional contexts base` + invalid,
		},
		{
			name: "additional context on a buildable service",
			project: types.Project{Services: types.Services{
				"web": {Name: "web", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{
					Build: &types.BuildConfig{AdditionalContexts: types.Mapping{"base": "service:base"}},
				}},
				"base": {Name: "base", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{Build: &types.BuildConfig{}}},
			}},
		},
		{
			name: "additional context on a disabled buildable service",
			project: types.Project{
				Services: webService(func(s *types.ServiceConfig) {
					s.Build = &types.BuildConfig{AdditionalContexts: types.Mapping{"base": "service:base"}}
				}),
				DisabledServices: types.Services{
					"base": {Name: "base", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{Build: &types.BuildConfig{}}},
				},
			},
		},
		{
			name: "additional context on a disabled non-buildable service",
			project: types.Project{
				Services: webService(func(s *types.ServiceConfig) {
					s.Build = &types.BuildConfig{AdditionalContexts: types.Mapping{"base": "service:base"}}
				}),
				DisabledServices: types.Services{
					"base": {Name: "base", ContainerSpec: types.ContainerSpec{Image: "scratch"}},
				},
			},
			err: `service "web" declares non-buildable service "base" as additional contexts base` + invalid,
		},
		{
			name: "additional context which is not a service",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Build = &types.BuildConfig{AdditionalContexts: types.Mapping{"base": "../base", "image": "docker-image://alpine"}}
			})},
		},

		// build: platforms must include the service platform
		{
			name: "build platforms without the service platform",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Platform = "linux/amd64"
				s.Build = &types.BuildConfig{Platforms: types.StringList{"linux/arm64", "linux/s390x"}}
			})},
			err: `service.build.platforms MUST include service.platform "linux/amd64"` + invalid,
		},
		{
			name: "build platforms with the service platform",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Platform = "linux/amd64"
				s.Build = &types.BuildConfig{Platforms: types.StringList{"linux/arm64", "linux/amd64"}}
			})},
		},
		{
			name: "build platforms without a service platform",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Build = &types.BuildConfig{Platforms: types.StringList{"linux/arm64"}}
			})},
		},
		{
			name: "service platform without build platforms",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Platform = "linux/amd64"
				s.Build = &types.BuildConfig{}
			})},
		},

		// networks must be declared
		{
			name: "undefined network",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Networks = map[string]*types.ServiceNetworkConfig{"front": nil}
			})},
			err: `service "web" refers to undefined network front` + invalid,
		},
		{
			name: "defined network",
			project: types.Project{
				Networks: types.Networks{"front": types.NetworkConfig{}},
				Services: webService(func(s *types.ServiceConfig) { s.Networks = map[string]*types.ServiceNetworkConfig{"front": nil} }),
			},
		},

		// healthcheck.test must start with a known keyword
		{
			name: "healthcheck test with an unknown keyword",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.HealthCheck = &types.HealthCheckConfig{Test: types.HealthCheckTest{"curl", "-f", "http://localhost"}}
			})},
			err: `healthcheck.test must start either by "CMD", "CMD-SHELL" or "NONE"` + invalid,
		},
		{
			name: "healthcheck test with CMD",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.HealthCheck = &types.HealthCheckConfig{Test: types.HealthCheckTest{"CMD", "true"}}
			})},
		},
		{
			name: "healthcheck test with CMD-SHELL",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.HealthCheck = &types.HealthCheckConfig{Test: types.HealthCheckTest{"CMD-SHELL", "true"}}
			})},
		},
		{
			name: "healthcheck test with NONE",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.HealthCheck = &types.HealthCheckConfig{Test: types.HealthCheckTest{"NONE"}}
			})},
		},
		{
			name: "healthcheck without test",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.HealthCheck = &types.HealthCheckConfig{}
			})},
		},

		// depends_on must target a service that exists (or may exist)
		{
			name: "depends on a disabled service which is required",
			project: types.Project{
				Services: webService(func(s *types.ServiceConfig) {
					s.DependsOn = types.DependsOnConfig{"db": {Required: true}}
				}),
				DisabledServices: types.Services{"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}}},
			},
			err: `service "web" depends on undefined service "db"` + invalid,
		},
		{
			name: "depends on an enabled service",
			project: types.Project{Services: types.Services{
				"web": {Name: "web", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{
					DependsOn: types.DependsOnConfig{"db": {Required: true}},
				}},
				"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}},
			}},
		},

		// network_mode: service:<name> must target an enabled service
		{
			name: "network_mode on a disabled service",
			project: types.Project{
				Services: webService(func(s *types.ServiceConfig) { s.NetworkMode = "service:db" }),
				DisabledServices: types.Services{
					"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}},
				},
			},
			// a disabled service is told apart from an unknown one
			err: `service "web" declares disabled service "db" for network_mode` + invalid,
		},

		// volumes, build secrets, configs, models and secrets must be declared
		{
			name: "undefined build secret",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Build = &types.BuildConfig{Secrets: []types.ServiceSecretConfig{{Source: "token"}}}
			})},
			err: `service "web" refers to undefined build secret token` + invalid,
		},
		{
			name: "defined build secret",
			project: types.Project{
				Secrets: types.Secrets{"token": types.SecretConfig{File: ".token"}},
				Services: webService(func(s *types.ServiceConfig) {
					s.Build = &types.BuildConfig{Secrets: []types.ServiceSecretConfig{{Source: "token"}}}
				}),
			},
		},
		{
			name: "undefined config",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Configs = []types.ServiceConfigObjConfig{{Source: "settings"}}
			})},
			err: `service "web" refers to undefined config settings` + invalid,
		},
		{
			name: "defined config",
			project: types.Project{
				Configs: types.Configs{"settings": types.ConfigObjConfig{File: "settings.yml"}},
				Services: webService(func(s *types.ServiceConfig) {
					s.Configs = []types.ServiceConfigObjConfig{{Source: "settings"}}
				}),
			},
		},
		{
			name: "undefined model",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Models = map[string]*types.ServiceModelConfig{"llm": nil}
			})},
			err: `service "web" refers to undefined model llm` + invalid,
		},
		{
			name: "defined model",
			project: types.Project{
				Models:   types.Models{"llm": types.ModelConfig{Model: "ai/smollm2"}},
				Services: webService(func(s *types.ServiceConfig) { s.Models = map[string]*types.ServiceModelConfig{"llm": nil} }),
			},
		},

		// scale and deploy.replicas, when both set, must agree and not be negative
		{
			name: "scale and deploy.replicas differ",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Scale = ptr(2)
				s.Deploy = &types.DeployConfig{Replicas: ptr(3)}
			})},
			err: "services.web: can't set distinct values on 'scale' and 'deploy.replicas'" + invalid,
		},
		{
			name: "scale and deploy.replicas agree",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Scale = ptr(2)
				s.Deploy = &types.DeployConfig{Replicas: ptr(2)}
			})},
		},
		{
			name: "scale without deploy",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Scale = ptr(0)
			})},
		},
		{
			name: "deploy.replicas without scale",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Deploy = &types.DeployConfig{Replicas: ptr(0)}
			})},
		},
		{
			// the distinct values are reported before the negative scale
			name: "negative scale and distinct deploy.replicas",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Scale = ptr(-1)
				s.Deploy = &types.DeployConfig{Replicas: ptr(2)}
			})},
			err: "services.web: can't set distinct values on 'scale' and 'deploy.replicas'" + invalid,
		},
		{
			name: "negative scale with deploy.replicas set to it",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Scale = ptr(-1)
				s.Deploy = &types.DeployConfig{Replicas: ptr(-1)}
			})},
			err: "services.web.scale: must be greater than or equal to 0" + invalid,
		},

		// cpus, mem_limit, mem_reservation and pids_limit, when also set under
		// deploy.resources, must agree with it
		{
			name: "cpus differs from deploy.resources.limits.cpus",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.CPUS = 2
				s.Deploy = &types.DeployConfig{Resources: types.Resources{Limits: &types.Resource{NanoCPUs: 1}}}
			})},
			err: "services.web: can't set distinct values on 'cpus' and 'deploy.resources.limits.cpus'" + invalid,
		},
		{
			name: "mem_limit differs from deploy.resources.limits.memory",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.MemLimit = 2048
				s.Deploy = &types.DeployConfig{Resources: types.Resources{Limits: &types.Resource{MemoryBytes: 1024}}}
			})},
			err: "services.web: can't set distinct values on 'mem_limit' and 'deploy.resources.limits.memory'" + invalid,
		},
		{
			name: "mem_reservation differs from deploy.resources.reservations.memory",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.MemReservation = 2048
				s.Deploy = &types.DeployConfig{Resources: types.Resources{Reservations: &types.Resource{MemoryBytes: 1024}}}
			})},
			err: "services.web: can't set distinct values on 'mem_reservation' and 'deploy.resources.reservations.memory'" + invalid,
		},
		{
			name: "pids_limit differs from deploy.resources.limits.pids",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.PidsLimit = 20
				s.Deploy = &types.DeployConfig{Resources: types.Resources{Limits: &types.Resource{Pids: 10}}}
			})},
			err: "services.web: can't set distinct values on 'pids_limit' and 'deploy.resources.limits.pids'" + invalid,
		},
		{
			name: "limits and reservations agree with deploy.resources",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.CPUS = 1
				s.MemLimit = 1024
				s.MemReservation = 512
				s.PidsLimit = 10
				s.Deploy = &types.DeployConfig{Resources: types.Resources{
					Limits:       &types.Resource{NanoCPUs: 1, MemoryBytes: 1024, Pids: 10},
					Reservations: &types.Resource{MemoryBytes: 512},
				}}
			})},
		},
		{
			name: "limits without deploy.resources",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.CPUS = 1
				s.MemLimit = 1024
				s.MemReservation = 512
				s.PidsLimit = 10
				s.Deploy = &types.DeployConfig{}
			})},
		},
		{
			name: "limits without deploy",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.CPUS = 1
				s.MemLimit = 1024
				s.MemReservation = 512
				s.PidsLimit = 10
			})},
		},

		// container_name is unique, so it cannot be set on several replicas
		{
			name: "container_name with scale",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.ContainerName = "web"
				s.Scale = ptr(2)
			})},
			// the message names the service, then the attribute which scales it
			err: "services.web: can't set container_name and scale as container name must be unique" + invalid,
		},
		{
			name: "container_name with deploy.replicas",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.ContainerName = "web"
				s.Deploy = &types.DeployConfig{Replicas: ptr(2)}
			})},
			err: "services.web: can't set container_name and deploy.replicas as container name must be unique" + invalid,
		},
		{
			name: "container_name with a single replica",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.ContainerName = "web"
				s.Scale = ptr(1)
			})},
		},

		// develop.watch: a target is required unless the action rebuilds or restarts
		{
			name: "watch sync+exec without target",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Develop = &types.DevelopConfig{Watch: []types.Trigger{{Action: types.WatchActionSyncExec, Path: "/app"}}}
			})},
			err: "services.web.develop.watch: target is required for non-rebuild actions" + invalid,
		},
		{
			name: "watch restart without target",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Develop = &types.DevelopConfig{Watch: []types.Trigger{{Action: types.WatchActionRestart, Path: "/app"}}}
			})},
		},
		{
			name: "watch second trigger without target",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Develop = &types.DevelopConfig{Watch: []types.Trigger{
					{Action: types.WatchActionSync, Path: "/app", Target: "/app"},
					{Action: types.WatchActionSync, Path: "/lib"},
				}}
			})},
			err: "services.web.develop.watch: target is required for non-rebuild actions" + invalid,
		},

		// a target can be mounted only once, as tmpfs or as volume
		{
			name: "tmpfs mounted twice",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Tmpfs = types.StringList{"/tmp", "/run", "/tmp:size=64m"}
			})},
			err: "services.web.tmpfs[2]: target /tmp already mounted as services.web.tmpfs[0]" + invalid,
		},
		{
			name: "volume mounted twice",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Volumes = []types.ServiceVolumeConfig{
					{Type: types.VolumeTypeBind, Source: ".", Target: "/data"},
					{Type: types.VolumeTypeBind, Source: "..", Target: "/data"},
				}
			})},
			err: "services.web.volumes[1]: target /data already mounted as services.web.volumes[0]" + invalid,
		},
		{
			name: "tmpfs and volumes on distinct targets",
			project: types.Project{Services: webService(func(s *types.ServiceConfig) {
				s.Tmpfs = types.StringList{"/tmp"}
				s.Volumes = []types.ServiceVolumeConfig{{Type: types.VolumeTypeBind, Source: ".", Target: "/data"}}
			})},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkConsistency(&tc.project)
			if tc.err == "" {
				assert.NilError(t, err)
				return
			}
			assert.Error(t, err, tc.err)
			// every consistency error tells the project is invalid
			assert.Assert(t, errors.Is(err, errdefs.ErrInvalid))
		})
	}
}

// TestCheckConsistencyScaleSetsReplicas locks that, once scale and
// deploy.replicas are known to agree, the check completes deploy.replicas
// with the scale: the project handed to the caller carries both.
func TestCheckConsistencyScaleSetsReplicas(t *testing.T) {
	project := &types.Project{Services: webService(func(s *types.ServiceConfig) {
		s.Scale = ptr(3)
		s.Deploy = &types.DeployConfig{}
	})}
	assert.NilError(t, checkConsistency(project))
	replicas := project.Services["web"].Deploy.Replicas
	assert.Assert(t, replicas != nil)
	assert.Equal(t, *replicas, 3)
}

// TestCheckConsistencyProjectChecks locks the checks that look at the project
// as a whole rather than at one service.
func TestCheckConsistencyProjectChecks(t *testing.T) {
	invalid := ": invalid compose project"
	job := types.JobConfig{Name: "migrate", ContainerSpec: types.ContainerSpec{Image: "scratch"}}
	for _, tc := range []struct {
		name    string
		project types.Project
		err     string
	}{
		{
			name: "a disabled job and an enabled service share a name",
			project: types.Project{
				Services:     webService(nil),
				DisabledJobs: types.Jobs{"web": job},
			},
			err: `"web" is declared both as a service and a job: service and job names must be unique` + invalid,
		},
		{
			name: "a disabled job and a disabled service share a name",
			project: types.Project{
				DisabledServices: webService(nil),
				DisabledJobs:     types.Jobs{"web": job},
			},
			err: `"web" is declared both as a service and a job: service and job names must be unique` + invalid,
		},
		{
			name: "a job depends on an undefined service",
			project: types.Project{
				Services: webService(nil),
				Jobs: types.Jobs{"migrate": {
					Name:          "migrate",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					WorkloadSpec:  types.WorkloadSpec{DependsOn: types.DependsOnConfig{"db": {Required: true}}},
				}},
			},
			err: `job "migrate" depends on undefined service or job "db"` + invalid,
		},
		{
			name: "a job depends on a disabled service which is required",
			project: types.Project{
				Services:         webService(nil),
				DisabledServices: types.Services{"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}}},
				Jobs: types.Jobs{"migrate": {
					Name:          "migrate",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					WorkloadSpec:  types.WorkloadSpec{DependsOn: types.DependsOnConfig{"db": {Required: true}}},
				}},
			},
			err: `job "migrate" depends on undefined service or job "db"` + invalid,
		},
		{
			name: "a job depends on a disabled service which is optional",
			project: types.Project{
				Services:         webService(nil),
				DisabledServices: types.Services{"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}}},
				Jobs: types.Jobs{"migrate": {
					Name:          "migrate",
					ContainerSpec: types.ContainerSpec{Image: "scratch"},
					WorkloadSpec:  types.WorkloadSpec{DependsOn: types.DependsOnConfig{"db": {Required: false}}},
				}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkConsistency(&tc.project)
			if tc.err == "" {
				assert.NilError(t, err)
				return
			}
			assert.Error(t, err, tc.err)
			// every consistency error tells the project is invalid
			assert.Assert(t, errors.Is(err, errdefs.ErrInvalid))
		})
	}
}

// TestCheckConsistencyOrder locks the order in which the checks run, which is
// observable because the first error met is the one returned: a project
// carrying one fault per check is repaired fault after fault, and each
// repair must reveal the next check, in turn, down to a consistent project.
// A single service is checked at a time, so that the order does not depend on
// the iteration over the services.
func TestCheckConsistencyOrder(t *testing.T) {
	invalid := ": invalid compose project"
	project := &types.Project{
		Services: types.Services{
			"web": {
				Name:          "web",
				ContainerName: "web",
				ContainerSpec: types.ContainerSpec{
					Platform:    "linux/amd64",
					NetworkMode: "host",
					Networks:    map[string]*types.ServiceNetworkConfig{"front": nil},
					Volumes: []types.ServiceVolumeConfig{
						{Type: types.VolumeTypeVolume, Source: "data", Target: "/data"},
						{Type: types.VolumeTypeBind, Source: ".", Target: "/data"},
					},
					Tmpfs:          types.StringList{"/data", "/data"},
					Configs:        []types.ServiceConfigObjConfig{{Source: "settings"}},
					Models:         map[string]*types.ServiceModelConfig{"llm": nil},
					Secrets:        []types.ServiceSecretConfig{{Source: "token"}},
					CPUS:           2,
					MemLimit:       2048,
					MemReservation: 1024,
					PidsLimit:      20,
				},
				WorkloadSpec: types.WorkloadSpec{
					Build: &types.BuildConfig{
						Dockerfile:         "Dockerfile",
						DockerfileInline:   "FROM scratch",
						AdditionalContexts: types.Mapping{"base": "service:ghost"},
						Platforms:          types.StringList{"linux/arm64"},
						Secrets:            []types.ServiceSecretConfig{{Source: "build-token"}},
					},
					HealthCheck: &types.HealthCheckConfig{Test: types.HealthCheckTest{"curl"}},
					DependsOn:   types.DependsOnConfig{"ghost": {Required: true}},
				},
				Scale: ptr(2),
				Deploy: &types.DeployConfig{
					Replicas: ptr(3),
					Resources: types.Resources{
						Limits:       &types.Resource{NanoCPUs: 1, MemoryBytes: 1024, Pids: 10},
						Reservations: &types.Resource{MemoryBytes: 512},
					},
				},
				Develop: &types.DevelopConfig{Watch: []types.Trigger{{Action: types.WatchActionSync, Path: "/app"}}},
			},
			"base": {Name: "base", ContainerSpec: types.ContainerSpec{Image: "scratch"}},
		},
		Jobs: types.Jobs{
			"web": {Name: "web", ContainerSpec: types.ContainerSpec{Image: "scratch"}},
			"migrate": {
				Name:          "migrate",
				ContainerSpec: types.ContainerSpec{Image: "scratch"},
				WorkloadSpec:  types.WorkloadSpec{DependsOn: types.DependsOnConfig{"ghost": {Required: true}}},
			},
		},
		Secrets: types.Secrets{"empty": types.SecretConfig{}},
	}
	web := func() types.ServiceConfig { return project.Services["web"] }
	update := func(mutate func(s *types.ServiceConfig)) func() {
		return func() {
			s := web()
			mutate(&s)
			project.Services["web"] = s
		}
	}

	for _, step := range []struct {
		err string // reported while the fault is there
		fix func() // repairs it, and sets up the next one when it needs more than a repair
	}{
		{
			err: `service "web" declares mutually exclusive dockerfile and dockerfile_inline` + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Build.DockerfileInline = "" }),
		},
		{
			err: `service "web" declares unknown service "ghost" as additional contexts base` + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Build.AdditionalContexts["base"] = "service:base" }),
		},
		{
			err: `service "web" declares non-buildable service "base" as additional contexts base` + invalid,
			fix: func() {
				project.Services["base"] = types.ServiceConfig{Name: "base", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{Build: &types.BuildConfig{}}}
			},
		},
		{
			err: `service.build.platforms MUST include service.platform "linux/amd64"` + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Build.Platforms = append(s.Build.Platforms, "linux/amd64") }),
		},
		{
			err: "service web declares mutually exclusive `network_mode` and `networks`" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.NetworkMode = "" }),
		},
		{
			err: `service "web" refers to undefined network front` + invalid,
			fix: func() { project.Networks = types.Networks{"front": types.NetworkConfig{}} },
		},
		{
			err: `healthcheck.test must start either by "CMD", "CMD-SHELL" or "NONE"` + invalid,
			fix: update(func(s *types.ServiceConfig) { s.HealthCheck.Test = types.HealthCheckTest{"CMD", "true"} }),
		},
		{
			err: `service "web" depends on undefined service "ghost"` + invalid,
			fix: func() {
				update(func(s *types.ServiceConfig) { s.DependsOn = types.DependsOnConfig{"migrate": {Required: true}} })()
			},
		},
		{
			// network_mode is checked after depends_on, with no networks left
			err: `service "web" cannot depend on job "migrate": services can only depend on other services` + invalid,
			fix: update(func(s *types.ServiceConfig) {
				s.DependsOn = nil
				s.Networks = nil
				s.NetworkMode = "service:ghost"
			}),
		},
		{
			err: `service "web" declares unknown service "ghost" for network_mode` + invalid,
			fix: update(func(s *types.ServiceConfig) { s.NetworkMode = "" }),
		},
		{
			err: `service "web" refers to undefined volume data` + invalid,
			fix: func() { project.Volumes = types.Volumes{"data": types.VolumeConfig{}} },
		},
		{
			err: `service "web" refers to undefined build secret build-token` + invalid,
			fix: func() { project.Secrets["build-token"] = types.SecretConfig{File: ".build-token"} },
		},
		{
			err: `service "web" refers to undefined config settings` + invalid,
			fix: func() { project.Configs = types.Configs{"settings": types.ConfigObjConfig{File: "settings.yml"}} },
		},
		{
			err: `service "web" refers to undefined model llm` + invalid,
			fix: func() { project.Models = types.Models{"llm": types.ModelConfig{Model: "ai/smollm2"}} },
		},
		{
			err: `service "web" refers to undefined secret token` + invalid,
			fix: func() { project.Secrets["token"] = types.SecretConfig{File: ".token"} },
		},
		{
			err: "services.web: can't set distinct values on 'scale' and 'deploy.replicas'" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Scale = ptr(-1); s.Deploy.Replicas = ptr(-1) }),
		},
		{
			err: "services.web.scale: must be greater than or equal to 0" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Scale = nil }),
		},
		{
			err: "services.web.deploy.replicas: must be greater than or equal to 0" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Deploy.Replicas = ptr(2) }),
		},
		{
			err: "services.web: can't set distinct values on 'cpus' and 'deploy.resources.limits.cpus'" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.CPUS = 1 }),
		},
		{
			err: "services.web: can't set distinct values on 'mem_limit' and 'deploy.resources.limits.memory'" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.MemLimit = 1024 }),
		},
		{
			err: "services.web: can't set distinct values on 'mem_reservation' and 'deploy.resources.reservations.memory'" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.MemReservation = 512 }),
		},
		{
			err: "services.web: can't set distinct values on 'pids_limit' and 'deploy.resources.limits.pids'" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.PidsLimit = 10 }),
		},
		{
			err: "services.web: can't set container_name and deploy.replicas as container name must be unique" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.ContainerName = "" }),
		},
		{
			err: "services.web.develop.watch: target is required for non-rebuild actions" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Develop.Watch[0].Target = "/app" }),
		},
		{
			err: "services.web.tmpfs[1]: target /data already mounted as services.web.tmpfs[0]" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Tmpfs = types.StringList{"/data"} }),
		},
		{
			err: "services.web.volumes[0]: target /data already mounted as services.web.tmpfs[0]" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Tmpfs = nil }),
		},
		{
			err: "services.web.volumes[1]: target /data already mounted as services.web.volumes[0]" + invalid,
			fix: update(func(s *types.ServiceConfig) { s.Volumes = s.Volumes[:1] }),
		},
		{
			// the project checks follow, on the jobs first
			err: `"web" is declared both as a service and a job: service and job names must be unique` + invalid,
			// one collision at a time, the jobs being iterated in no set order
			fix: func() {
				delete(project.Jobs, "web")
				project.DisabledJobs = types.Jobs{"base": {Name: "base", ContainerSpec: types.ContainerSpec{Image: "scratch"}}}
			},
		},
		{
			err: `"base" is declared both as a service and a job: service and job names must be unique` + invalid,
			fix: func() { delete(project.DisabledJobs, "base") },
		},
		{
			err: `job "migrate" depends on undefined service or job "ghost"` + invalid,
			fix: func() { delete(project.Jobs["migrate"].DependsOn, "ghost") },
		},
		{
			err: `secret "empty" must declare either ` + "`file` or `environment`" + invalid,
			fix: func() {
				delete(project.Secrets, "empty")
				base := project.Services["base"]
				base.DependsOn = types.DependsOnConfig{"web": {Required: true}}
				project.Services["base"] = base
				update(func(s *types.ServiceConfig) { s.DependsOn = types.DependsOnConfig{"base": {Required: true}} })()
			},
		},
		{
			err: "dependency cycle detected: base -> web -> base",
			fix: update(func(s *types.ServiceConfig) { s.DependsOn = nil }),
		},
	} {
		err := checkConsistency(project)
		assert.Error(t, err, step.err)
		// every consistency error tells the project is invalid, but the cycle
		// one which comes from the graph package
		if !strings.HasPrefix(step.err, "dependency cycle") {
			assert.Assert(t, errors.Is(err, errdefs.ErrInvalid), step.err)
		}
		step.fix()
	}
	assert.NilError(t, checkConsistency(project))
}

// TestCheckConsistencyCycle locks that a dependency cycle between services is
// reported, which graph.CheckCycle does with an error of its own.
func TestCheckConsistencyCycle(t *testing.T) {
	project := types.Project{Services: types.Services{
		"web": {Name: "web", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{
			DependsOn: types.DependsOnConfig{"db": {Required: true}},
		}},
		"db": {Name: "db", ContainerSpec: types.ContainerSpec{Image: "scratch"}, WorkloadSpec: types.WorkloadSpec{
			DependsOn: types.DependsOnConfig{"web": {Required: true}},
		}},
	}}
	assert.Error(t, checkConsistency(&project), "dependency cycle detected: db -> web -> db")
}
