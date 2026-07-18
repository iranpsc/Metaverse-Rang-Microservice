// Package errs defines sentinel errors for the notifications service.
package errs

import "errors"

var (
	// ErrNotImplemented indicates that the functionality is pending implementation.
	ErrNotImplemented = errors.New("not implemented")
	// ErrNotificationNotFound indicates that a notification was not found.
	ErrNotificationNotFound = errors.New("notification not found")
)
