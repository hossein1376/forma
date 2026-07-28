package forma

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextRendering(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		values  Values
		want    string
		wantErr bool
	}{
		{
			name:    "substitution and escaped dollar",
			pattern: "$$${name}",
			values:  Values{"name": "value"},
			want:    "$value",
		},
		{
			name:    "missing value",
			pattern: "${missing}",
			wantErr: true,
		},
		{
			name:    "unused value",
			pattern: "plain",
			values:  Values{"unused": "value"},
			wantErr: true,
		},
		{
			name:    "invalid placeholder",
			pattern: "${bad-name}",
			wantErr: true,
		},
		{
			name:    "unterminated placeholder",
			pattern: "${name",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)
			got, err := Text(test.pattern, test.values).render()
			if test.wantErr {
				assert.Error(err)
				return
			}
			require.NoError(err)
			assert.Equal(test.want, string(got))
		})
	}
}

func TestStructuredContent(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	value := map[string]any{
		"name": "forma",
		"port": 8080,
	}

	jsonData, err := JSON(value).render()
	require.NoError(err)
	var decoded map[string]any
	require.NoError(json.Unmarshal(jsonData, &decoded))

	firstYAML, err := YAML(value).render()
	require.NoError(err)
	secondYAML, err := YAML(value).render()
	require.NoError(err)
	assert.Equal(string(firstYAML), string(secondYAML))
}

func TestGoSourceFormatsAST(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "source.go", "package example\nfunc F(){println(\"ok\")}", 0)
	require.NoError(err)

	got, err := GoSource(fileSet, file).render()
	require.NoError(err)
	assert.Contains(string(got), "func F() {")

	_, err = GoSource(nil, nil).render()
	assert.Error(err)
}

func TestContentInputsAreCopied(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)
	bytesInput := []byte("original")
	bytesContent := Bytes(bytesInput)
	bytesInput[0] = 'X'
	got, err := bytesContent.render()
	require.NoError(err)
	assert.Equal("original", string(got))

	values := Values{"name": "original"}
	text := Text("${name}", values)
	values["name"] = "changed"
	got, err = text.render()
	require.NoError(err)
	assert.Equal("original", string(got))
}

func TestBuilderWrapsRenderErrors(t *testing.T) {
	assert := assert.New(t)
	builder := NewBuilder()
	err := builder.Add(File("file.txt", Text("${missing}", nil)))
	assert.ErrorIs(err, ErrRender)
}
