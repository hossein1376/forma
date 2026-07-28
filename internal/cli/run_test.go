package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateGoHTTPService(t *testing.T) {
	assert := assert.New(t)
	destination := filepath.Join(t.TempDir(), "orders")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(
		context.Background(),
		[]string{
			"create",
			"go-http-service",
			"--name",
			"orders",
			"--module",
			"example.com/orders",
			"--yes",
			destination,
		},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	)
	assert.Equal(exitOK, code, "stderr:\n%s", stderr.String())

	mainPath := filepath.Join(
		destination,
		"cmd",
		"orders",
		"main.go",
	)
	assert.FileExists(mainPath)
}

func TestPlanAllTemplates(t *testing.T) {
	templates := []struct {
		name        string
		serviceName string
	}{
		{name: "go-http-service", serviceName: "service"},
		{name: "go-cli", serviceName: "command"},
		{name: "go-library", serviceName: "library"},
	}

	for _, template := range templates {
		t.Run(template.name, func(t *testing.T) {
			assert := assert.New(t)
			code, stdout, stderr := invoke(
				[]string{
					"plan",
					template.name,
					"--name",
					template.serviceName,
					"--module",
					"example.com/project",
				},
				"",
			)
			assert.Equal(exitOK, code, "stderr:\n%s", stderr)
			assert.Contains(stdout, "go.mod")
		})
	}
}

func TestCreateRequiresConfirmation(t *testing.T) {
	assert := assert.New(t)
	destination := filepath.Join(t.TempDir(), "project")
	code, stdout, stderr := invoke(
		[]string{
			"create",
			"go-cli",
			"--name",
			"command",
			"--module",
			"example.com/command",
			destination,
		},
		"no\n",
	)
	assert.Equal(exitOK, code, "stderr:\n%s", stderr)
	assert.Contains(stdout, "Cancelled.")
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		assert.Fail("destination was created", "error: %v", err)
	}
}

func TestInvalidInvocation(t *testing.T) {
	assert := assert.New(t)
	code, _, stderr := invoke(
		[]string{"plan", "unknown"},
		"",
	)
	assert.Equal(exitUsage, code)
	assert.Contains(stderr, "unknown template")
}

func TestTemplatesRejectInvalidModulePath(t *testing.T) {
	templates := []string{
		"go-http-service",
		"go-cli",
		"go-library",
	}
	for _, template := range templates {
		t.Run(template, func(t *testing.T) {
			assert := assert.New(t)
			code, _, stderr := invoke(
				[]string{
					"plan",
					template,
					"--name",
					"project",
					"--module",
					"example.com/project@v1",
				},
				"",
			)
			assert.Equal(exitFailure, code)
			assert.Contains(stderr, "invalid module path")
		})
	}
}

func invoke(
	args []string,
	stdin string,
) (int, string, string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(
		context.Background(),
		args,
		strings.NewReader(stdin),
		&stdout,
		&stderr,
	)
	return code, stdout.String(), stderr.String()
}
