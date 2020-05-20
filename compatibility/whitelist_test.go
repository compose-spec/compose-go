package compatibility

import (
	"testing"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestWhiteList(t *testing.T) {
	var checker Checker = &WhiteList{
		Supported: []string{
			"network_mode",
			"privileged",
			"networks",
		},
	}
	dict, err := loader.ParseYAML([]byte(`
version: "3"
services:
  foo:
    image: busybox
    network_mode: host
    privileged: true
    mac_address: "a:b:c:d"
`))
	assert.NilError(t, err)

	project, err := loader.Load(types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "filename.yml", Config: dict},
		},
	})
	assert.NilError(t, err)

	checker.Check(project)
	errors := checker.Errors()
	assert.Check(t, len(errors) == 1)
	assert.Check(t, errdefs.IsUnsupportedError(errors[0]))
	assert.Equal(t, errors[0].Error(), "mac_address: unsupported attribute")

	service, err := project.GetService("foo")
	assert.NilError(t, err)
	assert.Check(t, service.MacAddress == "")
}
