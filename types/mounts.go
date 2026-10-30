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

import "fmt"

// ServiceMountConfig is a mount of a container, with the structure of the
// `docker run --mount` flag: unlike an entry of `volumes`, it leaves no latitude
// to the implementation on how the mount is made, and none of what it declares
// is a default of the short syntax. Its type comes from the volume it mirrors,
// and its options are the ones of the long syntax of `volumes`, but for:
//   - `bind.create_src`, which defaults to false like `bind-create-src` does,
//     where `bind.create_host_path` defaults to true;
//   - `bind.selinux`, that the flag has no equivalent for;
//   - `volume.driver` and `volume.driver_opts`, which only exist here.
type ServiceMountConfig struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Source      string `yaml:"source,omitempty" json:"source,omitempty"`
	Target      string `yaml:"target,omitempty" json:"target,omitempty"`
	ReadOnly    bool   `yaml:"read_only,omitempty" json:"read_only,omitempty"`
	Consistency string `yaml:"consistency,omitempty" json:"consistency,omitempty"`

	Bind   *ServiceMountBind   `yaml:"bind,omitempty" json:"bind,omitempty"`
	Volume *ServiceMountVolume `yaml:"volume,omitempty" json:"volume,omitempty"`
	Tmpfs  *ServiceVolumeTmpfs `yaml:"tmpfs,omitempty" json:"tmpfs,omitempty"`
	Image  *ServiceVolumeImage `yaml:"image,omitempty" json:"image,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

// ServiceMountBind are options for a mount of type bind
type ServiceMountBind struct {
	Propagation string `yaml:"propagation,omitempty" json:"propagation,omitempty"`
	// CreateSrc creates the source on the host when it is missing. Unlike
	// `create_host_path` of a volume, it is off unless set.
	CreateSrc bool   `yaml:"create_src,omitempty" json:"create_src,omitempty"`
	Recursive string `yaml:"recursive,omitempty" json:"recursive,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

// ServiceMountVolume are options for a mount of type volume
type ServiceMountVolume struct {
	Labels  Mapping `yaml:"labels,omitempty" json:"labels,omitempty"`
	NoCopy  bool    `yaml:"nocopy,omitempty" json:"nocopy,omitempty"`
	Subpath string  `yaml:"subpath,omitempty" json:"subpath,omitempty"`
	// Driver and DriverOpts apply to the volume created for this mount, as
	// `volume-driver` and `volume-opt` do.
	Driver     string  `yaml:"driver,omitempty" json:"driver,omitempty"`
	DriverOpts Options `yaml:"driver_opts,omitempty" json:"driver_opts,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

// ToMount returns the mount that does what the volume does, carrying the
// source and the target as they are, and with the options the volume sets or
// defaults to. It fails when the volume sets an option a mount can't carry,
// which is the SELinux relabeling of a bind mount.
func (s ServiceVolumeConfig) ToMount() (ServiceMountConfig, error) {
	m := ServiceMountConfig{
		Type:        s.Type,
		Source:      s.Source,
		Target:      s.Target,
		ReadOnly:    s.ReadOnly,
		Consistency: s.Consistency,
		Tmpfs:       s.Tmpfs,
		Image:       s.Image,
		Extensions:  s.Extensions,
	}
	switch {
	case s.Bind != nil:
		if s.Bind.SELinux != "" {
			return ServiceMountConfig{}, fmt.Errorf("volume %s: bind.selinux (%s) has no equivalent on a mount", s.Target, s.Bind.SELinux)
		}
		m.Bind = &ServiceMountBind{
			Propagation: s.Bind.Propagation,
			CreateSrc:   bool(s.Bind.CreateHostPath),
			Recursive:   s.Bind.Recursive,
			Extensions:  s.Bind.Extensions,
		}
	case s.Type == VolumeTypeBind:
		// a bind volume creates its host path unless it says not to
		m.Bind = &ServiceMountBind{CreateSrc: true}
	}
	if s.Volume != nil {
		m.Volume = &ServiceMountVolume{
			Labels:     s.Volume.Labels,
			NoCopy:     s.Volume.NoCopy,
			Subpath:    s.Volume.Subpath,
			Extensions: s.Volume.Extensions,
		}
	}
	return m, nil
}
