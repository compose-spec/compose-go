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

package types

import (
	_ "crypto/sha256"
	"testing"

	"github.com/compose-spec/compose-go/v2/utils"
	"github.com/distribution/reference"
	"github.com/opencontainers/go-digest"
	"gotest.tools/v3/assert"
)

func Test_ApplyProfiles(t *testing.T) {
	p := makeProject()
	p, err := p.ApplyProfiles([]string{"foo"})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.ServiceNames(), []string{"service_1", "service_2", "service_6"})
	assert.DeepEqual(t, p.DisabledServiceNames(), []string{"service_3", "service_4", "service_5"})

	p, err = p.EnableServices("service_4")
	assert.NilError(t, err)

	assert.DeepEqual(t, p.ServiceNames(), []string{"service_1", "service_2", "service_4", "service_5", "service_6"})
	assert.DeepEqual(t, p.DisabledServiceNames(), []string{"service_3"})

}

func Test_WithoutUnnecessaryResources(t *testing.T) {
	p := makeProject()
	p.Networks["unused"] = NetworkConfig{}
	p.Volumes["unused"] = VolumeConfig{}
	p.Secrets["unused"] = SecretConfig{}
	p.Configs["unused"] = ConfigObjConfig{}
	p, err := p.WithoutUnnecessaryResources()
	assert.NilError(t, err)
	if _, ok := p.Networks["unused"]; ok {
		t.Fail()
	}
	if _, ok := p.Volumes["unused"]; ok {
		t.Fail()
	}
	if _, ok := p.Secrets["unused"]; ok {
		t.Fail()
	}
	if _, ok := p.Configs["unused"]; ok {
		t.Fail()
	}
}

func Test_NoProfiles(t *testing.T) {
	p := makeProject()
	p, err := p.ApplyProfiles(nil)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services), 2)
	assert.Equal(t, len(p.DisabledServices), 4)
	assert.DeepEqual(t, p.ServiceNames(), []string{"service_1", "service_6"})
}

func Test_ServiceProfiles(t *testing.T) {
	p := makeProject()
	services, err := p.GetServices("service_1", "service_2")
	assert.NilError(t, err)

	profiles := services.GetProfiles()
	assert.Equal(t, len(profiles), 1)
	assert.Equal(t, profiles[0], "foo")
}

func Test_ForServices(t *testing.T) {
	p := makeProject()
	p, err := p.ForServices([]string{"service_2"})
	assert.NilError(t, err)

	assert.DeepEqual(t, p.DisabledServiceNames(), []string{"service_3", "service_4", "service_5", "service_6"})

	// Should not load the dependency service_1 when explicitly loading service_6
	p = makeProject()
	p, err = p.ForServices([]string{"service_6"})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.DisabledServiceNames(), []string{"service_1", "service_2", "service_3", "service_4", "service_5"})
}

func Test_ForServicesIgnoreDependencies(t *testing.T) {
	p := makeProject()
	p, err := p.ForServices([]string{"service_2"}, IgnoreDependencies)
	assert.NilError(t, err)

	assert.Equal(t, len(p.DisabledServices), 5)
	service, err := p.GetService("service_2")
	assert.NilError(t, err)
	assert.Equal(t, len(service.DependsOn), 0)

	p = makeProject()
	p, err = p.ForServices([]string{"service_2", "service_3"}, IgnoreDependencies)
	assert.NilError(t, err)

	assert.Equal(t, len(p.DisabledServices), 4)
	service, err = p.GetService("service_3")
	assert.NilError(t, err)
	assert.Equal(t, len(service.DependsOn), 1)
	_, dependsOn := service.DependsOn["service_2"]
	assert.Check(t, dependsOn)
}

func Test_ForServicesCycle(t *testing.T) {
	p := makeProject()
	service := p.Services["service_1"]
	service.Links = []string{"service_2"}
	p.Services["service_1"] = service
	p, err := p.ForServices([]string{"service_2"})
	assert.NilError(t, err)
}

func makeProject() *Project {
	return &Project{
		Services: Services{
			"service_1": ServiceConfig{
				Name: "service_1",
			},
			"service_2": ServiceConfig{
				Name:      "service_2",
				Profiles:  []string{"foo"},
				DependsOn: map[string]ServiceDependency{"service_1": {Required: true}},
			},
			"service_3": ServiceConfig{
				Name:      "service_3",
				Profiles:  []string{"bar"},
				DependsOn: map[string]ServiceDependency{"service_2": {Required: true}},
			},
			"service_4": ServiceConfig{
				Name:      "service_4",
				Profiles:  []string{"zot"},
				DependsOn: map[string]ServiceDependency{"service_2": {Required: false}},
			},
			"service_5": ServiceConfig{
				Name:     "service_5",
				Profiles: []string{"zot"},
			},
			"service_6": ServiceConfig{
				Name:  "service_6",
				Links: []string{"service_1"},
			},
		},
		Networks: Networks{},
		Volumes:  Volumes{},
		Secrets:  Secrets{},
		Configs:  Configs{},
	}
}

func Test_ResolveImages(t *testing.T) {
	p := makeProject()
	resolver := func(named reference.Named) (digest.Digest, error) {
		return "sha256:1234567890123456789012345678901234567890123456789012345678901234", nil
	}

	tests := []struct {
		image    string
		resolved string
	}{
		{
			image:    "com.acme/long:tag",
			resolved: "com.acme/long:tag@sha256:1234567890123456789012345678901234567890123456789012345678901234",
		},
		{
			image:    "com.acme/long",
			resolved: "com.acme/long:latest@sha256:1234567890123456789012345678901234567890123456789012345678901234",
		},
		{
			image:    "short",
			resolved: "docker.io/library/short:latest@sha256:1234567890123456789012345678901234567890123456789012345678901234",
		},
		{
			image:    "com.acme/digested:tag@sha256:1234567890123456789012345678901234567890123456789012345678901234",
			resolved: "com.acme/digested@sha256:1234567890123456789012345678901234567890123456789012345678901234",
		},
		{
			image:    "com.acme/digested@sha256:1234567890123456789012345678901234567890123456789012345678901234",
			resolved: "com.acme/digested@sha256:1234567890123456789012345678901234567890123456789012345678901234",
		},
	}

	for _, test := range tests {
		service := p.Services["service_1"]
		service.Image = test.image
		p.Services["service_1"] = service
		p, err := p.ResolveImages(resolver)
		assert.NilError(t, err)
		assert.Equal(t, p.Services["service_1"].Image, test.resolved)
	}
}

func TestWithServices(t *testing.T) {
	p := makeProject()
	var seen []string
	p, err := p.WithServices([]string{"service_3"}, func(name string, _ ServiceConfig) error {
		seen = append(seen, name)
		return nil
	}, IncludeDependencies)
	assert.NilError(t, err)
	assert.DeepEqual(t, seen, []string{"service_1", "service_2", "service_3"})

	seen = []string{}
	p, err = p.WithServices([]string{"service_1"}, func(name string, _ ServiceConfig) error {
		seen = append(seen, name)
		return nil
	}, IncludeDependents)
	assert.NilError(t, err)
	// Order of service_3 and service_4 may change because there both depending on service_2
	assert.Check(t, utils.ArrayContains(seen, []string{"service_3", "service_4", "service_2", "service_1"}))

	seen = []string{}
	p, err = p.WithServices([]string{"service_1"}, func(name string, _ ServiceConfig) error {
		seen = append(seen, name)
		return nil
	}, IgnoreDependencies)
	assert.NilError(t, err)
	assert.DeepEqual(t, seen, []string{"service_1"})

	seen = []string{}
	p, err = p.WithServices([]string{"service_4"}, func(name string, _ ServiceConfig) error {
		seen = append(seen, name)
		return nil
	}, IncludeDependencies)
	assert.NilError(t, err)
	assert.DeepEqual(t, seen, []string{"service_1", "service_2", "service_4"})
}
