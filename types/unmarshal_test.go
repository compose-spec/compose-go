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
	"testing"
	"time"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// Labels

func TestUnmarshalLabels_Map(t *testing.T) {
	var labels Labels
	err := yaml.Unmarshal([]byte(`foo: bar`), &labels)
	assert.NilError(t, err)
	assert.DeepEqual(t, labels, Labels{"foo": "bar"})
}

func TestUnmarshalLabels_List(t *testing.T) {
	var labels Labels
	err := yaml.Unmarshal([]byte("- foo=bar\n- baz=qux"), &labels)
	assert.NilError(t, err)
	assert.DeepEqual(t, labels, Labels{"foo": "bar", "baz": "qux"})
}

// Mapping

func TestUnmarshalMapping_Map(t *testing.T) {
	var m Mapping
	err := yaml.Unmarshal([]byte("foo: bar\nbaz: qux"), &m)
	assert.NilError(t, err)
	assert.DeepEqual(t, m, Mapping{"foo": "bar", "baz": "qux"})
}

func TestUnmarshalMapping_MapNullValue(t *testing.T) {
	var m Mapping
	err := yaml.Unmarshal([]byte("foo:\nbaz: qux"), &m)
	assert.NilError(t, err)
	assert.DeepEqual(t, m, Mapping{"foo": "", "baz": "qux"})
}

func TestUnmarshalMapping_List(t *testing.T) {
	var m Mapping
	err := yaml.Unmarshal([]byte("- foo=bar\n- baz=qux"), &m)
	assert.NilError(t, err)
	assert.DeepEqual(t, m, Mapping{"foo": "bar", "baz": "qux"})
}

func TestUnmarshalMapping_ListNoValue(t *testing.T) {
	var m Mapping
	err := yaml.Unmarshal([]byte("- foo"), &m)
	assert.NilError(t, err)
	assert.DeepEqual(t, m, Mapping{"foo": ""})
}

// MappingWithEquals

func TestUnmarshalMappingWithEquals_Map(t *testing.T) {
	var m MappingWithEquals
	err := yaml.Unmarshal([]byte("foo: bar\nbaz:"), &m)
	assert.NilError(t, err)
	bar := "bar"
	assert.DeepEqual(t, m, MappingWithEquals{"foo": &bar, "baz": nil})
}

func TestUnmarshalMappingWithEquals_List(t *testing.T) {
	var m MappingWithEquals
	err := yaml.Unmarshal([]byte("- foo=bar\n- baz"), &m)
	assert.NilError(t, err)
	bar := "bar"
	assert.DeepEqual(t, m, MappingWithEquals{"foo": &bar, "baz": nil})
}

func TestUnmarshalMappingWithEquals_ListEmptyValue(t *testing.T) {
	var m MappingWithEquals
	err := yaml.Unmarshal([]byte("- foo="), &m)
	assert.NilError(t, err)
	empty := ""
	assert.DeepEqual(t, m, MappingWithEquals{"foo": &empty})
}

// ShellCommand

func TestUnmarshalShellCommand_String(t *testing.T) {
	var cmd ShellCommand
	err := yaml.Unmarshal([]byte(`echo "hello world"`), &cmd)
	assert.NilError(t, err)
	assert.DeepEqual(t, cmd, ShellCommand{"echo", "hello world"})
}

func TestUnmarshalShellCommand_List(t *testing.T) {
	var cmd ShellCommand
	err := yaml.Unmarshal([]byte("- echo\n- hello world"), &cmd)
	assert.NilError(t, err)
	assert.DeepEqual(t, cmd, ShellCommand{"echo", "hello world"})
}

// UnitBytes

func TestUnmarshalUnitBytes_Integer(t *testing.T) {
	var u UnitBytes
	err := yaml.Unmarshal([]byte("1024"), &u)
	assert.NilError(t, err)
	assert.Equal(t, u, UnitBytes(1024))
}

func TestUnmarshalUnitBytes_String(t *testing.T) {
	var u UnitBytes
	err := yaml.Unmarshal([]byte("1GB"), &u)
	assert.NilError(t, err)
	assert.Equal(t, u, UnitBytes(1073741824)) // docker/go-units uses binary: 1GB = 1GiB
}

// Duration

func TestUnmarshalDuration_String(t *testing.T) {
	var d Duration
	err := yaml.Unmarshal([]byte("1m30s"), &d)
	assert.NilError(t, err)
	assert.Equal(t, d, Duration(90*time.Second))
}

func TestUnmarshalDuration_Seconds(t *testing.T) {
	var d Duration
	err := yaml.Unmarshal([]byte("30s"), &d)
	assert.NilError(t, err)
	assert.Equal(t, d, Duration(30*time.Second))
}

// NanoCPUs

func TestUnmarshalNanoCPUs_Float(t *testing.T) {
	var n NanoCPUs
	err := yaml.Unmarshal([]byte("1.5"), &n)
	assert.NilError(t, err)
	assert.Equal(t, n, NanoCPUs(1.5))
}

func TestUnmarshalNanoCPUs_Float64(t *testing.T) {
	var n NanoCPUs
	err := yaml.Unmarshal([]byte("0.5"), &n)
	assert.NilError(t, err)
	assert.Equal(t, n, NanoCPUs(0.5))
}

func TestUnmarshalNanoCPUs_Integer(t *testing.T) {
	var n NanoCPUs
	err := yaml.Unmarshal([]byte("2"), &n)
	assert.NilError(t, err)
	assert.Equal(t, n, NanoCPUs(2))
}

// DeviceCount

func TestUnmarshalDeviceCount_Integer(t *testing.T) {
	var c DeviceCount
	err := yaml.Unmarshal([]byte("3"), &c)
	assert.NilError(t, err)
	assert.Equal(t, c, DeviceCount(3))
}

func TestUnmarshalDeviceCount_All(t *testing.T) {
	var c DeviceCount
	err := yaml.Unmarshal([]byte("all"), &c)
	assert.NilError(t, err)
	assert.Equal(t, c, DeviceCount(-1))
}

// HealthCheckTest

func TestUnmarshalHealthCheckTest_String(t *testing.T) {
	var h HealthCheckTest
	err := yaml.Unmarshal([]byte("curl -f http://localhost/"), &h)
	assert.NilError(t, err)
	assert.DeepEqual(t, h, HealthCheckTest{"CMD-SHELL", "curl -f http://localhost/"})
}

func TestUnmarshalHealthCheckTest_List(t *testing.T) {
	var h HealthCheckTest
	err := yaml.Unmarshal([]byte("- CMD\n- curl\n- -f\n- http://localhost/"), &h)
	assert.NilError(t, err)
	assert.DeepEqual(t, h, HealthCheckTest{"CMD", "curl", "-f", "http://localhost/"})
}

// HostsList

func TestUnmarshalHostsList_Map(t *testing.T) {
	var h HostsList
	err := yaml.Unmarshal([]byte("myhost: 192.168.1.1"), &h)
	assert.NilError(t, err)
	assert.DeepEqual(t, h, HostsList{"myhost": {"192.168.1.1"}})
}

func TestUnmarshalHostsList_ListEquals(t *testing.T) {
	var h HostsList
	err := yaml.Unmarshal([]byte("- myhost=192.168.1.1"), &h)
	assert.NilError(t, err)
	assert.DeepEqual(t, h, HostsList{"myhost": {"192.168.1.1"}})
}

func TestUnmarshalHostsList_ListColon(t *testing.T) {
	var h HostsList
	err := yaml.Unmarshal([]byte("- myhost:192.168.1.1"), &h)
	assert.NilError(t, err)
	assert.DeepEqual(t, h, HostsList{"myhost": {"192.168.1.1"}})
}

// StringList

func TestUnmarshalStringList_String(t *testing.T) {
	var s StringList
	err := yaml.Unmarshal([]byte("hello"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, StringList{"hello"})
}

func TestUnmarshalStringList_List(t *testing.T) {
	var s StringList
	err := yaml.Unmarshal([]byte("- hello\n- world"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, StringList{"hello", "world"})
}

// StringOrNumberList

func TestUnmarshalStringOrNumberList_String(t *testing.T) {
	var s StringOrNumberList
	err := yaml.Unmarshal([]byte("8080"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, StringOrNumberList{"8080"})
}

func TestUnmarshalStringOrNumberList_List(t *testing.T) {
	var s StringOrNumberList
	err := yaml.Unmarshal([]byte("- 8080\n- 9090"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, StringOrNumberList{"8080", "9090"})
}

func TestUnmarshalStringOrNumberList_MixedList(t *testing.T) {
	var s StringOrNumberList
	err := yaml.Unmarshal([]byte("- 8080\n- http"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, StringOrNumberList{"8080", "http"})
}

// Options

func TestUnmarshalOptions_Map(t *testing.T) {
	var o Options
	err := yaml.Unmarshal([]byte("foo: bar\nbaz: qux"), &o)
	assert.NilError(t, err)
	assert.DeepEqual(t, o, Options{"foo": "bar", "baz": "qux"})
}

func TestUnmarshalOptions_MapNullValue(t *testing.T) {
	var o Options
	err := yaml.Unmarshal([]byte("foo:\nbaz: qux"), &o)
	assert.NilError(t, err)
	assert.DeepEqual(t, o, Options{"foo": "", "baz": "qux"})
}

// MultiOptions

func TestUnmarshalMultiOptions_Scalar(t *testing.T) {
	var m MultiOptions
	err := yaml.Unmarshal([]byte("foo: bar\nbaz: qux"), &m)
	assert.NilError(t, err)
	assert.DeepEqual(t, m, MultiOptions{"foo": {"bar"}, "baz": {"qux"}})
}

func TestUnmarshalMultiOptions_Sequence(t *testing.T) {
	var m MultiOptions
	err := yaml.Unmarshal([]byte("foo:\n  - bar\n  - baz"), &m)
	assert.NilError(t, err)
	assert.DeepEqual(t, m, MultiOptions{"foo": {"bar", "baz"}})
}

// SSHConfig

func TestUnmarshalSSHConfig_Map(t *testing.T) {
	var s SSHConfig
	err := yaml.Unmarshal([]byte("default: /home/user/.ssh/id_rsa"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, SSHConfig{{ID: "default", Path: "/home/user/.ssh/id_rsa"}})
}

func TestUnmarshalSSHConfig_MapNoPath(t *testing.T) {
	var s SSHConfig
	err := yaml.Unmarshal([]byte("default:"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, SSHConfig{{ID: "default"}})
}

func TestUnmarshalSSHConfig_List(t *testing.T) {
	var s SSHConfig
	err := yaml.Unmarshal([]byte("- default=/home/user/.ssh/id_rsa"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, SSHConfig{{ID: "default", Path: "/home/user/.ssh/id_rsa"}})
}

func TestUnmarshalSSHConfig_ListNoPath(t *testing.T) {
	var s SSHConfig
	err := yaml.Unmarshal([]byte("- default"), &s)
	assert.NilError(t, err)
	assert.DeepEqual(t, s, SSHConfig{{ID: "default"}})
}

// FileMode

func TestUnmarshalFileMode_OctalString(t *testing.T) {
	var m FileMode
	err := yaml.Unmarshal([]byte(`"0755"`), &m)
	assert.NilError(t, err)
	assert.Equal(t, m, FileMode(0755))
}

func TestUnmarshalFileMode_Integer(t *testing.T) {
	var m FileMode
	err := yaml.Unmarshal([]byte("0755"), &m)
	assert.NilError(t, err)
	// yaml/v4 presents !!int 0755 as decimal 755, then UnmarshalYAML parses as decimal
	assert.Equal(t, m, FileMode(755))
}

// UlimitsConfig

func TestUnmarshalUlimitsConfig_Integer(t *testing.T) {
	var u UlimitsConfig
	err := yaml.Unmarshal([]byte("1024"), &u)
	assert.NilError(t, err)
	assert.Equal(t, u.Single, 1024)
	assert.Equal(t, u.Soft, 0)
	assert.Equal(t, u.Hard, 0)
}

func TestUnmarshalUlimitsConfig_Map(t *testing.T) {
	var u UlimitsConfig
	err := yaml.Unmarshal([]byte("soft: 1024\nhard: 2048"), &u)
	assert.NilError(t, err)
	assert.Equal(t, u.Single, 0)
	assert.Equal(t, u.Soft, 1024)
	assert.Equal(t, u.Hard, 2048)
}

// BuildConfig

func TestUnmarshalBuildConfig_String(t *testing.T) {
	var b BuildConfig
	err := yaml.Unmarshal([]byte("./dir"), &b)
	assert.NilError(t, err)
	assert.Equal(t, b.Context, "./dir")
}

func TestUnmarshalBuildConfig_Map(t *testing.T) {
	var b BuildConfig
	err := yaml.Unmarshal([]byte("context: ./dir\ndockerfile: Dockerfile.dev"), &b)
	assert.NilError(t, err)
	assert.Equal(t, b.Context, "./dir")
	assert.Equal(t, b.Dockerfile, "Dockerfile.dev")
}

// DependsOnConfig

func TestUnmarshalDependsOnConfig_List(t *testing.T) {
	var d DependsOnConfig
	err := yaml.Unmarshal([]byte("- db\n- redis"), &d)
	assert.NilError(t, err)
	assert.Equal(t, len(d), 2)
	assert.Equal(t, d["db"].Condition, ServiceConditionStarted)
	assert.Equal(t, d["db"].Required, true)
	assert.Equal(t, d["redis"].Condition, ServiceConditionStarted)
	assert.Equal(t, d["redis"].Required, true)
}

func TestUnmarshalDependsOnConfig_Map(t *testing.T) {
	var d DependsOnConfig
	err := yaml.Unmarshal([]byte("db:\n  condition: service_healthy"), &d)
	assert.NilError(t, err)
	assert.Equal(t, d["db"].Condition, ServiceConditionHealthy)
	assert.Equal(t, d["db"].Required, true) // default when not explicitly set
}

func TestUnmarshalDependsOnConfig_MapExplicitRequired(t *testing.T) {
	var d DependsOnConfig
	err := yaml.Unmarshal([]byte("db:\n  condition: service_healthy\n  required: false"), &d)
	assert.NilError(t, err)
	assert.Equal(t, d["db"].Condition, ServiceConditionHealthy)
	assert.Equal(t, d["db"].Required, false)
}

// EnvFile

func TestUnmarshalEnvFile_String(t *testing.T) {
	var e EnvFile
	err := yaml.Unmarshal([]byte(".env"), &e)
	assert.NilError(t, err)
	assert.Equal(t, e.Path, ".env")
	assert.Equal(t, bool(e.Required), true)
}

func TestUnmarshalEnvFile_Map(t *testing.T) {
	var e EnvFile
	err := yaml.Unmarshal([]byte("path: .env\nrequired: false"), &e)
	assert.NilError(t, err)
	assert.Equal(t, e.Path, ".env")
	assert.Equal(t, bool(e.Required), false)
}

func TestUnmarshalEnvFile_MapNoRequired(t *testing.T) {
	var e EnvFile
	err := yaml.Unmarshal([]byte("path: .env"), &e)
	assert.NilError(t, err)
	assert.Equal(t, e.Path, ".env")
	assert.Equal(t, bool(e.Required), true) // defaults to true
}

// IncludeConfig

func TestUnmarshalIncludeConfig_String(t *testing.T) {
	var ic IncludeConfig
	err := yaml.Unmarshal([]byte("docker-compose.yml"), &ic)
	assert.NilError(t, err)
	assert.DeepEqual(t, ic.Path, StringList{"docker-compose.yml"})
}

func TestUnmarshalIncludeConfig_Map(t *testing.T) {
	var ic IncludeConfig
	err := yaml.Unmarshal([]byte("path:\n  - docker-compose.yml\nproject_directory: ./subdir"), &ic)
	assert.NilError(t, err)
	assert.DeepEqual(t, ic.Path, StringList{"docker-compose.yml"})
	assert.Equal(t, ic.ProjectDirectory, "./subdir")
}

// ServiceSecretConfig

func TestUnmarshalServiceSecretConfig_String(t *testing.T) {
	var s ServiceSecretConfig
	err := yaml.Unmarshal([]byte("my_secret"), &s)
	assert.NilError(t, err)
	assert.Equal(t, s.Source, "my_secret")
}

func TestUnmarshalServiceSecretConfig_Map(t *testing.T) {
	var s ServiceSecretConfig
	err := yaml.Unmarshal([]byte("source: my_secret\ntarget: /run/secrets/my_secret"), &s)
	assert.NilError(t, err)
	assert.Equal(t, s.Source, "my_secret")
	assert.Equal(t, s.Target, "/run/secrets/my_secret")
}

// ServiceConfigObjConfig

func TestUnmarshalServiceConfigObjConfig_String(t *testing.T) {
	var c ServiceConfigObjConfig
	err := yaml.Unmarshal([]byte("my_config"), &c)
	assert.NilError(t, err)
	assert.Equal(t, c.Source, "my_config")
}

func TestUnmarshalServiceConfigObjConfig_Map(t *testing.T) {
	var c ServiceConfigObjConfig
	err := yaml.Unmarshal([]byte("source: my_config\ntarget: /etc/my_config"), &c)
	assert.NilError(t, err)
	assert.Equal(t, c.Source, "my_config")
	assert.Equal(t, c.Target, "/etc/my_config")
}

// FileReferenceConfig

func TestUnmarshalFileReferenceConfig_String(t *testing.T) {
	var f FileReferenceConfig
	err := yaml.Unmarshal([]byte("my_ref"), &f)
	assert.NilError(t, err)
	assert.Equal(t, f.Source, "my_ref")
}

func TestUnmarshalFileReferenceConfig_Map(t *testing.T) {
	var f FileReferenceConfig
	err := yaml.Unmarshal([]byte("source: my_ref\ntarget: /path/to/ref"), &f)
	assert.NilError(t, err)
	assert.Equal(t, f.Source, "my_ref")
	assert.Equal(t, f.Target, "/path/to/ref")
}

// ServicePortConfig

func TestUnmarshalServicePortConfig_String(t *testing.T) {
	var p ServicePortConfig
	err := yaml.Unmarshal([]byte(`"8080:80"`), &p)
	assert.NilError(t, err)
	assert.Equal(t, p.Target, uint32(80))
	assert.Equal(t, p.Published, "8080")
	assert.Equal(t, p.Protocol, "tcp")
}

func TestUnmarshalServicePortConfig_Map(t *testing.T) {
	var p ServicePortConfig
	err := yaml.Unmarshal([]byte("target: 80\npublished: \"8080\"\nprotocol: tcp"), &p)
	assert.NilError(t, err)
	assert.Equal(t, p.Target, uint32(80))
	assert.Equal(t, p.Published, "8080")
	assert.Equal(t, p.Protocol, "tcp")
}

func TestUnmarshalServicePortConfig_StringTargetOnly(t *testing.T) {
	var p ServicePortConfig
	err := yaml.Unmarshal([]byte(`"80"`), &p)
	assert.NilError(t, err)
	assert.Equal(t, p.Target, uint32(80))
	assert.Equal(t, p.Protocol, "tcp")
}

// ServiceVolumeConfig (map form only, string form requires ParseVolumeFunc)

func TestUnmarshalServiceVolumeConfig_Map(t *testing.T) {
	var v ServiceVolumeConfig
	err := yaml.Unmarshal([]byte("type: bind\nsource: ./data\ntarget: /data\nread_only: true"), &v)
	assert.NilError(t, err)
	assert.Equal(t, v.Type, "bind")
	assert.Equal(t, v.Source, "./data")
	assert.Equal(t, v.Target, "/data")
	assert.Equal(t, v.ReadOnly, true)
}
