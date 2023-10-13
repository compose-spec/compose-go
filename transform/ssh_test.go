package transform

import (
	"testing"

	"github.com/compose-spec/compose-go/tree"
	"gotest.tools/v3/assert"
)

func TestSSHConfig(t *testing.T) {
	ssh, err := transformSSH([]any{
		"default",
		"foo=bar",
	}, tree.NewPath("test"))
	assert.NilError(t, err)
	assert.DeepEqual(t, ssh, map[string]any{
		"default": nil,
		"foo":     "bar",
	})
}
