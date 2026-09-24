package hub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"metarang/websocket-gateway/internal/hub"
)

type stubValidator struct {
	userID uint64
	err    error
}

func (s stubValidator) ValidateToken(_ context.Context, token string) (uint64, error) {
	if s.err != nil {
		return 0, s.err
	}
	if token == "" {
		return 0, fmt.Errorf("missing token")
	}
	if token != "valid-token" {
		return 0, fmt.Errorf("invalid token")
	}
	return s.userID, nil
}

type socketFrame struct {
	raw  string
	name string
	data json.RawMessage
}

func TestSocketIOAnonymousConnectAndFeatureBroadcast(t *testing.T) {
	h := hub.New(stubValidator{userID: 42}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	conn := dialEngineIO(t, srv.URL, "", false, nil)
	t.Cleanup(func() { _ = conn.Close() })

	connected := waitForEvent(t, conn, "connected", 3*time.Second)
	var payload map[string]any
	if err := json.Unmarshal(connected.data, &payload); err != nil {
		t.Fatalf("decode connected payload: %v", err)
	}
	if payload["authenticated"] != false {
		t.Fatalf("connected payload = %#v, want authenticated=false", payload)
	}
	if _, hasUser := payload["userId"]; hasUser {
		t.Fatalf("anonymous connected payload must not include userId: %#v", payload)
	}

	connections, users := h.Stats()
	if connections != 0 || users != 0 {
		t.Fatalf("stats after anonymous connect = %d/%d, want 0/0", connections, users)
	}

	h.BroadcastFeatureStatus(map[string]any{"id": float64(1001), "rgb": "G"})
	feature := waitForEvent(t, conn, "feature-status-changed", 3*time.Second)
	var featurePayload map[string]any
	if err := json.Unmarshal(feature.data, &featurePayload); err != nil {
		t.Fatalf("decode feature payload: %v", err)
	}
	if fmt.Sprint(featurePayload["id"]) != "1001" || featurePayload["rgb"] != "G" {
		t.Fatalf("feature payload = %#v", featurePayload)
	}

	h.BroadcastUserStatus(map[string]any{"user_id": float64(99), "online": true})
	status := waitForEvent(t, conn, "user-status-changed", 3*time.Second)
	var statusPayload map[string]any
	if err := json.Unmarshal(status.data, &statusPayload); err != nil {
		t.Fatalf("decode user-status payload: %v", err)
	}
	if statusPayload["online"] != true {
		t.Fatalf("user-status payload = %#v", statusPayload)
	}

	// Private notifications target user rooms only; anonymous sockets never join them.
	h.BroadcastNotification(map[string]any{
		"user_id": float64(42),
		"id":      "n-1",
		"title":   "secret",
	})
	if got := readEventIfAny(t, conn, 500*time.Millisecond); got != nil && got.name == "notification-received" {
		t.Fatalf("anonymous client received private notification: %s", got.raw)
	}
}

func TestSocketIOConnectAndFeatureBroadcast(t *testing.T) {
	h := hub.New(stubValidator{userID: 42}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	conn := dialEngineIO(t, srv.URL, "valid-token", false, nil)
	t.Cleanup(func() { _ = conn.Close() })

	connected := waitForEvent(t, conn, "connected", 3*time.Second)
	var payload map[string]any
	if err := json.Unmarshal(connected.data, &payload); err != nil {
		t.Fatalf("decode connected payload: %v", err)
	}
	if fmt.Sprint(payload["userId"]) != "42" {
		t.Fatalf("connected payload = %#v", payload)
	}
	if payload["authenticated"] != true {
		t.Fatalf("connected payload = %#v, want authenticated=true", payload)
	}

	connections, users := h.Stats()
	if connections != 1 || users != 1 {
		t.Fatalf("stats after connect = %d/%d, want 1/1", connections, users)
	}

	h.BroadcastFeatureStatus(map[string]any{"id": float64(1001), "rgb": "G"})
	feature := waitForEvent(t, conn, "feature-status-changed", 3*time.Second)
	var featurePayload map[string]any
	if err := json.Unmarshal(feature.data, &featurePayload); err != nil {
		t.Fatalf("decode feature payload: %v", err)
	}
	if fmt.Sprint(featurePayload["id"]) != "1001" || featurePayload["rgb"] != "G" {
		t.Fatalf("feature payload = %#v", featurePayload)
	}
}

func TestSocketIOAuthPacket(t *testing.T) {
	h := hub.New(stubValidator{userID: 7}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	conn := dialEngineIO(t, srv.URL, "valid-token", true, nil)
	t.Cleanup(func() { _ = conn.Close() })

	connected := waitForEvent(t, conn, "connected", 3*time.Second)
	var payload map[string]any
	if err := json.Unmarshal(connected.data, &payload); err != nil {
		t.Fatalf("decode connected payload: %v", err)
	}
	if fmt.Sprint(payload["userId"]) != "7" {
		t.Fatalf("connected payload = %#v", payload)
	}
}

func TestSocketIOBearerAuthorizationHeader(t *testing.T) {
	h := hub.New(stubValidator{userID: 9}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer valid-token")
	conn := dialEngineIO(t, srv.URL, "", false, headers)
	t.Cleanup(func() { _ = conn.Close() })

	connected := waitForEvent(t, conn, "connected", 3*time.Second)
	var payload map[string]any
	if err := json.Unmarshal(connected.data, &payload); err != nil {
		t.Fatalf("decode connected payload: %v", err)
	}
	if fmt.Sprint(payload["userId"]) != "9" {
		t.Fatalf("connected payload = %#v", payload)
	}
}

func TestSocketIORejectsInvalidToken(t *testing.T) {
	h := hub.New(stubValidator{userID: 42}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	conn := dialEngineIO(t, srv.URL, "bad-token", false, nil)
	t.Cleanup(func() { _ = conn.Close() })

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}
		raw := string(message)
		if strings.HasPrefix(raw, "44") || strings.Contains(raw, "unauthorized") {
			return
		}
	}
	t.Fatal("expected unauthorized connect error")
}

func TestBroadcastUserStatusAndNotification(t *testing.T) {
	h := hub.New(stubValidator{userID: 42}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	conn := dialEngineIO(t, srv.URL, "valid-token", false, nil)
	t.Cleanup(func() { _ = conn.Close() })
	_ = waitForEvent(t, conn, "connected", 3*time.Second)

	h.BroadcastUserStatus(map[string]any{"user_id": float64(42), "online": true})
	status := waitForEvent(t, conn, "user-status-changed", 3*time.Second)
	var statusPayload map[string]any
	if err := json.Unmarshal(status.data, &statusPayload); err != nil {
		t.Fatalf("decode user-status payload: %v", err)
	}
	if statusPayload["online"] != true {
		t.Fatalf("user-status payload = %#v", statusPayload)
	}

	h.BroadcastNotification(map[string]any{
		"user_id": float64(42),
		"id":      "n-1",
		"type":    "system",
		"title":   "hello",
		"message": "world",
	})
	notification := waitForEvent(t, conn, "notification-received", 3*time.Second)
	var notificationPayload map[string]any
	if err := json.Unmarshal(notification.data, &notificationPayload); err != nil {
		t.Fatalf("decode notification payload: %v", err)
	}
	if notificationPayload["title"] != "hello" {
		t.Fatalf("notification payload = %#v", notificationPayload)
	}
}

func TestBroadcastIgnoresInvalidIDs(t *testing.T) {
	h := hub.New(stubValidator{userID: 42}, []string{"http://localhost:5173", "*"})
	t.Cleanup(func() { _ = h.Close() })

	// Should not panic on unparseable identifiers.
	h.BroadcastUserStatus(map[string]any{"user_id": "nope"})
	h.BroadcastFeatureStatus(map[string]any{"rgb": "G"})
	h.BroadcastNotification(map[string]any{"title": "x"})

	connections, users := h.Stats()
	if connections != 0 || users != 0 {
		t.Fatalf("stats = %d/%d, want 0/0", connections, users)
	}
}

func dialEngineIO(t *testing.T, baseURL, token string, authPacket bool, headers http.Header) *websocket.Conn {
	t.Helper()

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	parsed.Scheme = "ws"
	parsed.Path = "/socket.io/"
	query := parsed.Query()
	query.Set("EIO", "4")
	query.Set("transport", "websocket")
	if token != "" && !authPacket {
		query.Set("token", token)
	}
	parsed.RawQuery = query.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), headers)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, open, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read engine.io open: %v", err)
	}
	if !strings.HasPrefix(string(open), "0") {
		t.Fatalf("expected engine.io open packet, got %q", open)
	}

	connect := "40"
	if authPacket {
		body, err := json.Marshal(map[string]string{"token": token})
		if err != nil {
			t.Fatalf("marshal auth: %v", err)
		}
		connect = "40" + string(body)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(connect)); err != nil {
		t.Fatalf("write socket.io connect: %v", err)
	}
	return conn
}

func waitForEvent(t *testing.T, conn *websocket.Conn, name string, timeout time.Duration) socketFrame {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		frame, err := readSocketFrame(conn, 500*time.Millisecond)
		if err != nil {
			if isTimeoutErr(err) {
				continue
			}
			t.Fatalf("read while waiting for %s: %v", name, err)
		}
		if frame != nil && frame.name == name {
			return *frame
		}
	}
	t.Fatalf("timed out waiting for %s", name)
	return socketFrame{}
}

func readEventIfAny(t *testing.T, conn *websocket.Conn, wait time.Duration) *socketFrame {
	t.Helper()
	frame, err := readSocketFrame(conn, wait)
	if err != nil {
		return nil
	}
	return frame
}

func readSocketFrame(conn *websocket.Conn, wait time.Duration) (*socketFrame, error) {
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	_, message, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	raw := string(message)
	switch {
	case raw == "2":
		_ = conn.WriteMessage(websocket.TextMessage, []byte("3"))
		return nil, nil
	case strings.HasPrefix(raw, "42"):
		frame, ok := parseSocketEvent(raw)
		if !ok {
			return nil, nil
		}
		return &frame, nil
	default:
		return nil, nil
	}
}

func isTimeoutErr(err error) bool {
	type timeout interface{ Timeout() bool }
	if te, ok := err.(timeout); ok {
		return te.Timeout()
	}
	return false
}

func parseSocketEvent(raw string) (socketFrame, bool) {
	payload := strings.TrimPrefix(raw, "42")
	var decoded []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil || len(decoded) < 2 {
		return socketFrame{}, false
	}
	var name string
	if err := json.Unmarshal(decoded[0], &name); err != nil {
		return socketFrame{}, false
	}
	return socketFrame{raw: raw, name: name, data: decoded[1]}, true
}
