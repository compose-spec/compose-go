package errdefs

import "errors"

var (
	// ErrNotFound is returned when an object is not found
	ErrNotFound = errors.New("not found")

	// ErrInvalid is returned when a compose project is invalid
	ErrInvalid = errors.New("invalid compose project")

	// ErrUnsupported is returned when a compose project uses an unsupported attribute
	ErrUnsupported = errors.New("unsupported attribute")
)

// IsNotFoundError returns true if the unwrapped error is ErrNotFound
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsInvalidError returns true if the unwrapped error is ErrInvalid
func IsInvalidError(err error) bool {
	return errors.Is(err, ErrInvalid)
}

// IsUnsupportedError returns true if the unwrapped error is ErrUnsupported
func IsUnsupportedError(err error) bool {
	return errors.Is(err, ErrUnsupported)
}
