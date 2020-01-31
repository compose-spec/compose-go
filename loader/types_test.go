package loader

import (
	"encoding/json"
	"os"
	"testing"

	yaml "gopkg.in/yaml.v2"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestMarshallConfig(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)
	cfg := fullExampleConfig(workingDir, homeDir)
	expected := fullExampleYAML(workingDir, homeDir)

	actual, err := yaml.Marshal(cfg)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(expected, string(actual)))

	// Make sure the expected still
	dict, err := ParseYAML([]byte("version: '3.7'\n" + expected))
	assert.NilError(t, err)
	_, err = Load(buildConfigDetails(dict, map[string]string{}))
	assert.NilError(t, err)
}

func TestJSONMarshallConfig(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)

	cfg := fullExampleConfig(workingDir, homeDir)
	expected := fullExampleJSON(workingDir, homeDir)

	actual, err := json.MarshalIndent(cfg, "", "  ")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(expected, string(actual)))

	dict, err := ParseYAML([]byte(expected))
	assert.NilError(t, err)
	_, err = Load(buildConfigDetails(dict, map[string]string{}))
	assert.NilError(t, err)
}
