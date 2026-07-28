package gocli

import (
	"testing"

	"github.com/hossein1376/forma"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplate(t *testing.T) {
	require := require.New(t)
	plan, err := forma.PlanFor(
		Template{},
		Input{
			Name:       "tool",
			ModulePath: "example.com/tool",
			GitHubCI:   true,
		},
	)
	require.NoError(err)
	assertPaths(
		t,
		plan,
		".github/workflows/test.yml",
		"cmd/tool/main.go",
		"go.mod",
	)
}

func TestTemplateRejectsInvalidInput(t *testing.T) {
	assert := assert.New(t)
	_, err := forma.PlanFor(
		Template{},
		Input{Name: "../tool", ModulePath: "bad path"},
	)
	assert.Error(err)
}

func assertPaths(t *testing.T, plan forma.Plan, wanted ...string) {
	t.Helper()
	assert := assert.New(t)
	paths := make(map[string]bool)
	for _, artifact := range plan.Artifacts() {
		paths[artifact.Path().String()] = true
	}
	for _, path := range wanted {
		assert.True(paths[path], "plan does not contain %q", path)
	}
}
