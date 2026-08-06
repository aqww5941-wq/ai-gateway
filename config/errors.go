package config

import (
	"errors"
	"fmt"
)

// ErrorKind is a stable, machine-readable classification for failures at the
// bootstrap configuration boundary.
type ErrorKind string

const (
	ErrorKindRead        ErrorKind = "read"
	ErrorKindEnvironment ErrorKind = "environment"
	ErrorKindParse       ErrorKind = "parse"
	ErrorKindValidation  ErrorKind = "validation"
)

// ConfigError reports which configuration phase and field failed without
// exposing field values. Err retains the underlying filesystem or YAML cause.
type ConfigError struct {
	Kind    ErrorKind
	Path    string
	Problem string
	Err     error
}

func (e *ConfigError) Error() string {
	location := ""
	if e.Path != "" {
		location = " at " + e.Path
	}
	return fmt.Sprintf("config %s error%s: %s", e.Kind, location, e.Problem)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// ErrorKindOf extracts a stable configuration error classification.
func ErrorKindOf(err error) (ErrorKind, bool) {
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		return "", false
	}
	return configErr.Kind, true
}

func validationError(path, format string, args ...any) error {
	return &ConfigError{
		Kind:    ErrorKindValidation,
		Path:    path,
		Problem: fmt.Sprintf(format, args...),
	}
}
