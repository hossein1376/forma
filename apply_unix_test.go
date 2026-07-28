//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package forma

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDirectoryModesIgnoreUmask(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	originalUmask := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(originalUmask) })

	destination := filepath.Join(t.TempDir(), "project")
	builder := NewBuilder()
	require.NoError(builder.Add(File("nested/file", Bytes(nil))))

	_, err := Apply(
		context.Background(),
		builder.Plan(),
		destination,
		ApplyOptions{},
	)
	require.NoError(err)

	info, err := os.Stat(filepath.Join(destination, "nested"))
	require.NoError(err)
	assert.Equal(os.FileMode(0o755), info.Mode().Perm())
}

func TestApplyPreservesExistingParentMode(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	destination := t.TempDir()
	parent := filepath.Join(destination, "existing")
	require.NoError(os.Mkdir(parent, 0o700))
	require.NoError(os.Chmod(parent, 0o700))

	builder := NewBuilder()
	require.NoError(builder.Add(File("existing/nested/file", Bytes(nil))))
	_, err := Apply(
		context.Background(),
		builder.Plan(),
		destination,
		ApplyOptions{},
	)
	require.NoError(err)

	info, err := os.Stat(parent)
	require.NoError(err)
	assert.Equal(os.FileMode(0o700), info.Mode().Perm())
}
