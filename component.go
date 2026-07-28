package forma

import (
	"io/fs"
)

type componentData interface {
	isComponentData()
}

// Component is a sealed, immutable description that contributes artifacts to
// a plan.
type Component interface {
	component() componentData
}

type component struct {
	data componentData
}

func (c component) component() componentData { return c.data }

type dirComponent struct {
	path     string
	children []Component
}

func (dirComponent) isComponentData() {}

type fileComponent struct {
	path    string
	content Content
	mode    fs.FileMode
	valid   bool
}

func (fileComponent) isComponentData() {}

type copyFileComponent struct {
	path       string
	source     fs.FS
	sourcePath string
	mode       fs.FileMode
	valid      bool
}

func (copyFileComponent) isComponentData() {}

type groupComponent struct {
	components []Component
}

func (groupComponent) isComponentData() {}

// FileOption configures a generated file.
type FileOption interface {
	applyFile(*fileSettings)
	sealedFileOption()
}

type fileSettings struct {
	mode  fs.FileMode
	valid bool
}

type fileOption func(*fileSettings)

func (o fileOption) applyFile(settings *fileSettings) { o(settings) }
func (fileOption) sealedFileOption()                  {}

// WithFileMode sets regular file permission bits.
func WithFileMode(mode fs.FileMode) FileOption {
	return fileOption(func(settings *fileSettings) { settings.mode = mode })
}

// Dir creates a directory containing children.
func Dir(path string, children ...Component) Component {
	return component{data: dirComponent{
		path: path, children: append([]Component(nil), children...),
	}}
}

// File creates a file from rendered content.
func File(path string, content Content, opts ...FileOption) Component {
	settings := fileSettings{mode: 0o644, valid: true}
	for _, opt := range opts {
		if opt == nil {
			settings.valid = false
			continue
		}
		opt.applyFile(&settings)
	}
	return component{data: fileComponent{
		path: path, content: content, mode: settings.mode, valid: settings.valid,
	}}
}

// CopyFile creates a file by reading one regular file from source.
func CopyFile(
	path string, source fs.FS, sourcePath string, opts ...FileOption,
) Component {
	settings := fileSettings{mode: 0o644, valid: true}
	for _, opt := range opts {
		if opt == nil {
			settings.valid = false
			continue
		}
		opt.applyFile(&settings)
	}
	return component{data: copyFileComponent{
		path:       path,
		source:     source,
		sourcePath: sourcePath,
		mode:       settings.mode,
		valid:      settings.valid,
	}}
}

// Group combines components without adding a directory prefix.
func Group(components ...Component) Component {
	copied := append([]Component(nil), components...)
	return component{data: groupComponent{components: copied}}
}

// When includes components only when condition is true.
func When(condition bool, components ...Component) Component {
	if !condition {
		return Group()
	}
	return Group(components...)
}
