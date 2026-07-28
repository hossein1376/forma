# Command-line interface

The Forma CLI is a thin wrapper around the public library API.

## Commands

Display a plan without reading or writing a destination:

```text
forma plan <template> [options]
```

Inspect and create a project:

```text
forma create <template> [options] <destination>
```

`create` displays destination changes before writing. It prompts for
confirmation unless `--yes` is supplied.

## Common options

- `--name`: generated project, command, package, or service name.
- `--module`: generated Go module path.
- `--github-ci`: add a GitHub Actions test workflow.
- `--yes`: skip confirmation for `create`.
- `--overwrite`: replace differing regular files for `create`.

Module paths are validated with Go's module path rules before any files are
planned.

## Templates

### `go-http-service`

Generates:

- A command under `cmd/<name>`.
- An HTTP server and health endpoint.
- A server test and README.
- An optional Dockerfile with `--docker`.
- An optional workflow with `--github-ci`.

Use `--port` to select the listen port. It defaults to `8080`.

### `go-cli`

Generates a dependency-free command under `cmd/<name>`, a README, and an
optional workflow.

### `go-library`

Generates a package, test, README, and optional workflow. Its name must be a
valid lowercase Go package identifier.

## Existing destinations

Unrelated files are preserved. Identical generated files are reported as
unchanged and are not rewritten.

Differing regular files require `--overwrite`. Symlinks, special files, and
file/directory collisions are always rejected.

Permission-only directory changes are displayed as `chmod` and do not require
`--overwrite`.

## Exit codes

- `0`: success or user cancellation.
- `1`: template, inspection, or application failure.
- `2`: invalid command-line usage.
