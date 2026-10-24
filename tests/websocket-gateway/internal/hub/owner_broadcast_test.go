package hub_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"metarang/websocket-gateway/internal/hub"
)

func TestFeatureBroadcastTargetsOwnersWithoutChangingInput(t *testing.T) {
	h := hub.New(stubValidator{userID: 42}, []string{"*"})
	t.Cleanup(func() { _ = h.Close() })
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	conn := dialEngineIO(t, srv.URL, "valid-token", false, nil)
	t.Cleanup(func() { _ = conn.Close() })
	_ = waitForEvent(t, conn, "connected", 3*time.Second)
	data := map[string]any{"old_owner_id": json.Number("42"), "new_owner_id": "43", "rgb": "R"}
	h.BroadcastFeatureStatus(data)
	event := waitForEvent(t, conn, "feature-status-changed", 3*time.Second)
	var payload map[string]any
	if err := json.Unmarshal(event.data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["userType"] != "old_owner" || payload["rgb"] != "R" {
		t.Fatalf("owner payload = %#v", payload)
	}
	if _, ok := data["userType"]; ok {
		t.Fatal("broadcast mutated caller payload")
	}
}
