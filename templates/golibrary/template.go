// Package golibrary provides Forma's reference Go library template.
package golibrary

import (
	"fmt"
	"regexp"

	"github.com/hossein1376/forma"
	"github.com/hossein1376/forma/templates/internal/templateutil"
)

// Input configures a generated Go library.
type Input struct {
	Name       string
	ModulePath string
	GitHubCI   bool
}

// Template generates a small Go library with a test.
type Template struct{}

// Metadata describes the template.
func (Template) Metadata() forma.Metadata {
	return forma.Metadata{
		Name:        "go-library",
		Description: "A small tested Go library",
	}
}

var packageNamePattern = regexp.MustCompile(
	`^[a-z][a-z0-9_]*$`,
)

// Build validates input and returns the library's component tree.
func (Template) Build(input Input) (forma.Component, error) {
	if !packageNamePattern.MatchString(input.Name) {
		return nil, fmt.Errorf("name must match %s", packageNamePattern.String())
	}
	if err := templateutil.ValidateModulePath(input.ModulePath); err != nil {
		return nil, err
	}

	values := forma.Values{"name": input.Name}
	return forma.Group(
		forma.File(
			"go.mod", templateutil.GoMod(input.ModulePath),
		),
		forma.File(
			input.Name+".go", forma.Text(libraryTemplate, values),
		),
		forma.File(
			input.Name+"_test.go", forma.Text(testTemplate, values),
		),
		forma.File(
			"README.md", forma.Text(readmeTemplate, values),
		),
		forma.When(
			input.GitHubCI,
			forma.Dir(
				".github",
				forma.Dir(
					"workflows", forma.File("test.yml", templateutil.Workflow()),
				),
			),
		),
	), nil
}

const libraryTemplate = `// Package ${name} provides ${name} helpers.
package ${name}

// Hello returns a friendly greeting.
func Hello() string {
	return "hello"
}
`

const testTemplate = `package ${name}

import "testing"

func TestHello(t *testing.T) {
	if got := Hello(); got != "hello" {
		t.Fatalf("Hello() = %q, want %q", got, "hello")
	}
}
`

const readmeTemplate = `# ${name}

Package ${name} provides a starting point for a Go library.
`
