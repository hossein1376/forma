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

func TestBuilderComposesAndRendersDeterministically(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	builder := NewBuilder()
	err := builder.Add(
		Dir("cmd", File("main.go", Text("package ${package}\n", Values{"package": "main"}))),
		File("README.md", Bytes([]byte("# Example\n"))),
	)
	require.NoError(err)

	artifacts := builder.Plan().Artifacts()
	require.Len(artifacts, 3)
	gotPaths := []string{
		artifacts[0].Path().String(),
		artifacts[1].Path().String(),
		artifacts[2].Path().String(),
	}
	wantPaths := []string{"README.md", "cmd", "cmd/main.go"}
	assert.Equal(wantPaths, gotPaths)
	gotContents := string(artifacts[2].(FileArtifact).Contents())
	assert.Equal("package main\n", gotContents)
}

func TestBuilderAddIsTransactional(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	builder := NewBuilder()
	require.NoError(builder.Add(File("kept", Bytes(nil))))
	err := builder.Add(File("new", Bytes(nil)), File("kept/child", Bytes(nil)))
	assert.ErrorIs(err, ErrPathCollision)

	artifacts := builder.Plan().Artifacts()
	require.Len(artifacts, 1)
	assert.Equal("kept", artifacts[0].Path().String())
}

func TestApplyPolicyAndIdempotence(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	ctx := context.Background()
	destination := t.TempDir()
	builder := NewBuilder()
	content := Text("hello ${name}\n", Values{"name": "world"})
	require.NoError(builder.Add(File("hello.txt", content)))
	plan := builder.Plan()

	first, err := Apply(ctx, plan, destination, ApplyOptions{})
	require.NoError(err)
	require.Len(first.Outcomes, 1)
	assert.Equal(ApplyCreated, first.Outcomes[0].Status)

	second, err := Apply(ctx, plan, destination, ApplyOptions{})
	require.NoError(err)
	require.Len(second.Outcomes, 1)
	assert.Equal(ApplyUnchanged, second.Outcomes[0].Status)

	helloPath := filepath.Join(destination, "hello.txt")
	err = os.WriteFile(helloPath, []byte("local\n"), 0o644)
	require.NoError(err)
	_, err = Apply(ctx, plan, destination, ApplyOptions{})
	assert.ErrorIs(err, ErrApply)
	got, err := os.ReadFile(filepath.Join(destination, "hello.txt"))
	require.NoError(err)
	assert.Equal("local\n", string(got))

	overwritten, err := Apply(ctx, plan, destination, ApplyOptions{ExistingFilePolicy: OverwriteExisting})
	require.NoError(err)
	require.Len(overwritten.Outcomes, 1)
	assert.Equal(ApplyOverwritten, overwritten.Outcomes[0].Status)
}

func TestInspectRejectsSymlinkParent(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation commonly requires additional privileges on Windows")
	}

	destination := t.TempDir()
	outside := t.TempDir()
	linkPath := filepath.Join(destination, "linked")
	require.NoError(os.Symlink(outside, linkPath))
	builder := NewBuilder()
	component := File("linked/file.txt", Bytes([]byte("unsafe")))
	require.NoError(builder.Add(component))

	changes, err := Inspect(context.Background(), builder.Plan(), destination)
	assert.ErrorIs(err, ErrInspectConflict)
	require.Len(changes, 1)
	assert.Equal(ChangeConflict, changes[0].Kind)
}
