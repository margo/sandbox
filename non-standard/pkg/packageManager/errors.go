package packageManager

import "fmt"

type ErrUnsupportedSource struct {
	Type SourceType
}

func (e *ErrUnsupportedSource) Error() string {
	return fmt.Sprintf("unsupported source type: %s", e.Type)
}

type ErrPackageNotFound struct {
	Path string
}

func (e *ErrPackageNotFound) Error() string {
	return fmt.Sprintf("package not found at: %s", e.Path)
}

type ErrDescriptionNotFound struct {
	Path string
}

func (e *ErrDescriptionNotFound) Error() string {
	return fmt.Sprintf(
		"no %s found in: %s",
		ExpectedDescriptionFileName,
		e.Path,
	)
}

type ErrInvalidDescription struct {
	Path           string
	ContextualInfo interface{}
}

func (e *ErrInvalidDescription) Error() string {
	return fmt.Sprintf(
		"invalid application description at: %s, more: %v",
		e.Path,
		e.ContextualInfo,
	)
}

type ErrValidation struct {
	Err error
}

func (e *ErrValidation) Error() string {
	return fmt.Sprintf("validation failed: %v", e.Err)
}

func (e *ErrValidation) Unwrap() error {
	return e.Err
}
