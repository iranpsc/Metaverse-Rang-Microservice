package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseCORSOrigins(t *testing.T) {
	got := parseCORSOrigins(" https://a.test, ,https://b.test ")
	if len(got) != 2 || got[0] != "https://a.test" || got[1] != "https://b.test" {
		t.Fatalf("origins=%v", got)
	}
	got = parseCORSOrigins(" , ")
	if len(got) != 1 || got[0] != "*" {
		t.Fatalf("default origins=%v", got)
	}
}

func TestCORSMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		origins                 []string
		origin, method, allowed string
		status                  int
	}{
		{"allowed", []string{"https://a.test"}, "https://a.test", http.MethodGet, "https://a.test", 200},
		{"rejected", []string{"https://a.test"}, "https://b.test", http.MethodGet, "", 200},
		{"wildcard origin", []string{"*"}, "https://b.test", http.MethodGet, "https://b.test", 200},
		{"wildcard no origin", []string{"*"}, "", http.MethodGet, "*", 200},
		{"preflight", []string{"https://a.test"}, "https://a.test", http.MethodOptions, "https://a.test", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := withCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(200) }), tc.origins)
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status || rec.Header().Get("Access-Control-Allow-Origin") != tc.allowed {
				t.Fatalf("status=%d headers=%v", rec.Code, rec.Header())
			}
			if called == (tc.method == http.MethodOptions) {
				t.Fatalf("handler called=%v", called)
			}
		})
	}
}

func TestWriteJSONAndLoadConfig(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusAccepted, map[string]string{"status": "ok"})
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted || body["status"] != "ok" {
		t.Fatalf("status=%d body=%v", rec.Code, body)
	}
	loadConfig()
}
