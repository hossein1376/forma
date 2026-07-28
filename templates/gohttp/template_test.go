package gohttp

import (
	"testing"

	"github.com/hossein1376/forma"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateOptions(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	plan, err := forma.PlanFor(
		Template{},
		Input{
			Name:       "service",
			ModulePath: "example.com/service",
			Port:       9090,
			Docker:     true,
			GitHubCI:   true,
		},
	)
	require.NoError(err)
	paths := make(map[string]bool)
	for _, artifact := range plan.Artifacts() {
		paths[artifact.Path().String()] = true
	}
	for _, path := range []string{
		".github/workflows/test.yml",
		"Dockerfile",
		"cmd/service/main.go",
		"internal/http/server.go",
	} {
		assert.True(paths[path], "plan does not contain %q", path)
	}
}

func TestTemplateRejectsInvalidPort(t *testing.T) {
	assert := assert.New(t)
	_, err := forma.PlanFor(
		Template{},
		Input{
			Name:       "service",
			ModulePath: "example.com/service",
			Port:       0,
		},
	)
	assert.Error(err)
}
