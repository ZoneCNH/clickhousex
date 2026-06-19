package clickhousex

import (
	"context"
	"errors"
)

// ErrorKind classifies the origin of an error.
type ErrorKind string

const (
	ErrorKindConfig      ErrorKind = "config"
	ErrorKindValidation  ErrorKind = "validation"
	ErrorKindConnection  ErrorKind = "connection"
	ErrorKindUnavailable ErrorKind = "unavailable"
	ErrorKindTimeout     ErrorKind = "timeout"
	ErrorKindAuth        ErrorKind = "auth"
	ErrorKindConflict    ErrorKind = "conflict"
	ErrorKindRateLimit   ErrorKind = "rate_limit"
	ErrorKindInternal    ErrorKind = "internal"
	ErrorKindQuery       ErrorKind = "query"
	ErrorKindBatch       ErrorKind = "batch"
)

// Error is the structured error type for clickhousex.
type Error struct {
	Kind      ErrorKind
	Op        string
	Message   string
	Cause     error
	Retryable bool
}

// NewError creates a new Error without a cause.
func NewError(kind ErrorKind, op string, message string, retryable bool) *Error {
	return newError(kind, op, message, retryable, nil)
}

// WrapError creates a new Error wrapping an existing cause.
func WrapError(kind ErrorKind, op string, message string, retryable bool, cause error) *Error {
	return newError(kind, op, message, retryable, cause)
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	detail := e.Message
	if detail == "" && e.Cause != nil {
		detail = e.Cause.Error()
	}

	message := "clickhousex"
	if e.Op != "" {
		message += ": " + e.Op
	}
	if detail != "" {
		message += ": " + detail
	}
	if e.Op == "" && detail == "" {
		message += ": " + string(e.Kind)
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// IsKind checks whether err contains an Error with the given kind.
func IsKind(err error, kind ErrorKind) bool {
	var target *Error
	if errors.As(err, &target) {
		return target.Kind == kind
	}
	return false
}

func newError(kind ErrorKind, op string, message string, retryable bool, cause error) *Error {
	if message == "" && cause != nil {
		message = cause.Error()
	}
	return &Error{
		Kind:      kind,
		Op:        op,
		Message:   message,
		Cause:     cause,
		Retryable: retryable,
	}
}

func validationError(op string, message string, cause error) *Error {
	return newError(ErrorKindValidation, op, message, false, cause)
}

func contextError(op string, cause error) *Error {
	kind := ErrorKindUnavailable
	retryable := false
	if errors.Is(cause, context.DeadlineExceeded) {
		kind = ErrorKindTimeout
		retryable = true
	}
	return newError(kind, op, "", retryable, cause)
}

func errorKind(err error) ErrorKind {
	var target *Error
	if errors.As(err, &target) {
		return target.Kind
	}
	return ErrorKindInternal
}
