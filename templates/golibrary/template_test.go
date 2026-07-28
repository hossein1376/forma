package golibrary

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
			Name:       "example",
			ModulePath: "example.com/library",
			GitHubCI:   true,
		},
	)
	require.NoError(err)
	assertPaths(
		t,
		plan,
		".github/workflows/test.yml",
		"example.go",
		"example_test.go",
		"go.mod",
	)
}

func TestTemplateRejectsInvalidPackageName(t *testing.T) {
	assert := assert.New(t)
	_, err := forma.PlanFor(
		Template{},
		Input{
			Name:       "bad-name",
			ModulePath: "example.com/library",
		},
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
