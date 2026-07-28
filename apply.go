package forma

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ChangeKind classifies a planned destination change.
type ChangeKind uint8

const (
	ChangeCreate ChangeKind = iota + 1
	ChangeOverwrite
	ChangeUnchanged
	ChangeConflict
	ChangeUpdatePermissions
)

// Change describes the effect of one artifact at a destination.
type Change struct {
	Path   Path
	Kind   ChangeKind
	Reason string
}

// Changes is an ordered collection of destination changes.
type Changes []Change

// Inspect compares a plan with a destination without writing.
func Inspect(
	ctx context.Context,
	plan Plan,
	destination string,
) (Changes, error) {
	if ctx == nil {
		return nil, errors.New("forma: nil context")
	}
	if destination == "" {
		return nil, fmt.Errorf("%w: empty destination", ErrInvalidPath)
	}
	root, err := resolveDestination(destination)
	if err != nil {
		conflict := Change{
			Kind:   ChangeConflict,
			Reason: err.Error(),
		}
		return Changes{conflict}, errors.Join(
			ErrInspectConflict,
			err,
		)
	}

	if err := validateDirectoryPath(root); err != nil {
		conflict := Change{
			Kind:   ChangeConflict,
			Reason: err.Error(),
		}
		return Changes{conflict}, errors.Join(ErrInspectConflict, err)
	}

	changes := make(Changes, 0, len(plan.artifacts))
	var conflicts []error
	for _, artifact := range plan.artifacts {
		if err := ctx.Err(); err != nil {
			return changes, err
		}
		change, inspectErr := inspectArtifact(root, artifact)
		changes = append(changes, change)
		if inspectErr != nil {
			conflicts = append(conflicts, inspectErr)
		}
	}
	if len(conflicts) != 0 {
		joined := append([]error{ErrInspectConflict}, conflicts...)
		return changes, errors.Join(joined...)
	}
	return changes, nil
}

func inspectArtifact(root string, artifact Artifact) (Change, error) {
	change := Change{Path: artifact.Path()}
	target := filepath.Join(root, filepath.FromSlash(artifact.Path().String()))

	if err := inspectParents(root, target); err != nil {
		change.Kind = ChangeConflict
		change.Reason = err.Error()
		return change, artifactPathError(artifact, err)
	}

	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		change.Kind = ChangeCreate
		return change, nil
	}
	if err != nil {
		change.Kind = ChangeConflict
		change.Reason = err.Error()
		return change, artifactPathError(artifact, err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		change.Kind = ChangeConflict
		change.Reason = "destination is a symlink"
		return change, artifactPathError(artifact, ErrPathCollision)
	}

	switch artifact.Kind() {
	case ArtifactDirectory:
		if !info.IsDir() {
			change.Kind = ChangeConflict
			change.Reason = "a non-directory exists at the directory path"
			return change, artifactPathError(artifact, ErrPathCollision)
		}
		if info.Mode().Perm() != artifact.Mode().Perm() {
			change.Kind = ChangeUpdatePermissions
			change.Reason = "permissions differ"
			return change, nil
		}
		change.Kind = ChangeUnchanged
		return change, nil

	case ArtifactFile:
		if !info.Mode().IsRegular() {
			change.Kind = ChangeConflict
			change.Reason = "a non-regular file exists at the file path"
			return change, artifactPathError(artifact, ErrPathCollision)
		}
		existing, readErr := os.ReadFile(target)
		if readErr != nil {
			change.Kind = ChangeConflict
			change.Reason = readErr.Error()
			return change, artifactPathError(artifact, readErr)
		}
		file := artifact.(FileArtifact)
		sameContents := bytes.Equal(existing, file.Contents())
		sameMode := info.Mode().Perm() == artifact.Mode().Perm()
		if sameContents && sameMode {
			change.Kind = ChangeUnchanged
			return change, nil
		}
		change.Kind = ChangeOverwrite
		change.Reason = "contents or mode differ"
		return change, nil
	default:
		change.Kind = ChangeConflict
		change.Reason = "unknown artifact kind"
		return change, artifactPathError(artifact, ErrInvalidComponent)
	}
}

func artifactPathError(artifact Artifact, err error) *PathError {
	return &PathError{
		Path:   artifact.Path().String(),
		Origin: artifactOrigin(artifact),
		Err:    err,
	}
}

// inspectParents rejects symlinks and non-directories between root and target.
// A missing parent is safe because Apply will create it.
func inspectParents(root, target string) error {
	relative, err := filepath.Rel(root, filepath.Dir(target))
	if err != nil {
		return err
	}
	if relative == "." {
		return nil
	}
	current := root
	for _, part := range splitPath(relative) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return statErr
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: parent %q is a symlink", ErrPathCollision, part)
		}
		if !info.IsDir() {
			return fmt.Errorf("%w: parent %q is not a directory", ErrPathCollision, part)
		}
	}
	return nil
}

func validateDirectoryPath(value string) error {
	current, parts := absolutePathParts(value)
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect destination component: %w", err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf(
				"%w: destination component %q is a symlink",
				ErrPathCollision,
				current,
			)
		}
		if !info.IsDir() {
			return fmt.Errorf(
				"%w: destination component %q is not a directory",
				ErrPathCollision,
				current,
			)
		}
	}
	return nil
}

func resolveDestination(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve destination: %w", err)
	}

	current := absolute
	var missing []string
	for {
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if info.Mode()&fs.ModeSymlink != 0 {
				return "", fmt.Errorf(
					"%w: destination ancestor %q is a symlink",
					ErrPathCollision,
					current,
				)
			}
			if !info.IsDir() {
				return "", fmt.Errorf(
					"%w: destination ancestor %q is not a directory",
					ErrPathCollision,
					current,
				)
			}
			break
		}
		if !errors.Is(statErr, fs.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf(
				"%w: no existing destination ancestor",
				ErrInvalidPath,
			)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}

	canonical, err := filepath.EvalSymlinks(current)
	if err != nil {
		return "", fmt.Errorf("resolve destination ancestor: %w", err)
	}
	for i := len(missing) - 1; i >= 0; i-- {
		canonical = filepath.Join(canonical, missing[i])
	}
	return canonical, nil
}

func ensureDirectoryPath(value string, mode fs.FileMode) error {
	current, parts := absolutePathParts(value)
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&fs.ModeSymlink != 0 {
				return fmt.Errorf(
					"%w: directory component %q is a symlink",
					ErrPathCollision,
					current,
				)
			}
			if !info.IsDir() {
				return fmt.Errorf(
					"%w: directory component %q is not a directory",
					ErrPathCollision,
					current,
				)
			}
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.Mkdir(current, mode.Perm()); err != nil {
			return err
		}
		if err := os.Chmod(current, mode.Perm()); err != nil {
			return err
		}
	}
	return nil
}

func absolutePathParts(value string) (string, []string) {
	volume := filepath.VolumeName(value)
	root := volume + string(filepath.Separator)
	relative := strings.TrimPrefix(value, root)
	if volume == "" {
		root = string(filepath.Separator)
		relative = strings.TrimPrefix(value, root)
	}
	return root, splitPath(relative)
}

func splitPath(value string) []string {
	var parts []string
	for value != "." && value != string(filepath.Separator) && value != "" {
		parent, leaf := filepath.Split(value)
		if leaf != "" {
			parts = append([]string{leaf}, parts...)
		}
		value = filepath.Clean(parent)
	}
	return parts
}

// ExistingFilePolicy controls handling of differing regular files.
type ExistingFilePolicy uint8

const (
	FailOnExisting ExistingFilePolicy = iota
	OverwriteExisting
)

// ApplyOptions configures application behavior.
type ApplyOptions struct {
	ExistingFilePolicy ExistingFilePolicy
}

// ApplyStatus records the result for one artifact.
type ApplyStatus uint8

const (
	ApplyCreated ApplyStatus = iota + 1
	ApplyOverwritten
	ApplyUnchanged
	ApplyFailed
	ApplyPermissionsUpdated
)

// ApplyOutcome records the path and status of one artifact.
type ApplyOutcome struct {
	Path   Path
	Status ApplyStatus
}

// ApplyResult reports outcomes completed before Apply returned.
type ApplyResult struct {
	Outcomes []ApplyOutcome
}

// Apply preflights and writes a plan to destination.
func Apply(
	ctx context.Context, plan Plan, destination string, options ApplyOptions,
) (ApplyResult, error) {
	var result ApplyResult
	if ctx == nil {
		return result, errors.New("forma: nil context")
	}
	if options.ExistingFilePolicy != FailOnExisting &&
		options.ExistingFilePolicy != OverwriteExisting {
		return result, fmt.Errorf("%w: unknown existing-file policy", ErrApply)
	}

	changes, err := Inspect(ctx, plan, destination)
	if err != nil {
		return result, errors.Join(ErrApply, err)
	}
	if options.ExistingFilePolicy == FailOnExisting {
		var existing []error
		for _, change := range changes {
			if change.Kind == ChangeOverwrite {
				existing = append(existing, &PathError{
					Path: change.Path.String(),
					Err:  fmt.Errorf("%w: file already exists", ErrInspectConflict),
				})
			}
		}
		if len(existing) != 0 {
			return result, errors.Join(append([]error{ErrApply}, existing...)...)
		}
	}

	root, err := resolveDestination(destination)
	if err != nil {
		return result, errors.Join(ErrApply, err)
	}
	if err := validateDirectoryPath(root); err != nil {
		return result, errors.Join(ErrApply, err)
	}
	if err := ensureDirectoryPath(root, 0o755); err != nil {
		createErr := fmt.Errorf("create destination: %w", err)
		return result, errors.Join(ErrApply, createErr)
	}

	changeByPath := make(map[string]Change, len(changes))
	for _, change := range changes {
		changeByPath[change.Path.String()] = change
	}

	for _, artifact := range plan.artifacts {
		change := changeByPath[artifact.Path().String()]
		if change.Kind == ChangeUnchanged {
			result.Outcomes = append(
				result.Outcomes,
				ApplyOutcome{Path: artifact.Path(), Status: ApplyUnchanged},
			)
			continue
		}
		if err := ctx.Err(); err != nil {
			result.Outcomes = append(
				result.Outcomes,
				ApplyOutcome{Path: artifact.Path(), Status: ApplyFailed},
			)
			return result, errors.Join(ErrApply, err)
		}

		target := filepath.Join(root, filepath.FromSlash(artifact.Path().String()))
		if err := applyArtifact(ctx, root, target, artifact); err != nil {
			result.Outcomes = append(
				result.Outcomes,
				ApplyOutcome{Path: artifact.Path(), Status: ApplyFailed},
			)
			return result, errors.Join(ErrApply, &PathError{
				Path: artifact.Path().String(), Origin: artifactOrigin(artifact), Err: err,
			})
		}
		status := ApplyCreated
		if change.Kind == ChangeOverwrite {
			status = ApplyOverwritten
		} else if change.Kind == ChangeUpdatePermissions {
			status = ApplyPermissionsUpdated
		}
		result.Outcomes = append(
			result.Outcomes,
			ApplyOutcome{Path: artifact.Path(), Status: status},
		)
	}
	return result, nil
}

func applyArtifact(
	ctx context.Context, root, target string, artifact Artifact,
) error {
	if err := inspectParents(root, target); err != nil {
		return err
	}

	if artifact.Kind() == ArtifactDirectory {
		if err := ensureDirectoryPath(target, 0o755); err != nil {
			return err
		}
		info, err := os.Lstat(target)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return ErrPathCollision
		}
		return os.Chmod(target, artifact.Mode().Perm())
	}

	if err := ensureDirectoryPath(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := inspectParents(root, target); err != nil {
		return err
	}

	pattern := "." + filepath.Base(target) + ".tmp-*"
	temp, err := os.CreateTemp(filepath.Dir(target), pattern)
	if err != nil {
		return err
	}
	tempName := temp.Name()
	committed := false
	defer func() {
		_ = temp.Close()
		if !committed {
			_ = os.Remove(tempName)
		}
	}()

	file := artifact.(FileArtifact)
	if _, err := temp.Write(file.Contents()); err != nil {
		return err
	}
	if err := temp.Chmod(artifact.Mode().Perm()); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := inspectParents(root, target); err != nil {
		return err
	}
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&fs.ModeSymlink != 0 {
			return ErrPathCollision
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(tempName, target); err != nil {
		return err
	}
	committed = true
	return nil
}
