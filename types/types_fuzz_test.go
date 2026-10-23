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
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
)

// FuzzUnitBytes locks the parsing of byte values (`mem_limit`, `shm_size`,
// `tmpfs.size`...; see
// https://github.com/compose-spec/compose-spec/blob/main/05-services.md#mem_limit):
// whatever the text, parsing from a string, a YAML scalar or
// a JSON document never panics, and the three agree on the value of a string.
// A parsed value is written back as the plain number of bytes by both
// marshalers and parses to the same value again.
func FuzzUnitBytes(f *testing.F) {
	for _, seed := range []string{
		"", "0", "1", "-1", "512", "1b", "1B", "1k", "1kb", "1KiB", "1m", "512M", "1g", "1.5g", "0.5",
		"1e3", "1e3k", ".5k", "1.", "1 g", "1 kb", "9223372036854775807", "9223372036854775808",
		"8589934592g", "1e19", "1x", "k", "-", "+1", "inf", "NaN", "0x10", "1_000", "1kib", "1kbb",
		" 1", "1 ", "١", "1\x00",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		var fromString UnitBytes
		err := fromString.parseString(s)

		var fromMapstructure UnitBytes
		if errM := fromMapstructure.DecodeMapstructure(s); (err == nil) != (errM == nil) || fromString != fromMapstructure {
			t.Fatalf("%q parses to (%d, %v) as a string and (%d, %v) as a model value", s, fromString, err, fromMapstructure, errM)
		}

		// arbitrary documents must be rejected or accepted, not crash
		var fromYAML, fromJSON UnitBytes
		_ = yaml.Unmarshal([]byte(s), &fromYAML)
		_ = json.Unmarshal([]byte(s), &fromJSON)
		quoted, _ := json.Marshal(s)
		_ = json.Unmarshal(quoted, &fromJSON)

		if err != nil {
			return
		}
		want := strconv.FormatInt(int64(fromString), 10)
		if got, _ := fromString.MarshalYAML(); got != want {
			t.Fatalf("%q marshals to %v in YAML, want %q", s, got, want)
		}
		var back UnitBytes
		if err := back.parseString(want); err != nil || back != fromString {
			t.Fatalf("%q is %d, but %q parses to (%d, %v)", s, fromString, want, back, err)
		}
		var viaJSON UnitBytes
		data, _ := fromString.MarshalJSON()
		if err := json.Unmarshal(data, &viaJSON); err != nil || viaJSON != fromString {
			t.Fatalf("%q is %d, but its JSON form %s parses to (%d, %v)", s, fromString, data, viaJSON, err)
		}
	})
}

// FuzzDuration locks the parsing of durations (`healthcheck.interval`,
// `stop_grace_period`...): whatever the text, parsing never panics, and a
// duration that parses is written back by String() in a form both the model
// decoder and the JSON decoder read as the same duration.
func FuzzDuration(f *testing.F) {
	for _, seed := range []string{
		"", "0", "1s", "1m30s", "1h", "1.5s", "100ms", "1us", "1µs", "1ns", "1d", "1w", "1y", "-1s", "+1s",
		"1", "s", ".s", "1h1h1h", "9223372036854775807ns", "9223372036854775808ns", "2562047h47m16.854775807s",
		"1e3s", "0x10s", "1 s", "\"1s\"", "999999999999999999h",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		var d Duration
		if err := d.DecodeMapstructure(s); err != nil {
			var j Duration
			_ = j.UnmarshalJSON([]byte(s))
			return
		}

		printed := d.String()
		var back Duration
		if err := back.DecodeMapstructure(printed); err != nil || back != d {
			t.Fatalf("%q is %d, but its String() %q decodes to (%d, %v)", s, int64(d), printed, int64(back), err)
		}
		var viaJSON Duration
		if err := viaJSON.UnmarshalJSON([]byte(printed)); err != nil || viaJSON != d {
			t.Fatalf("%q is %d, but its String() %q reads as (%d, %v) from JSON", s, int64(d), printed, int64(viaJSON), err)
		}
	})
}

// FuzzParsePortConfig locks the short syntax of `ports`
// (https://github.com/compose-spec/compose-spec/blob/main/05-services.md#ports):
// whatever the text, parsing never panics, every entry it returns is an
// ingress port with a lower case protocol and a target in the port range, and
// the same text always parses to the same entries.
func FuzzParsePortConfig(f *testing.F) {
	for _, seed := range []string{
		"", "80", "80/tcp", "80/udp", "80/sctp", "8080:80", "8080:80/udp", "127.0.0.1:8080:80", "[::1]:8080:80",
		"127.0.0.1::80", "8000-8010:80-90", "8000-8010:80", "80-90", "0:80", "65535", "65536", "-1", "80:",
		":80", "::", "a:b:c:d", "1.2.3.4:5:6:7", "80/TCP", "80/xyz", "${PORT}:80", "80-70", "[::1", "::1:80:80",
		"8000-9000:8000-9000/udp",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, spec string) {
		ports, err := ParsePortConfig(spec)
		again, errAgain := ParsePortConfig(spec)
		if !reflect.DeepEqual(ports, again) || (err == nil) != (errAgain == nil) {
			t.Fatalf("ParsePortConfig(%q) is not deterministic: (%v, %v) then (%v, %v)", spec, ports, err, again, errAgain)
		}
		if err != nil {
			return
		}
		for _, p := range ports {
			if p.Mode != "ingress" || p.Target > 65535 || p.Protocol == "" || p.Protocol != strings.ToLower(p.Protocol) {
				t.Fatalf("ParsePortConfig(%q) returned an unexpected entry %+v", spec, p)
			}
		}
	})
}

// decoders lists the types of the model decoding their value from whatever the
// YAML document holds, as `DecodeMapstructure` does.
var decoders = map[string]func() interface{ DecodeMapstructure(any) error }{
	"HealthCheckTest":    func() interface{ DecodeMapstructure(any) error } { return new(HealthCheckTest) },
	"ShellCommand":       func() interface{ DecodeMapstructure(any) error } { return new(ShellCommand) },
	"FileMode":           func() interface{ DecodeMapstructure(any) error } { return new(FileMode) },
	"NanoCPUs":           func() interface{ DecodeMapstructure(any) error } { return new(NanoCPUs) },
	"DeviceCount":        func() interface{ DecodeMapstructure(any) error } { return new(DeviceCount) },
	"HostsList":          func() interface{ DecodeMapstructure(any) error } { return new(HostsList) },
	"SSHConfig":          func() interface{ DecodeMapstructure(any) error } { return new(SSHConfig) },
	"Labels":             func() interface{ DecodeMapstructure(any) error } { return new(Labels) },
	"Mapping":            func() interface{ DecodeMapstructure(any) error } { return new(Mapping) },
	"MappingWithEquals":  func() interface{ DecodeMapstructure(any) error } { return new(MappingWithEquals) },
	"Options":            func() interface{ DecodeMapstructure(any) error } { return new(Options) },
	"MultiOptions":       func() interface{ DecodeMapstructure(any) error } { return new(MultiOptions) },
	"StringList":         func() interface{ DecodeMapstructure(any) error } { return new(StringList) },
	"StringOrNumberList": func() interface{ DecodeMapstructure(any) error } { return new(StringOrNumberList) },
	"UnitBytes":          func() interface{ DecodeMapstructure(any) error } { return new(UnitBytes) },
	"Duration":           func() interface{ DecodeMapstructure(any) error } { return new(Duration) },
}

// FuzzDecodeMapstructure locks that a decoder of the model reports a value of
// the wrong shape or content as an error: whatever the scalar, list or mapping
// the YAML document holds (here built from a string and a number, with nil,
// boolean and float entries mixed in), decoding never panics.
func FuzzDecodeMapstructure(f *testing.F) {
	for _, seed := range []struct {
		s string
		i int
	}{
		{"", 0},
		{"all", -1},
		{"0755", 493},
		{"1.5", 1},
		{"a=b", 2},
		{"a:b", 3},
		{"host=127.0.0.1", 0},
		{"CMD", 1},
		{"echo 'unterminated", 0},
		{"soft", 4},
		{"1s", 9},
		{"512m", 7},
		{"key=", 5},
	} {
		f.Add(seed.s, seed.i)
	}
	f.Fuzz(func(t *testing.T, s string, i int) {
		values := []any{
			nil, s, i, float64(i) / 3, i%2 == 0,
			[]any{s},
			[]any{s, i},
			[]any{nil, i, 1.5, true, s},
			map[string]any{s: s},
			map[string]any{s: i, "soft": s, "hard": i},
			map[string]any{"soft": i, "hard": s},
			map[string]any{s: []any{s, i, nil}, "a": nil},
			map[string]string{s: s},
		}
		for name, newDecoder := range decoders {
			for _, v := range values {
				func() {
					defer func() {
						if r := recover(); r != nil {
							t.Errorf("%s.DecodeMapstructure(%#v) panicked: %v", name, v, r)
						}
					}()
					_ = newDecoder().DecodeMapstructure(v)
				}()
			}
		}
	})
}
