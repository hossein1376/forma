// Package gocli provides Forma's reference Go command template.
package gocli

import (
	"fmt"
	"regexp"

	"github.com/hossein1376/forma"
	"github.com/hossein1376/forma/templates/internal/templateutil"
)

// Input configures a generated Go command.
type Input struct {
	Name       string
	ModulePath string
	GitHubCI   bool
}

// Template generates a dependency-free Go command.
type Template struct{}

// Metadata describes the template.
func (Template) Metadata() forma.Metadata {
	return forma.Metadata{
		Name:        "go-cli",
		Description: "A small dependency-free Go command",
	}
}

var commandNamePattern = regexp.MustCompile(
	`^[A-Za-z][A-Za-z0-9_-]*$`,
)

// Build validates input and returns the command's component tree.
func (Template) Build(input Input) (forma.Component, error) {
	if !commandNamePattern.MatchString(input.Name) {
		return nil, fmt.Errorf("name must match %s", commandNamePattern.String())
	}
	if err := templateutil.ValidateModulePath(input.ModulePath); err != nil {
		return nil, err
	}

	return forma.Group(
		forma.File(
			"go.mod", templateutil.GoMod(input.ModulePath),
		),
		forma.Dir(
			"cmd",
			forma.Dir(
				input.Name,
				forma.File(
					"main.go",
					forma.Text(
						mainTemplate,
						forma.Values{"name": input.Name},
					),
				),
			),
		),
		forma.File(
			"README.md",
			forma.Text(readmeTemplate, forma.Values{"name": input.Name}),
		),
		forma.When(
			input.GitHubCI,
			forma.Dir(
				".github",
				forma.Dir(
					"workflows",
					forma.File("test.yml", templateutil.Workflow()),
				),
			),
		),
	), nil
}

const mainTemplate = `package main

import "fmt"

func main() {
	fmt.Println("${name}")
}
`

const readmeTemplate = `# ${name}

Run the command:

` + "```sh" + `
go run ./cmd/${name}
` + "```" + `
`
