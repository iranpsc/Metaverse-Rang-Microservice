// Package webtransport is a compile-time compatibility shim for engine.io.
//
// The gateway only serves Socket.IO over websocket/polling (see service README).
// Upstream github.com/zishang520/webtransport-go@v0.9.1 requires vulnerable
// github.com/quic-go/quic-go < v0.59.1 (GO-2026-5676 / CVE-2026-40898) and is
// incompatible with patched quic-go releases. This shim preserves the small API
// surface engine.io imports while depending on a patched quic-go.
package webtransport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/quic-go/quic-go/http3"
)

var errWebTransportDisabled = errors.New("webtransport: not enabled in websocket-gateway")

// SessionErrorCode is the WebTransport session error code type expected by engine.io.
type SessionErrorCode uint32

// Server mirrors the engine.io-facing fields of upstream webtransport.Server.
type Server struct {
	H3 http3.Server

	// ReorderingTimeout retained for struct compatibility with engine.io callers.
	ReorderingTimeout time.Duration

	// CheckOrigin retained for struct compatibility with engine.io callers.
	CheckOrigin func(r *http.Request) bool
}

// Close closes the embedded HTTP/3 server, matching upstream behaviour used on shutdown.
func (s *Server) Close() error {
	return s.H3.Close()
}

// ListenAndServeTLS is unused by this gateway (websocket/polling only).
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return errWebTransportDisabled
}

// Upgrade is unused by this gateway (websocket/polling only).
func (s *Server) Upgrade(http.ResponseWriter, *http.Request) (*Session, error) {
	return nil, errWebTransportDisabled
}

// Session is the engine.io-facing WebTransport session type.
type Session struct{}

// AcceptStream is unused by this gateway (websocket/polling only).
func (s *Session) AcceptStream(context.Context) (*Stream, error) {
	return nil, errWebTransportDisabled
}

// CloseWithError is a no-op for the disabled transport.
func (s *Session) CloseWithError(SessionErrorCode, string) error {
	return nil
}

// LocalAddr returns nil for the disabled transport.
func (s *Session) LocalAddr() net.Addr { return nil }

// RemoteAddr returns nil for the disabled transport.
func (s *Session) RemoteAddr() net.Addr { return nil }

// Stream satisfies the stream deadline interface used by engine.io's wrapper.
type Stream struct{}

func (s *Stream) Read([]byte) (int, error)  { return 0, errWebTransportDisabled }
func (s *Stream) Write([]byte) (int, error) { return 0, errWebTransportDisabled }
func (s *Stream) Close() error              { return nil }
func (s *Stream) SetReadDeadline(time.Time) error {
	return errWebTransportDisabled
}
func (s *Stream) SetWriteDeadline(time.Time) error {
	return errWebTransportDisabled
}
