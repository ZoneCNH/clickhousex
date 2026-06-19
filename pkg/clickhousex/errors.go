package clickhousex

import (
	"context"
	"errors"
	"io"
	"strings"
)

// ErrorKind classifies the origin of an error.
type ErrorKind string

const (
	ErrorKindConfig              ErrorKind = "config"
	ErrorKindValidation          ErrorKind = "validation"
	ErrorKindConnection          ErrorKind = "connection"
	ErrorKindUnavailable         ErrorKind = "unavailable"
	ErrorKindTimeout             ErrorKind = "timeout"
	ErrorKindAuth                ErrorKind = "auth"
	ErrorKindConflict            ErrorKind = "conflict"
	ErrorKindRateLimit           ErrorKind = "rate_limit"
	ErrorKindInternal            ErrorKind = "internal"
	ErrorKindQuery               ErrorKind = "query"
	ErrorKindBatch               ErrorKind = "batch"
	ErrorKindEmptyColumns        ErrorKind = "empty_columns"
	ErrorKindColumnCountMismatch ErrorKind = "column_count_mismatch"
	ErrorKindTypeMismatch        ErrorKind = "type_mismatch"
	ErrorKindTableNotFound       ErrorKind = "table_not_found"
)

var (
	ErrEmptyColumns        = NewError(ErrorKindEmptyColumns, "", "empty columns", false)
	ErrColumnCountMismatch = NewError(ErrorKindColumnCountMismatch, "", "column count mismatch", false)
	ErrTypeMismatch        = NewError(ErrorKindTypeMismatch, "", "type mismatch", false)
	ErrTableNotFound       = NewError(ErrorKindTableNotFound, "", "table not found", false)
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

func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	var targetError *Error
	if errors.As(target, &targetError) {
		return targetError.Kind != "" && e.Kind == targetError.Kind
	}
	return false
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

func operationError(kind ErrorKind, op string, err error) *Error {
	if err == nil {
		return nil
	}
	var typed *Error
	if errors.As(err, &typed) {
		return typed
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return contextError(op, err)
	}
	retryable := isRetryableError(err)
	classified := kind
	if retryable {
		classified = ErrorKindConnection
	}
	if isTableNotFoundError(err) {
		classified = ErrorKindTableNotFound
		retryable = false
	}
	return newError(classified, op, "", retryable, err)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Retryable
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection refused",
		"connection reset",
		"connection lost",
		"broken pipe",
		"i/o timeout",
		"timeout",
		"eof",
		"server closed",
		"bad connection",
		"acquire conn timeout",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func isTableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTableNotFound) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unknown table") ||
		strings.Contains(message, "table") && strings.Contains(message, "doesn't exist") ||
		strings.Contains(message, "table") && strings.Contains(message, "does not exist")
}

func errorKind(err error) ErrorKind {
	var target *Error
	if errors.As(err, &target) {
		return target.Kind
	}
	return ErrorKindInternal
}
