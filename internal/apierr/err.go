package apierr

import "fmt"

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func Wrap(status int, code, message string, err error) *Error {
	if err == nil {
		return New(status, code, message)
	}
	return New(status, code, fmt.Sprintf("%s: %s", message, err.Error()))
}

func Invalid(message string) *Error { return New(400, "validation", message) }
func Unauthorized() *Error          { return New(401, "unauthenticated", "Authentication is required") }
func Forbidden(message string) *Error {
	if message == "" {
		message = "You do not have permission to perform this action"
	}
	return New(403, "forbidden", message)
}
func NotFound(resource string) *Error {
	return New(404, "not_found", resource+" was not found")
}
func Conflict(code, message string) *Error { return New(409, code, message) }
func StalePlan() *Error {
	return New(409, "stale_plan", "The impact preview no longer matches the current plan. Preview the policy again.")
}
func Unsupported(message string) *Error { return New(409, "unsupported_capability", message) }
