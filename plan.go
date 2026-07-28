package forma

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Metadata describes a reusable template.
type Metadata struct {
	Name        string
	Description string
}

// Template builds a component tree from typed input.
type Template[I any] interface {
	Metadata() Metadata
	Build(input I) (Component, error)
}

// PlanFor validates template input and returns its immutable plan.
func PlanFor[I any](template Template[I], input I) (Plan, error) {
	if template == nil {
		return Plan{}, fmt.Errorf("%w: nil template", ErrInvalidComponent)
	}
	root, err := template.Build(input)
	if err != nil {
		return Plan{}, err
	}
	builder := NewBuilder()
	if err := builder.Add(root); err != nil {
		return Plan{}, err
	}
	return builder.Plan(), nil
}

// ArtifactKind identifies a planned artifact type.
type ArtifactKind uint8

const (
	ArtifactDirectory ArtifactKind = iota + 1
	ArtifactFile
)

// Path is a validated, normalized destination-relative path.
type Path struct{ value string }

// String returns the slash-separated path.
func (p Path) String() string { return p.value }

// Artifact describes one immutable planned output.
type Artifact interface {
	Path() Path
	Kind() ArtifactKind
	Mode() fs.FileMode
	sealedArtifact()
}

// FileArtifact is an artifact with file contents.
type FileArtifact interface {
	Artifact
	Contents() []byte
}

type directoryArtifact struct {
	path   Path
	mode   fs.FileMode
	origin string
}

func (a directoryArtifact) Path() Path        { return a.path }
func (directoryArtifact) Kind() ArtifactKind  { return ArtifactDirectory }
func (a directoryArtifact) Mode() fs.FileMode { return a.mode }
func (directoryArtifact) sealedArtifact()     {}

type fileArtifact struct {
	path     Path
	mode     fs.FileMode
	contents []byte
	origin   string
}

func (a fileArtifact) Path() Path        { return a.path }
func (fileArtifact) Kind() ArtifactKind  { return ArtifactFile }
func (a fileArtifact) Mode() fs.FileMode { return a.mode }
func (fileArtifact) sealedArtifact()     {}
func (a fileArtifact) Contents() []byte {
	return append([]byte(nil), a.contents...)
}

// Plan is an immutable, deterministically ordered artifact collection.
type Plan struct {
	artifacts []Artifact
}

// Artifacts returns a defensive copy of the ordered artifacts.
func (p Plan) Artifacts() []Artifact {
	return append([]Artifact(nil), p.artifacts...)
}

// Builder incrementally validates and assembles components.
type Builder struct {
	artifacts map[string]Artifact
}

// NewBuilder returns an empty builder.
func NewBuilder() *Builder {
	return &Builder{artifacts: make(map[string]Artifact)}
}

// Add validates and transactionally adds components.
func (b *Builder) Add(components ...Component) error {
	if b == nil {
		return fmt.Errorf("%w: nil builder", ErrInvalidComponent)
	}
	next := make(map[string]Artifact, len(b.artifacts))
	maps.Copy(next, b.artifacts)
	for i, value := range components {
		origin := fmt.Sprintf("component[%d]", i)
		if err := addComponent(next, "", value, origin); err != nil {
			return err
		}
	}
	b.artifacts = next
	return nil
}

// Plan returns an immutable snapshot of the builder.
func (b *Builder) Plan() Plan {
	if b == nil {
		return Plan{}
	}
	artifacts := make([]Artifact, 0, len(b.artifacts))
	for _, artifact := range b.artifacts {
		artifacts = append(artifacts, cloneArtifact(artifact))
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Path().String() < artifacts[j].Path().String()
	})
	return Plan{artifacts: artifacts}
}

func addComponent(
	artifacts map[string]Artifact,
	prefix string,
	value Component,
	origin string,
) error {
	if value == nil {
		return &PathError{
			Origin: origin,
			Err:    fmt.Errorf("%w: nil component", ErrInvalidComponent),
		}
	}
	data := value.component()
	if data == nil {
		return &PathError{
			Origin: origin,
			Err:    fmt.Errorf("%w: nil component data", ErrInvalidComponent),
		}
	}

	switch c := data.(type) {
	case dirComponent:
		destination, err := joinOutputPath(prefix, c.path)
		if err != nil {
			return &PathError{Path: c.path, Origin: origin, Err: err}
		}
		if err := addArtifact(
			artifacts,
			directoryArtifact{
				path: Path{value: destination}, mode: 0o755, origin: origin,
			},
			origin,
		); err != nil {
			return err
		}
		for i, child := range c.children {
			childOrigin := fmt.Sprintf("%s/dir(%s)[%d]", origin, destination, i)
			err := addComponent(artifacts, destination, child, childOrigin)
			if err != nil {
				return err
			}
		}
		return nil

	case fileComponent:
		destination, err := joinOutputPath(prefix, c.path)
		if err != nil {
			return &PathError{Path: c.path, Origin: origin, Err: err}
		}
		if !c.valid {
			return invalidComponentError(destination, origin, "nil file option")
		}
		if err := validateFileMode(c.mode); err != nil {
			return &PathError{Path: destination, Origin: origin, Err: err}
		}
		if c.content == nil {
			return invalidComponentError(destination, origin, "nil content")
		}
		contents, err := c.content.render()
		if err != nil {
			return &PathError{
				Path:   destination,
				Origin: origin,
				Err:    fmt.Errorf("%w: %v", ErrRender, err),
			}
		}
		return addArtifact(
			artifacts,
			fileArtifact{
				path:     Path{value: destination},
				mode:     c.mode,
				contents: append([]byte(nil), contents...),
				origin:   origin,
			},
			origin,
		)

	case copyFileComponent:
		destination, err := joinOutputPath(prefix, c.path)
		if err != nil {
			return &PathError{Path: c.path, Origin: origin, Err: err}
		}
		if !c.valid {
			return invalidComponentError(destination, origin, "nil file option")
		}
		if err := validateFileMode(c.mode); err != nil {
			return &PathError{Path: destination, Origin: origin, Err: err}
		}
		if c.source == nil {
			return invalidComponentError(
				destination, origin, "nil source filesystem",
			)
		}
		info, err := fs.Stat(c.source, c.sourcePath)
		if err != nil {
			return invalidComponentError(
				destination, origin, fmt.Sprintf("copy source: %v", err),
			)
		}
		if !info.Mode().IsRegular() {
			return invalidComponentError(
				destination, origin, "copy source is not a regular file",
			)
		}
		contents, err := fs.ReadFile(c.source, c.sourcePath)
		if err != nil {
			return invalidComponentError(
				destination, origin, fmt.Sprintf("copy source: %v", err),
			)
		}
		return addArtifact(
			artifacts,
			fileArtifact{
				path:     Path{value: destination},
				mode:     c.mode,
				contents: contents,
				origin:   origin,
			},
			origin,
		)

	case groupComponent:
		for i, child := range c.components {
			childOrigin := fmt.Sprintf("%s/group[%d]", origin, i)
			err := addComponent(artifacts, prefix, child, childOrigin)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return invalidComponentError("", origin, "unknown component")
	}
}

func invalidComponentError(path, origin, message string) error {
	return &PathError{
		Path:   path,
		Origin: origin,
		Err:    fmt.Errorf("%w: %s", ErrInvalidComponent, message),
	}
}

func addArtifact(
	artifacts map[string]Artifact, artifact Artifact, origin string,
) error {
	destination := artifact.Path().String()
	if _, exists := artifacts[destination]; exists {
		return &PathError{
			Path: destination, Origin: origin, Err: ErrDuplicatePath,
		}
	}
	for existingPath, existing := range artifacts {
		isChild := strings.HasPrefix(destination, existingPath+"/")
		if isChild && existing.Kind() == ArtifactFile {
			return &PathError{
				Path:   destination,
				Origin: origin,
				Err: fmt.Errorf(
					"%w: parent %q is a file", ErrPathCollision, existingPath,
				),
			}
		}
		isParent := strings.HasPrefix(
			existingPath,
			destination+"/",
		)
		if isParent &&
			artifact.Kind() == ArtifactFile {
			return &PathError{
				Path:   destination,
				Origin: origin,
				Err: fmt.Errorf(
					"%w: path is a parent of %q", ErrPathCollision, existingPath,
				),
			}
		}
	}
	artifacts[destination] = artifact
	return nil
}

func joinOutputPath(prefix, value string) (string, error) {
	if value == "" || strings.IndexByte(value, 0) >= 0 {
		return "", ErrInvalidPath
	}
	if strings.Contains(value, `\`) || path.IsAbs(value) ||
		filepath.IsAbs(value) ||
		filepath.VolumeName(value) != "" {
		return "", ErrInvalidPath
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") ||
		cleaned != value {
		return "", ErrInvalidPath
	}
	if prefix == "" {
		return cleaned, nil
	}
	return prefix + "/" + cleaned, nil
}

func validateFileMode(mode fs.FileMode) error {
	if mode&fs.ModeType != 0 || mode.Perm() != mode {
		return fmt.Errorf("%w: invalid file mode %v", ErrInvalidComponent, mode)
	}
	return nil
}

func cloneArtifact(artifact Artifact) Artifact {
	switch a := artifact.(type) {
	case directoryArtifact:
		return a
	case fileArtifact:
		a.contents = append([]byte(nil), a.contents...)
		return a
	default:
		panic(errors.New("forma: impossible artifact type"))
	}
}

func artifactOrigin(artifact Artifact) string {
	switch a := artifact.(type) {
	case directoryArtifact:
		return a.origin
	case fileArtifact:
		return a.origin
	default:
		return ""
	}
}
