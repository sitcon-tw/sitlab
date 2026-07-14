package apperror

import "fmt"

type Kind string

const (
	KindMalformed    Kind = "malformed"
	KindInvalid      Kind = "invalid"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindUnavailable  Kind = "unavailable"
	KindMethod       Kind = "method_not_allowed"
)

type Field struct {
	Name    string
	Code    string
	Message string
}

type Error struct {
	Kind    Kind
	Code    string
	Message string
	Fields  []Field
}

func (e *Error) Error() string { return e.Message }

func Invalid(code, message string, fields ...Field) error {
	return &Error{Kind: KindInvalid, Code: code, Message: message, Fields: fields}
}

func Malformed(message string, fields ...Field) error {
	return &Error{Kind: KindMalformed, Code: "VALIDATION_FAILED", Message: message, Fields: fields}
}

func Unavailable(message string) error {
	return &Error{Kind: KindUnavailable, Code: "SERVICE_UNAVAILABLE", Message: message}
}

func MethodNotAllowed(message string) error {
	return &Error{Kind: KindMethod, Code: "METHOD_NOT_ALLOWED", Message: message}
}

func Unauthorized(code, message string) error {
	return &Error{Kind: KindUnauthorized, Code: code, Message: message}
}

func Forbidden(code, message string) error {
	return &Error{Kind: KindForbidden, Code: code, Message: message}
}

func NotFound(resource string) error {
	return &Error{Kind: KindNotFound, Code: "NOT_FOUND", Message: fmt.Sprintf("%s not found", resource)}
}

func Conflict(code, message string) error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}
