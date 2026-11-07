package service

// ValidationError marks a service error caused by client-provided input.
// Transport handlers can map it to InvalidArgument without inspecting the
// human-readable error message.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func newValidationError(message string) error {
	return &ValidationError{Message: message}
}
