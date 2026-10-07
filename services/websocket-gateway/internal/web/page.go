// Package web serves the Socket.IO connection tester page.
package web

import (
	_ "embed"
	"net/http"
)

//go:embed tester.html
var testerHTML []byte

// Handler serves the WebSocket connection tester.
func Handler() http.Handler {
	return http.HandlerFunc(serveTester)
}

func serveTester(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(testerHTML)
}
