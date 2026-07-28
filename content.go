package forma

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"maps"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Content is a sealed value that renders file bytes during planning.
type Content interface {
	render() ([]byte, error)
	sealedContent()
}

type bytesContent []byte

func (c bytesContent) render() ([]byte, error) {
	return append([]byte(nil), c...), nil
}

func (bytesContent) sealedContent() {}

// Bytes returns content containing a defensive copy of contents.
func Bytes(contents []byte) Content {
	return bytesContent(append([]byte(nil), contents...))
}

// Values contains named substitutions for Text.
type Values map[string]string

type textContent struct {
	pattern string
	values  Values
}

func (c textContent) sealedContent() {}

var placeholderName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (c textContent) render() ([]byte, error) {
	var out strings.Builder
	used := make(map[string]bool, len(c.values))

	for i := 0; i < len(c.pattern); {
		if c.pattern[i] != '$' {
			out.WriteByte(c.pattern[i])
			i++
			continue
		}
		if i+1 < len(c.pattern) && c.pattern[i+1] == '$' {
			out.WriteByte('$')
			i += 2
			continue
		}
		if i+1 >= len(c.pattern) || c.pattern[i+1] != '{' {
			out.WriteByte('$')
			i++
			continue
		}

		end := strings.IndexByte(c.pattern[i+2:], '}')
		if end < 0 {
			return nil, fmt.Errorf("unterminated placeholder at byte %d", i)
		}
		end += i + 2
		name := c.pattern[i+2 : end]
		if !placeholderName.MatchString(name) {
			return nil, fmt.Errorf("invalid placeholder %q", name)
		}
		value, ok := c.values[name]
		if !ok {
			return nil, fmt.Errorf("missing value for %q", name)
		}
		out.WriteString(value)
		used[name] = true
		i = end + 1
	}

	var unused []string
	for name := range c.values {
		if !used[name] {
			unused = append(unused, name)
		}
	}
	if len(unused) != 0 {
		sort.Strings(unused)
		return nil, fmt.Errorf("unused values: %s", strings.Join(unused, ", "))
	}
	return []byte(out.String()), nil
}

// Text returns restricted ${name} substitution content.
func Text(pattern string, values Values) Content {
	copied := make(Values, len(values))
	maps.Copy(copied, values)
	return textContent{pattern: pattern, values: copied}
}

type jsonContent struct{ value any }

func (jsonContent) sealedContent() {}
func (c jsonContent) render() ([]byte, error) {
	data, err := json.MarshalIndent(c.value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// JSON returns indented JSON content with a trailing newline.
func JSON(value any) Content { return jsonContent{value: value} }

type yamlContent struct{ value any }

func (yamlContent) sealedContent() {}
func (c yamlContent) render() ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(c.value); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// YAML returns deterministically encoded YAML content.
func YAML(value any) Content { return yamlContent{value: value} }

type goSourceContent struct {
	fileSet *token.FileSet
	file    *ast.File
}

func (goSourceContent) sealedContent() {}
func (c goSourceContent) render() ([]byte, error) {
	if c.fileSet == nil || c.file == nil {
		return nil, errors.New("nil Go file set or AST")
	}
	var out bytes.Buffer
	if err := format.Node(&out, c.fileSet, c.file); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// GoSource returns Go source formatted from file with go/format.
func GoSource(fileSet *token.FileSet, file *ast.File) Content {
	return goSourceContent{
		fileSet: fileSet,
		file:    file,
	}
}
