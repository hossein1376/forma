# Design

Forma keeps generation deterministic and filesystem effects explicit.

## Typed templates

Templates are Go values with typed inputs. Forma intentionally has no YAML
behavior configuration, reflection-driven questionnaire, plugin loader, or
general-purpose template language.

This keeps validation in normal Go code and makes template changes visible to
the compiler.

Bundled templates share Go's canonical module-path validator and one generated
Go version constant, keeping `go.mod`, Dockerfiles, and CI workflows aligned.

## Sealed descriptions

`Component`, `Content`, `Artifact`, and option interfaces are sealed. Only Forma
can create artifact-producing primitives, so validation and immutability
invariants do not depend on third-party implementations.

Applications remain extensible by composing built-in components in functions.

## Deterministic plans

Planning validates paths, renders content, detects collisions, and sorts
artifacts by normalized destination path. Planning never writes to the
destination.

Plans expose only defensive copies. Reusing an input plan cannot change its
contents or ordering.

## Filesystem safety

Inspection rejects symlinks and non-directory parent paths. Apply repeats parent
checks before writes, creates temporary files beside their targets, sets modes,
syncs contents, and renames the temporary files.

For a missing destination, Forma requires the nearest existing ancestor to be a
real directory rather than a symlink. Earlier platform-managed ancestors are
canonicalized. These checks prevent accidental traversal; they do not defend
against a privileged process concurrently replacing path components.

The default conflict policy does not overwrite differing files. Overwrite is an
explicit opt-in and applies only to regular files.

Missing directories are created component-by-component and explicitly chmodded
so process umask cannot change their planned permissions. Existing unrelated
parent directory permissions are preserved.

Forma does not promise rollback after a partial filesystem failure. Instead,
the apply result identifies completed and failed artifacts.
