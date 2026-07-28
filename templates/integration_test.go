package templates_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hossein1376/forma"
	"github.com/hossein1376/forma/templates/gocli"
	"github.com/hossein1376/forma/templates/gohttp"
	"github.com/hossein1376/forma/templates/golibrary"
	"github.com/stretchr/testify/require"
)

func TestGeneratedProjects(t *testing.T) {
	tests := []struct {
		name string
		plan func(*testing.T) forma.Plan
	}{
		{
			name: "go-http-service",
			plan: httpPlan,
		},
		{
			name: "go-cli",
			plan: cliPlan,
		},
		{
			name: "go-library",
			plan: libraryPlan,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require := require.New(t)
			destination := filepath.Join(t.TempDir(), "project")
			_, err := forma.Apply(
				context.Background(),
				test.plan(t),
				destination,
				forma.ApplyOptions{},
			)
			require.NoError(err)

			command := exec.Command("go", "test", "./...")
			command.Dir = destination
			command.Env = append(os.Environ(), "GOWORK=off")
			output, err := command.CombinedOutput()
			require.NoError(err, "go test output:\n%s", output)
		})
	}
}

func httpPlan(t *testing.T) forma.Plan {
	t.Helper()
	require := require.New(t)
	plan, err := forma.PlanFor(
		gohttp.Template{},
		gohttp.Input{
			Name:       "service",
			ModulePath: "example.com/service",
			Port:       8080,
		},
	)
	require.NoError(err)
	return plan
}

func cliPlan(t *testing.T) forma.Plan {
	t.Helper()
	require := require.New(t)
	plan, err := forma.PlanFor(
		gocli.Template{},
		gocli.Input{
			Name:       "command",
			ModulePath: "example.com/command",
		},
	)
	require.NoError(err)
	return plan
}

func libraryPlan(t *testing.T) forma.Plan {
	t.Helper()
	require := require.New(t)
	plan, err := forma.PlanFor(
		golibrary.Template{},
		golibrary.Input{Name: "library", ModulePath: "example.com/library"},
	)
	require.NoError(err)
	return plan
}
