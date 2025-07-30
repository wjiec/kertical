package errs

import (
	stderrors "errors"
)

// Ignore checks if the provided error is nil or matches any of the specified errors to ignore.
//
// If the error is nil, it returns nil indicating no error.
// If the error matches any of the specified errors to ignore, it returns nil indicating no error.
// Otherwise, it returns the original error.
func Ignore(err error, ignores ...error) error {
	if err == nil {
		return nil
	}

	for _, ignore := range ignores {
		if stderrors.Is(err, ignore) {
			return nil
		}
	}
	return err
}
