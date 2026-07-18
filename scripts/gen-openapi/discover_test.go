package main

import "testing"

func TestParseMethodsAndPath(t *testing.T) {
	tests := []struct {
		input   string
		methods []string
		path    string
	}{
		{"POST /api/auth/register", []string{"post"}, "/api/auth/register"},
		{"GET /api/features/{feature}", []string{"get"}, "/api/features/{feature}"},
		{"GET /api/features/{feature}/trade-history.", []string{"get"}, "/api/features/{feature}/trade-history"},
		{"PUT/PATCH /api/kyc", []string{"put", "patch"}, "/api/kyc"},
		{"GET /api/tutorials/categories/{category:slug}", []string{"get"}, "/api/tutorials/categories/{category:slug}"},
	}

	for _, tt := range tests {
		methods, path, ok := parseMethodsAndPath(tt.input)
		if !ok {
			t.Fatalf("parseMethodsAndPath(%q) failed", tt.input)
		}
		if len(methods) != len(tt.methods) {
			t.Fatalf("methods = %v, want %v", methods, tt.methods)
		}
		for i := range methods {
			if methods[i] != tt.methods[i] {
				t.Fatalf("methods[%d] = %q, want %q", i, methods[i], tt.methods[i])
			}
		}
		if path != tt.path {
			t.Fatalf("path = %q, want %q", path, tt.path)
		}
	}
}

func TestDiscoverQueryParamsFromBody(t *testing.T) {
	body := `
		page := r.URL.Query().Get("page")
		if lb := r.URL.Query().Get("load_buildings"); lb == "true" {
		}
		title := r.FormValue("title")
	`
	params := discoverQueryParamsFromBody(body)
	names := map[string]string{}
	for _, p := range params {
		names[p.Name] = p.Type
	}

	if names["page"] != "integer" {
		t.Fatalf("page type = %q", names["page"])
	}
	if names["load_buildings"] != "boolean" {
		t.Fatalf("load_buildings type = %q", names["load_buildings"])
	}
	if names["title"] != "string" {
		t.Fatalf("title type = %q", names["title"])
	}
}

func TestEndpointKeyPatternMatch(t *testing.T) {
	key1 := endpointKey("get", "/api/features/{feature}")
	key2 := endpointKey("get", "/api/features/{id}")
	if key1 != key2 {
		t.Fatalf("pattern keys differ: %q vs %q", key1, key2)
	}
}

func TestParseHandlerFileFeaturesList(t *testing.T) {
	content := `// ListFeatures handles GET /api/features
// Query params: points (array), load_buildings (bool), user_features_location (bool)
func (h *FeaturesHandler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	if lb := r.URL.Query().Get("load_buildings"); lb == "true" {}
}`
	eps := parseHandlerFile(content)
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}
	if eps[0].Path != "/api/features" {
		t.Fatalf("path = %q", eps[0].Path)
	}
	names := map[string]bool{}
	for _, p := range eps[0].QueryParams {
		names[p.Name] = true
	}
	for _, want := range []string{"points", "load_buildings", "user_features_location"} {
		if !names[want] {
			t.Fatalf("missing query param %q", want)
		}
	}
}
