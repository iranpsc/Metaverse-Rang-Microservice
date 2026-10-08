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

	// A single deadline: gorilla/websocket panics if ReadMessage is called again after any error.
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if isTimeoutErr(err) {
				t.Fatal("expected unauthorized connect error")
			}
			return
		}
		raw := string(message)
		if raw == "2" {
			if err := conn.WriteMessage(websocket.TextMessage, []byte("3")); err != nil {
				t.Fatalf("write engine.io pong: %v", err)
			}
			continue
		}
		if strings.HasPrefix(raw, "44") || strings.Contains(raw, "unauthorized") {
			return
		}
	}
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

	h.BroadcastUserStatus(map[string]any{
		"data": map[string]any{"user_id": "42", "online": false},
	})
	wrapped := waitForEvent(t, conn, "user-status-changed", 3*time.Second)
	var wrappedPayload map[string]any
	if err := json.Unmarshal(wrapped.data, &wrappedPayload); err != nil {
		t.Fatalf("decode wrapped user-status payload: %v", err)
	}
	inner, ok := wrappedPayload["data"].(map[string]any)
	if !ok || inner["user_id"] != "42" || inner["online"] != false {
		t.Fatalf("wrapped user-status payload = %#v", wrappedPayload)
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
	// A single deadline: gorilla/websocket panics if ReadMessage is called again after any error.
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	for {
		frame, err := readSocketFrame(conn)
		if err != nil {
			if isTimeoutErr(err) {
				t.Fatalf("timed out waiting for %s", name)
			}
			t.Fatalf("read while waiting for %s: %v", name, err)
		}
		if frame != nil && frame.name == name {
			return *frame
		}
	}
}

func readEventIfAny(t *testing.T, conn *websocket.Conn, wait time.Duration) *socketFrame {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	frame, err := readSocketFrame(conn)
	if err != nil {
		return nil
	}
	return frame
}

func readSocketFrame(conn *websocket.Conn) (*socketFrame, error) {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		raw := string(message)
		switch {
		case raw == "2":
			if err := conn.WriteMessage(websocket.TextMessage, []byte("3")); err != nil {
				return nil, err
			}
		case strings.HasPrefix(raw, "42"):
			frame, ok := parseSocketEvent(raw)
			if !ok {
				continue
			}
			return &frame, nil
		}
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
