package cli

import "fmt"

// ExitCode mirrors conventional exit code semantics:
//
//	0 — success
//	1 — user/input error (bad args, invalid input)
//	2 — system error (cannot open file, network failure)
type ExitCode int

const (
	CodeSuccess ExitCode = 0
	CodeError   ExitCode = 1
	CodeFatal   ExitCode = 2
)

// Error wraps an exit code, message, and optional cause.
// It implements the error interface so it can flow through RunE handlers.
type Error struct {
	Code    ExitCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// Exitf returns a *Error with the given code and formatted message.
// Use this to signal a controlled exit from a command handler.
func Exitf(code ExitCode, format string, args ...interface{}) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap returns a *Error with code CodeError wrapping an underlying error.
func Wrap(err error, format string, args ...interface{}) *Error {
	return &Error{
		Code:    CodeError,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
	}
}

// WrapFatal returns a *Error with code CodeFatal wrapping an underlying error.
func WrapFatal(err error, format string, args ...interface{}) *Error {
	return &Error{
		Code:    CodeFatal,
		Message: fmt.Sprintf(format, args...),
		Cause:   err,
	}
}
