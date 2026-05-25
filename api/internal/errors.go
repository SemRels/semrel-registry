package internal

import "errors"

var (
	ErrNotImplemented      = errors.New("not implemented")
	ErrDatabaseUnavailable = errors.New("database unavailable")
	ErrPluginNotFound      = errors.New("plugin not found")
	ErrDuplicatePlugin     = errors.New("plugin already exists")
	ErrValidation          = errors.New("validation error")
)

type ValidationError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ErrValidation.Error()
	}
	if e.Field == "" {
		return e.Issue
	}
	if e.Issue == "" {
		return e.Field
	}
	return e.Field + ": " + e.Issue
}

func (e *ValidationError) Unwrap() error {
	return ErrValidation
}
