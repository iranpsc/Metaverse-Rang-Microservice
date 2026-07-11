package sentry

import (
	"net/http"

	sentryhttp "github.com/getsentry/sentry-go/http"
)

// HTTPMiddleware wraps an HTTP handler with Sentry panic recovery and request context.
func HTTPMiddleware(next http.Handler) http.Handler {
	if !enabled {
		return next
	}

	return sentryhttp.New(sentryhttp.Options{
		Repanic:         false,
		WaitForDelivery: false,
	}).Handle(next)
}
