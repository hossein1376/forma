package templateutil

import (
	"fmt"

	"golang.org/x/mod/module"

	"github.com/hossein1376/forma"
)

const GoVersion = "1.26"

func ValidateModulePath(value string) error {
	if err := module.CheckPath(value); err != nil {
		return fmt.Errorf("invalid module path %q: %w", value, err)
	}
	return nil
}

func GoMod(modulePath string) forma.Content {
	return forma.Text(
		goModTemplate,
		forma.Values{"module": modulePath, "go_version": GoVersion},
	)
}

func Workflow() forma.Content {
	return forma.Text(workflowTemplate, forma.Values{"go_version": GoVersion})
}

const goModTemplate = `module ${module}

go ${go_version}
`

const workflowTemplate = `name: test

on:
  push:
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "${go_version}"
      - run: go test ./...
`
