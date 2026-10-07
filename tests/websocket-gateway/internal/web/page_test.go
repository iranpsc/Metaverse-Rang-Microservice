package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"metarang/websocket-gateway/internal/web"
)

func TestTesterPage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	web.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q", got)
	}

	body := rec.Body.String()
	for _, id := range []string{"host", "channel", "event", "token", "connect", "alert", "broadcast"} {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Fatalf("tester page missing #%s", id)
		}
	}
	if !strings.Contains(body, "notifications") {
		t.Fatal("tester page missing private notifications channel")
	}
}
