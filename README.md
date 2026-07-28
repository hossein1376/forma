# Forma

Forma is a small Go library for describing, inspecting, and safely applying
deterministic project scaffolds. It also includes a command-line interface with
several ready-to-use Go templates.

The library is the primary API. Components describe output without writing to
disk, plans are immutable, and filesystem changes can be inspected before they
are applied.

## Features

- Typed Go templates with no configuration DSL.
- Composable files, directories, groups, and conditional components.
- Restricted `${name}` text substitution.
- Deterministic JSON, YAML, and formatted Go source generation.
- Path validation and file/directory collision detection.
- Preflight inspection with create, overwrite, unchanged, and conflict states.
- Atomic sibling-file writes and symlink-aware destination checks.
- Context cancellation and partial apply results.

## Installation

Install the CLI:

```sh
go install github.com/hossein1376/forma/cmd/forma@latest
```

Add the library to a module:

```sh
go get github.com/hossein1376/forma
```

Forma requires Go 1.26 or newer. Generated projects use the same Go version.

## CLI quick start

Inspect a template without writing:

```sh
forma plan go-http-service --name orders --module example.com/orders
```

Create the project:

```sh
forma create go-http-service --name orders --module example.com/orders ./orders
```

Forma displays all planned changes and asks for confirmation. Use `--yes` in
non-interactive environments. Existing differing files are rejected unless
`--overwrite` is explicitly provided.

Bundled templates:

- `go-http-service`: an HTTP service with a health endpoint.
- `go-cli`: a dependency-free Go command.
- `go-library`: a tested Go package.

All templates support `--github-ci`. The HTTP service also supports `--port`
and `--docker`.

## Library quick start

```go
package main

import (
	"context"
	"log"

	"github.com/hossein1376/forma"
)

func main() {
	component := forma.Dir(
		"cmd",
		forma.Dir(
			"hello",
			forma.File(
				"main.go",
				forma.Text(
					"package ${package}\n", forma.Values{"package": "main"},
				),
			),
		),
	)

	builder := forma.NewBuilder()
	if err := builder.Add(component); err != nil {
		log.Fatal(err)
	}
	plan := builder.Plan()

	changes, err := forma.Inspect(
		context.Background(),
		plan,
		"./output",
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d change(s)", len(changes))

	_, err = forma.Apply(
		context.Background(),
		plan,
		"./output",
		forma.ApplyOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}
}
```

See [docs/API.md](docs/API.md) for the library reference and [docs/CLI.md](docs/CLI.md)
for command details.

## License

Forma is available under the [MIT License](LICENSE).
