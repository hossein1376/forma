# Library API

Package `forma` separates scaffold description from filesystem mutation:

1. Components describe files and directories.
2. A builder validates components and produces an immutable plan.
3. `Inspect` compares the plan with a destination.
4. `Apply` performs the approved changes.

## Components

Create files and directories with:

```go
forma.Dir(path, children...)
forma.File(path, content, options...)
forma.CopyFile(path, source, sourcePath, options...)
forma.Group(components...)
forma.When(condition, components...)
```

Components are sealed. Applications extend Forma by writing ordinary functions
that return `forma.Component`.

Paths must be clean, relative, slash-separated paths. Forma rejects empty paths,
absolute paths, traversal, backslashes, duplicates, and paths that collide with
a file required as a parent.

Files default to mode `0644`. Use `forma.WithFileMode` to select another regular
file permission. Directories use `0755`.

## Content

`forma.Bytes` copies raw bytes into a component.

`forma.Text` performs only `${name}` substitution:

```go
forma.Text(
	"module ${module}\n",
	forma.Values{"module": "example.com/project"},
)
```

Use `$$` for a literal dollar sign. Missing values, malformed placeholders, and
unused values are errors. Text content has no loops, conditions, functions,
includes, or filesystem access.

Structured helpers are also available:

```go
forma.JSON(value)
forma.YAML(value)
forma.GoSource(fileSet, astFile)
```

JSON and YAML output is deterministic. `GoSource` formats an `*ast.File` with
`go/format` using its associated `*token.FileSet`.

## Builder and plans

```go
builder := forma.NewBuilder()
if err := builder.Add(component); err != nil {
	return err
}
plan := builder.Plan()
```

`Builder.Add` is transactional. If any component in a call is invalid, none of
that call's artifacts are retained.

`Plan.Artifacts` returns artifacts in normalized path order. Returned slices and
file contents are defensive copies.

For typed reusable templates:

```go
type Template[I any] interface {
	Metadata() forma.Metadata
	Build(input I) (forma.Component, error)
}

plan, err := forma.PlanFor(template, input)
```

Input validation belongs in the template's `Build` method.

## Inspection

```go
changes, err := forma.Inspect(ctx, plan, destination)
```

Each change is classified as:

- `ChangeCreate`
- `ChangeOverwrite`
- `ChangeUnchanged`
- `ChangeConflict`
- `ChangeUpdatePermissions`

An identical regular file is unchanged. A differing regular file is an
overwrite. File/directory collisions, symlinks, and special files are
conflicts. A planned directory with different permission bits is a permission
update.

Inspection performs no writes.

## Application

```go
result, err := forma.Apply(
	ctx,
	plan,
	destination,
	forma.ApplyOptions{},
)
```

The zero-value policy, `FailOnExisting`, rejects differing existing files. Set
`ExistingFilePolicy` to `OverwriteExisting` to replace regular files.

Forma creates missing parents and writes files through temporary siblings before
renaming them. Unrelated destination entries are preserved. Rollback is not
provided; on failure, `ApplyResult` reports completed and failed artifacts.
New directories receive exact `0755` permissions regardless of process umask.
Existing unrelated parent directory permissions are preserved.

## Errors

Use `errors.Is` with these categories:

- `ErrInvalidPath`
- `ErrInvalidComponent`
- `ErrRender`
- `ErrDuplicatePath`
- `ErrPathCollision`
- `ErrInspectConflict`
- `ErrApply`

Path-specific failures use `*forma.PathError`, which includes the destination
path and component origin when available.
