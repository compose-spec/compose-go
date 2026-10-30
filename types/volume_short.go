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
// place for (a tmpfs, an image, a subpath, labels, a consistency...), or something
// it would read otherwise (a colon in a path, a bind source that
// is not a path, a bind that doesn't create its host path).
// The extensions (x-*) are not counted: they are for the tools that read the model, and
// are no part of what the volume says to the engine. They are lost in the short syntax,
// which MarshalYAML takes into account.
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

// fitsShortSyntax checks what has no place in the short syntax, whatever the
// spelling: the type, and the attributes and options it does not know.
func fitsShortSyntax(v ServiceVolumeConfig) bool {
	if v.Target == "" || v.Consistency != "" {
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
		if b := v.Bind; b != nil && (!bool(b.CreateHostPath) || b.Recursive != "") {
			return false
		}
	case VolumeTypeVolume:
		// the short syntax puts the SELinux and propagation options of a volume in
		// its bind, as the parser does; create_host_path is of no use to a volume,
		// but the loader defaults it to true like it does for any bind
		if b := v.Bind; b != nil && b.Recursive != "" {
			return false
		}
		if vol := v.Volume; vol != nil && (len(vol.Labels) > 0 || vol.Subpath != "") {
			return false
		}
	default:
		return false
	}
	return true
}

func isZeroVolume(v *ServiceVolumeVolume) bool {
	return v == nil || (!v.NoCopy && v.Subpath == "" && len(v.Labels) == 0)
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

// readsBackWithItsBlocks tells whether the short syntax reads back with the option
// blocks that the volume has, no more and no less, and the options they default
// to: the JSON of a service, which its hash is computed from, would not be the
// same otherwise. A `type: volume` without `volume` reads back with an empty one,
// an anonymous volume of one or two characters reads back without one, and the
// bind options of a named volume read back with create_host_path set, which are
// the same volumes but not the same JSON. Such a volume is written in the long
// syntax so that it stays the same when it is written and read again.
func (v ServiceVolumeConfig) readsBackWithItsBlocks(spec string) bool {
	parsed, err := ParseVolume(spec)
	if err != nil {
		return false
	}
	if (parsed.Volume != nil) != (v.Volume != nil) || (parsed.Bind != nil) != (v.Bind != nil) {
		return false
	}
	// the loader sets create_host_path on every bind it reads, the one of a named
	// volume as well, and the short syntax has no word to turn it off
	return v.Bind == nil || bool(v.Bind.CreateHostPath)
}

// hasExtensions tells whether the volume, or one of its options, has extensions.
func (v ServiceVolumeConfig) hasExtensions() bool {
	return len(v.Extensions) > 0 ||
		(v.Bind != nil && len(v.Bind.Extensions) > 0) ||
		(v.Volume != nil && len(v.Volume.Extensions) > 0)
}

// MarshalYAML writes a volume the short syntax can say as a string, and the
// others as the long syntax, so that the short syntax is the canonical form of
// a volume and the long one only what it cannot be. A volume that has extensions
// is one of the others: the short syntax has no place for them.
func (v ServiceVolumeConfig) MarshalYAML() (interface{}, error) {
	if spec, ok := v.ShortSyntax(); ok && !v.hasExtensions() && v.readsBackWithItsBlocks(spec) {
		return spec, nil
	}
	type t ServiceVolumeConfig
	return t(v), nil
}
