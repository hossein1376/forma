package forma

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderRejectsInvalidPaths(t *testing.T) {
	paths := []string{
		"",
		".",
		"..",
		"../file",
		"/absolute",
		"dir//file",
		"dir/./file",
		`dir\file`,
		"file\x00name",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			assert := assert.New(t)
			builder := NewBuilder()
			err := builder.Add(File(path, Bytes(nil)))
			assert.ErrorIs(err, ErrInvalidPath)
		})
	}
}

func TestBuilderRejectsDuplicateAndCollidingPaths(t *testing.T) {
	tests := []struct {
		name       string
		components []Component
		want       error
	}{
		{
			name: "duplicate",
			components: []Component{
				File("same", Bytes(nil)),
				File("same", Bytes(nil)),
			},
			want: ErrDuplicatePath,
		},
		{
			name: "file used as parent",
			components: []Component{
				File("parent", Bytes(nil)),
				File("parent/child", Bytes(nil)),
			},
			want: ErrPathCollision,
		},
		{
			name: "file replaces required parent",
			components: []Component{
				File("parent/child", Bytes(nil)),
				File("parent", Bytes(nil)),
			},
			want: ErrPathCollision,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)
			builder := NewBuilder()
			err := builder.Add(test.components...)
			assert.ErrorIs(err, test.want)
		})
	}
}

func TestCopyFileAndModes(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	source := fstest.MapFS{
		"source.txt": &fstest.MapFile{
			Data: []byte("copied"),
			Mode: 0o600,
		},
	}
	builder := NewBuilder()
	err := builder.Add(
		CopyFile(
			"destination.txt",
			source,
			"source.txt",
			WithFileMode(0o640),
		),
	)
	require.NoError(err)

	artifacts := builder.Plan().Artifacts()
	require.Len(artifacts, 1)
	artifact := artifacts[0]
	assert.Equal(fs.FileMode(0o640), artifact.Mode())
	file := artifact.(FileArtifact)
	assert.Equal("copied", string(file.Contents()))
}

func TestPlanIsDefensivelyCopied(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	input := []byte("content")
	builder := NewBuilder()
	require.NoError(builder.Add(File("file", Bytes(input))))
	plan := builder.Plan()

	artifacts := plan.Artifacts()
	contents := artifacts[0].(FileArtifact).Contents()
	contents[0] = 'X'
	artifacts[0] = nil

	fresh := plan.Artifacts()
	require.NotNil(fresh[0])
	assert.Equal("content", string(fresh[0].(FileArtifact).Contents()))
}

type testTemplate struct {
	err error
}

func (testTemplate) Metadata() Metadata {
	return Metadata{Name: "test"}
}

func (template testTemplate) Build(string) (Component, error) {
	if template.err != nil {
		return nil, template.err
	}
	return File("value", Bytes(nil)), nil
}

func TestPlanFor(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	plan, err := PlanFor(testTemplate{}, "input")
	require.NoError(err)
	assert.Len(plan.Artifacts(), 1)

	buildErr := errors.New("build failed")
	_, err = PlanFor(testTemplate{err: buildErr}, "input")
	assert.ErrorIs(err, buildErr)
}
