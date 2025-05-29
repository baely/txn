// Package errors provides standardized error handling for the TXN project
package errors

import (
	"errors"
	"fmt"
)

// Common error types
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrInternal      = errors.New("internal error")
	ErrAlreadyExists = errors.New("already exists")
)

// Wrap adds context to an error while preserving the original error
func Wrap(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// Is reports whether err is or wraps target
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error in err's chain that matches target
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Must panics if err is not nil
func Must(err error) {
	if err != nil {
		panic(err)
	}
}
