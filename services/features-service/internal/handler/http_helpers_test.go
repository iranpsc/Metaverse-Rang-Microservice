package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"metarang/features-service/internal/models"
	featurespb "metarang/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestEffectiveHTTPMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := effectiveHTTPMethod(req); got != http.MethodGet {
		t.Fatalf("got %s", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/?_method=patch", nil)
	if got := effectiveHTTPMethod(req); got != http.MethodPatch {
		t.Fatalf("got %s", got)
	}

	form := url.Values{}
	form.Set("_method", "delete")
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := effectiveHTTPMethod(req); got != http.MethodDelete {
		t.Fatalf("got %s", got)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("_method", "put")
	_ = mw.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if got := effectiveHTTPMethod(req); got != http.MethodPut {
		t.Fatalf("got %s", got)
	}
}

func TestDecodeBody(t *testing.T) {
	var into map[string]interface{}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if err := decodeBody(req, &into); err == nil {
		t.Fatal("expected EOF")
	}

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"karbari":"m"}`))
	req.Header.Set("Content-Type", "application/json")
	if err := decodeBody(req, &into); err != nil {
		t.Fatal(err)
	}
	if into["karbari"] != "m" {
		t.Fatalf("got %#v", into)
	}

	form := url.Values{}
	form.Set("karbari", "t")
	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	into = map[string]interface{}{}
	if err := decodeBody(req, &into); err != nil {
		t.Fatal(err)
	}
	if into["karbari"] != "t" {
		t.Fatalf("got %#v", into)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("minimum_price_percentage", "90")
	_ = mw.Close()
	req = httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.ContentLength = -1
	into = map[string]interface{}{}
	if err := decodeBody(req, &into); err != nil {
		t.Fatal(err)
	}
	if into["minimum_price_percentage"] != "90" {
		t.Fatalf("got %#v", into)
	}
}

func TestParsePoints(t *testing.T) {
	q := url.Values{}
	q.Set("points[0]", "1")
	q.Set("points[1]", "2")
	q.Set("points[2]", "3")
	q.Set("points[3]", "4")
	got, ok := parsePoints(q)
	if !ok || len(got) != 4 {
		t.Fatalf("indexed: %v %v", got, ok)
	}

	q = url.Values{"points[]": {"1", "2", "3", "4"}}
	if _, ok := parsePoints(q); !ok {
		t.Fatal("points[]")
	}

	q = url.Values{"points": {"1", "2", "3", "4"}}
	if _, ok := parsePoints(q); !ok {
		t.Fatal("repeated points")
	}

	q = url.Values{"points": {`["1","2","3","4"]`}}
	if _, ok := parsePoints(q); !ok {
		t.Fatal("json points")
	}

	if _, ok := parsePoints(url.Values{}); ok {
		t.Fatal("expected false")
	}
}

func TestClientIPAndJSONHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", " 1.2.3.4, 5.6.7.8")
	if got := clientIP(req); got != "1.2.3.4" {
		t.Fatalf("got %s", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "9.9.9.9")
	if got := clientIP(req); got != "9.9.9.9" {
		t.Fatalf("got %s", got)
	}
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:99"
	if got := clientIP(req); got != "10.0.0.1" {
		t.Fatalf("got %s", got)
	}

	if parseJSONString("") != nil {
		t.Fatal("empty")
	}
	if parseJSONString("{bad") != "{bad" {
		t.Fatal("invalid json should return raw")
	}
	if parseJSONString(`{"a":1}`) == nil {
		t.Fatal("expected object")
	}

	id := uint64(3)
	if optionalUint64(nil) != nil || optionalUint64(&id) != id {
		t.Fatal("optionalUint64")
	}
	n := int64(4)
	if optionalInt64(nil) != nil || optionalInt64(&n) != n {
		t.Fatal("optionalInt64")
	}
	s := "x"
	if optionalString(nil) != nil || optionalString(&s) != "x" {
		t.Fatal("optionalString")
	}
	if emptyToNil("") != nil || emptyToNil("a") != "a" {
		t.Fatal("emptyToNil")
	}
	f := 1.5
	if optionalFloat64(nil) != nil || optionalFloat64(&f) != f {
		t.Fatal("optionalFloat64")
	}

	if parseFlexibleNumber("") != "" {
		t.Fatal("empty number")
	}
	if parseFlexibleNumber("12") != int64(12) {
		t.Fatal("int")
	}
	if parseFlexibleNumber("1.5") != 1.5 {
		t.Fatal("float")
	}
	if parseFlexibleNumber("abc") != "abc" {
		t.Fatal("raw")
	}
}

func TestCitizenJSONHelpers(t *testing.T) {
	pts := toProtoCitizenChartPoints([]models.CitizenChartPoint{{Karbari: "m", Label: "l", Amount: 1}})
	out := citizenChartPointsJSON([]*featurespb.CitizenChartPoint{nil, pts[0]})
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if citizenCenterJSON(nil) != nil {
		t.Fatal("nil center")
	}
	if citizenCenterJSON(&featurespb.CitizenFeatureCenter{X: 1, Y: 2}) == nil {
		t.Fatal("center")
	}
	imgs := citizenImagesJSON([]*featurespb.Image{nil, {Id: 1, Url: "u"}})
	if len(imgs) != 1 {
		t.Fatalf("imgs=%d", len(imgs))
	}
}

func TestParseBuildingInformation(t *testing.T) {
	if parseBuildingInformation(map[string]interface{}{}) != nil {
		t.Fatal("empty")
	}
	info := parseBuildingInformation(map[string]interface{}{"name": "Shop"})
	if info == nil || info.Name != "Shop" {
		t.Fatalf("%#v", info)
	}
	nested := parseBuildingInformation(map[string]interface{}{
		"information": map[string]interface{}{"address": "a"},
	})
	if nested == nil || nested.Address != "a" {
		t.Fatalf("%#v", nested)
	}
	m := buildingInformationMap(nil)
	if len(m) != 0 {
		t.Fatal(m)
	}
	m = buildingInformationMap(&featurespb.BuildingInformation{
		ActivityLine: "al", Name: "n", Address: "a", PostalCode: "p", Website: "w", Description: "d",
	})
	if len(m) != 6 {
		t.Fatalf("%#v", m)
	}
}

func TestWriteJSONAndGRPCError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, nil)
	if rec.Code != 200 {
		t.Fatal(rec.Code)
	}

	rec = httptest.NewRecorder()
	writeError(rec, 400, "nope")
	if rec.Code != 400 {
		t.Fatal(rec.Code)
	}

	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.Unauthenticated, "auth"))
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.NotFound, "nf"))
	if rec.Code != 404 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.InvalidArgument, "bad"))
	if rec.Code != 422 && rec.Code != 400 {
		t.Fatalf("invalid arg status %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.PermissionDenied, "no"))
	if rec.Code != 403 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.AlreadyExists, "dup"))
	if rec.Code != 409 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.FailedPrecondition, "pre"))
	if rec.Code != 412 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.Unavailable, "down"))
	if rec.Code != 503 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, status.Error(codes.Internal, "boom"))
	if rec.Code != 500 {
		t.Fatal(rec.Code)
	}
	rec = httptest.NewRecorder()
	writeGRPCError(rec, assertErr{})
	if rec.Code != 500 {
		t.Fatal(rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/x?page=2", nil)
	req.Host = "example.test"
	t.Setenv("APP_URL", "https://app.example")
	if !strings.HasPrefix(publicBaseURL(req), "https://app.example") {
		t.Fatalf("%s", publicBaseURL(req))
	}
	t.Setenv("APP_URL", "")
	req.Header.Set("X-Forwarded-Proto", "https")
	if publicBaseURL(req) != "https://example.test" {
		t.Fatalf("%s", publicBaseURL(req))
	}
	links := buildSimplePaginationLinks(req, 2, true)
	if links["prev"] == nil || links["next"] == nil {
		t.Fatalf("%#v", links)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "plain" }
