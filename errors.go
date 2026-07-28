package forma

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPath      = errors.New("invalid path")
	ErrInvalidComponent = errors.New("invalid component")
	ErrRender           = errors.New("content render failed")
	ErrDuplicatePath    = errors.New("duplicate path")
	ErrPathCollision    = errors.New("path collision")
	ErrInspectConflict  = errors.New("inspection conflict")
	ErrApply            = errors.New("apply failed")
)

// PathError associates an error with an output path and, when available, the
// component that produced it.
type PathError struct {
	Path   string
	Origin string
	Err    error
}

func (e *PathError) Error() string {
	location := e.Path
	if location == "" {
		location = "<unknown>"
	}
	if e.Origin != "" {
		return fmt.Sprintf("%s (%s): %v", location, e.Origin, e.Err)
	}
	return fmt.Sprintf("%s: %v", location, e.Err)
}

func (e *PathError) Unwrap() error { return e.Err }
