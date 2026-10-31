package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"metarang/sitemap-generator-service/internal/service"
)

func TestTranslationSourceReplacesCacheAndFallsBack(t *testing.T) {
	response := `{"data":[{"code":"fa"},{"code":"en"}]}`
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/api/translations" || r.URL.Query().Get("active") != "true" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if response == "error" {
			http.Error(w, "unavailable", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	cache := filepath.Join(t.TempDir(), "translations.json")
	source := service.NewTranslationSource(server.URL, cache)
	check := func(want []string) {
		t.Helper()
		got, err := source.Active(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	check([]string{"en", "fa"})
	mu.Lock()
	response = `{"data":[{"code":"de"}]}`
	mu.Unlock()
	check([]string{"de"})
	mu.Lock()
	response = "error"
	mu.Unlock()
	check([]string{"de"})
	restarted := service.NewTranslationSource(server.URL, cache)
	got, err := restarted.Active(context.Background())
	if err != nil || !reflect.DeepEqual(got, []string{"de"}) {
		t.Fatalf("restart fallback: %v, %v", got, err)
	}
	data, err := os.ReadFile(cache)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `["de"]` {
		t.Fatalf("old cache retained: %s", data)
	}
}

func TestTranslationSourceFailsWithoutCacheAndRejectsBadCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"code":"../escape"}]}`))
	}))
	defer server.Close()
	source := service.NewTranslationSource(server.URL, filepath.Join(t.TempDir(), "missing.json"))
	if _, err := source.Active(context.Background()); err == nil {
		t.Fatal("expected error without valid response or cache")
	}
}
