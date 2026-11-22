// Package httperr carries an HTTP status, a stable error code and a client-safe
// message through a handler's error path, and writes them as one JSON shape:
//
//	{"error": {"code": "not_found", "message": "order 42 not found"}}
//
// Domain code keeps its own error types and never imports this package; the
// HTTP adapter maps domain errors to *Error in one place and calls Write.
package httperr

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Codes used by the constructors. Applications may add their own with New.
const (
	CodeBadRequest   = "bad_request"
	CodeInvalid      = "invalid"
	CodeUnauthorized = "unauthorized"
	CodeForbidden    = "forbidden"
	CodeNotFound     = "not_found"
	CodeConflict     = "conflict"
	CodeInternal     = "internal"
)

const internalMessage = "internal error"

// Error is an error with the status and body it is answered with. Cause is
// kept for logs and errors.Is/As; it is never sent to the client.
type Error struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

// New returns an *Error with the given status, code and client-safe message.
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// BadRequest is a request that cannot be read: malformed JSON, a wrong type.
func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, CodeBadRequest, message)
}

// Invalid is a readable request that breaks a domain rule.
func Invalid(message string) *Error {
	return New(http.StatusBadRequest, CodeInvalid, message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, CodeForbidden, message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, CodeNotFound, message)
}

// Conflict is a duplicate or a state that does not allow the request.
func Conflict(message string) *Error {
	return New(http.StatusConflict, CodeConflict, message)
}

// Internal wraps an unexpected error. Its message is always "internal error",
// so nothing about the cause reaches the client.
func Internal(cause error) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: CodeInternal, Message: internalMessage, Cause: cause}
}

// WithCause returns a copy of e that keeps cause for logs and errors.Is/As.
func (e *Error) WithCause(cause error) *Error {
	c := *e
	c.Cause = cause
	return &c
}

func (e *Error) Error() string {
	text := "httperr: " + e.Code + ": " + e.Message
	if e.Cause != nil {
		text += ": " + e.Cause.Error()
	}
	return text
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// From returns the *Error in err's chain, or Internal(err) when there is none.
// From(nil) returns nil.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal(err)
}

type body struct {
	Error bodyError `json:"error"`
}

type bodyError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Write answers with From(err): its status and the JSON error body. A 5xx
// always carries the generic internal message, whatever Message says.
func Write(w http.ResponseWriter, err error) {
	e := From(err)
	if e == nil {
		e = Internal(errors.New("httperr: Write called with a nil error"))
	}
	message := e.Message
	if e.Status >= http.StatusInternalServerError {
		message = internalMessage
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(body{Error: bodyError{Code: e.Code, Message: message}})
}
