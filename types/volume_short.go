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
	"strings"
)

// ShortSyntax renders a volume in the short syntax, `[SOURCE:]TARGET[:OPTIONS]`,
// leaving out the options that are the default (`rw`).
// It reports false when the short syntax can't say what the volume says, which
// is when no spelling of it parses back as the same volume: something it has no
// place for (a tmpfs, an image, a subpath, labels, a consistency, extensions...),
// or something it would read otherwise (a colon in a path, a bind source that
// is not a path, a bind that doesn't create its host path).
func (v ServiceVolumeConfig) ShortSyntax() (string, bool) {
	if !fitsShortSyntax(v) {
		return "", false
	}

	var options []string
	if v.ReadOnly {
		options = append(options, "ro")
	}
	if v.Bind != nil && v.Bind.SELinux != "" {
		options = append(options, v.Bind.SELinux)
	}
	if v.Bind != nil && v.Bind.Propagation != "" {
		options = append(options, v.Bind.Propagation)
	}
	if v.Volume != nil && v.Volume.NoCopy {
		options = append(options, "nocopy")
	}

	// an anonymous volume has no place for options
	spec := v.Target
	if v.Source != "" {
		spec = v.Source + ":" + v.Target
		if len(options) > 0 {
			spec += ":" + strings.Join(options, ",")
		}
	} else if len(options) > 0 {
		return "", false
	}

	parsed, err := ParseVolume(spec)
	if err != nil || !sameVolume(parsed, v) {
		return "", false
	}
	return spec, true
}

// IsShortSyntaxExpressible tells whether the short syntax can say what the volume
// says, so that a volume which isn't can only be written, and understood, as a mount.
func (v ServiceVolumeConfig) IsShortSyntaxExpressible() bool {
	_, ok := v.ShortSyntax()
	return ok
}

// fitsShortSyntax checks what has no place in the short syntax, whatever the
// spelling: the type, and the attributes and options it does not know.
func fitsShortSyntax(v ServiceVolumeConfig) bool {
	if v.Target == "" || v.Consistency != "" || len(v.Extensions) > 0 {
		return false
	}
	if v.Tmpfs != nil || v.Image != nil {
		return false
	}
	switch v.Type {
	case VolumeTypeBind:
		if v.Source == "" {
			return false
		}
		if !isZeroVolume(v.Volume) {
			return false
		}
		if b := v.Bind; b != nil && (!bool(b.CreateHostPath) || b.Recursive != "" || len(b.Extensions) > 0) {
			return false
		}
	case VolumeTypeVolume:
		if !isZeroBind(v.Bind) {
			return false
		}
		if vol := v.Volume; vol != nil && (len(vol.Labels) > 0 || vol.Subpath != "" || len(vol.Extensions) > 0) {
			return false
		}
	default:
		return false
	}
	return true
}

func isZeroBind(b *ServiceVolumeBind) bool {
	return b == nil || (b.SELinux == "" && b.Propagation == "" && b.Recursive == "" && len(b.Extensions) == 0 && !bool(b.CreateHostPath))
}

func isZeroVolume(v *ServiceVolumeVolume) bool {
	return v == nil || (!v.NoCopy && v.Subpath == "" && len(v.Labels) == 0 && len(v.Extensions) == 0)
}

// sameVolume compares what the short syntax can carry. An option that is not set
// is the same as one that is not there: the parser always sets the options of
// the type it recognizes.
func sameVolume(parsed, v ServiceVolumeConfig) bool {
	if parsed.Type != v.Type || parsed.Source != v.Source || parsed.Target != v.Target || parsed.ReadOnly != v.ReadOnly {
		return false
	}
	var pb, vb ServiceVolumeBind
	if parsed.Bind != nil {
		pb = *parsed.Bind
	}
	if v.Bind != nil {
		vb = *v.Bind
	}
	// a bind of the short syntax creates its host path, which is also the default
	// of a bind that doesn't say
	if v.Type == VolumeTypeBind {
		pb.CreateHostPath, vb.CreateHostPath = true, true
	} else {
		pb.CreateHostPath, vb.CreateHostPath = false, false
	}
	if pb.SELinux != vb.SELinux || pb.Propagation != vb.Propagation || pb.CreateHostPath != vb.CreateHostPath {
		return false
	}
	var pv, vv ServiceVolumeVolume
	if parsed.Volume != nil {
		pv = *parsed.Volume
	}
	if v.Volume != nil {
		vv = *v.Volume
	}
	return pv.NoCopy == vv.NoCopy
}

// MarshalYAML writes a volume the short syntax can say as a string, and the
// others as the long syntax, so that the short syntax is the canonical form of
// a volume and the long one only what it cannot be.
func (v ServiceVolumeConfig) MarshalYAML() (interface{}, error) {
	if spec, ok := v.ShortSyntax(); ok {
		return spec, nil
	}
	type t ServiceVolumeConfig
	return t(v), nil
}
