package domain

import "fmt"

type ErrorCode string

const (
	CodeInvalid   ErrorCode = "invalid"
	CodeNotFound  ErrorCode = "not_found"
	CodeConflict  ErrorCode = "conflict"
	CodeForbidden ErrorCode = "forbidden"
	CodeState     ErrorCode = "invalid_state"
)

type Error struct {
	Code           ErrorCode
	Field, Message string
	Details        any
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}
func NewError(code ErrorCode, field, message string) error {
	return &Error{Code: code, Field: field, Message: message}
}
func NewDetailedError(code ErrorCode, field, message string, details any) error {
	return &Error{Code: code, Field: field, Message: message, Details: details}
}
func ErrorCodeOf(err error) ErrorCode {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ""
}
