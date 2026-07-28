package templateutil

import (
	"testing"

	"github.com/hossein1376/forma"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateModulePath(t *testing.T) {
	valid := []string{
		"example.com/project",
		"github.com/owner/project/v2",
	}
	for _, value := range valid {
		t.Run("valid_"+value, func(t *testing.T) {
			assert := assert.New(t)
			assert.NoError(ValidateModulePath(value))
		})
	}

	invalid := []string{
		"",
		"bad path",
		"example.com/bad\"path",
		"example.com/project@v1",
		"../project",
		"example.com//project",
	}
	for _, value := range invalid {
		t.Run("invalid_"+value, func(t *testing.T) {
			assert := assert.New(t)
			assert.Error(ValidateModulePath(value))
		})
	}
}

func TestGeneratedVersionContent(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	builder := forma.NewBuilder()
	err := builder.Add(
		forma.File("go.mod", GoMod("example.com/project")),
		forma.File("workflow.yml", Workflow()),
	)
	require.NoError(err)
	artifacts := builder.Plan().Artifacts()
	require.Len(artifacts, 2)

	for _, artifact := range artifacts {
		contents := artifact.(forma.FileArtifact).Contents()
		assert.Contains(string(contents), GoVersion)
	}
}
