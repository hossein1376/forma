package forma

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectClassifiesChanges(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	destination := t.TempDir()
	require.NoError(os.WriteFile(
		filepath.Join(destination, "same"), []byte("same"), 0o644,
	))
	require.NoError(os.WriteFile(
		filepath.Join(destination, "different"), []byte("old"), 0o644,
	))

	builder := NewBuilder()
	err := builder.Add(
		File("new", Bytes([]byte("new"))),
		File("same", Bytes([]byte("same"))),
		File("different", Bytes([]byte("new"))),
	)
	require.NoError(err)

	changes, err := Inspect(context.Background(), builder.Plan(), destination)
	require.NoError(err)
	got := make(map[string]ChangeKind, len(changes))
	for _, change := range changes {
		got[change.Path.String()] = change.Kind
	}
	want := map[string]ChangeKind{
		"new":       ChangeCreate,
		"same":      ChangeUnchanged,
		"different": ChangeOverwrite,
	}
	assert.Equal(want, got)
}

func TestInspectRejectsFileDirectoryCollision(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	destination := t.TempDir()
	require.NoError(os.WriteFile(
		filepath.Join(destination, "directory"), []byte("file"), 0o644,
	))

	builder := NewBuilder()
	require.NoError(builder.Add(Dir("directory")))
	changes, err := Inspect(context.Background(), builder.Plan(), destination)
	assert.ErrorIs(err, ErrInspectConflict)
	require.Len(changes, 1)
	assert.Equal(ChangeConflict, changes[0].Kind)
}

func TestApplyHonorsCancellation(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	builder := NewBuilder()
	require.NoError(builder.Add(
		File("file", Bytes([]byte("content"))),
	))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Apply(ctx, builder.Plan(), t.TempDir(), ApplyOptions{})
	assert.ErrorIs(err, ErrApply)
	assert.ErrorIs(err, context.Canceled)
}

func TestApplyCreatesImplicitParentsAndMode(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	destination := t.TempDir()
	builder := NewBuilder()
	require.NoError(builder.Add(
		File(
			"nested/path/file",
			Bytes([]byte("content")),
			WithFileMode(0o600),
		),
	))

	_, err := Apply(context.Background(), builder.Plan(), destination, ApplyOptions{})
	require.NoError(err)
	info, err := os.Stat(filepath.Join(destination, "nested", "path", "file"))
	require.NoError(err)
	assert.Equal(os.FileMode(0o600), info.Mode().Perm())
}

func TestInspectRejectsSymlinkAboveMissingRoot(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require privileges")
	}

	parent := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(parent, "link")
	require.NoError(os.Symlink(outside, link))
	destination := filepath.Join(link, "missing", "project")

	builder := NewBuilder()
	require.NoError(builder.Add(File("file", Bytes(nil))))
	_, err := Inspect(context.Background(), builder.Plan(), destination)

	assert.ErrorIs(err, ErrInspectConflict)
	assert.NoFileExists(filepath.Join(outside, "missing", "project", "file"))
}

func TestApplyUpdatesPlannedDirectoryPermissions(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	destination := t.TempDir()
	directory := filepath.Join(destination, "planned")
	require.NoError(os.Mkdir(directory, 0o700))
	require.NoError(os.Chmod(directory, 0o700))

	builder := NewBuilder()
	require.NoError(builder.Add(Dir("planned")))
	changes, err := Inspect(context.Background(), builder.Plan(), destination)
	require.NoError(err)
	require.Len(changes, 1)
	assert.Equal(ChangeUpdatePermissions, changes[0].Kind)

	result, err := Apply(
		context.Background(),
		builder.Plan(),
		destination,
		ApplyOptions{},
	)
	require.NoError(err)
	require.Len(result.Outcomes, 1)
	assert.Equal(ApplyPermissionsUpdated, result.Outcomes[0].Status)
	info, err := os.Stat(directory)
	require.NoError(err)
	assert.Equal(os.FileMode(0o755), info.Mode().Perm())
}
