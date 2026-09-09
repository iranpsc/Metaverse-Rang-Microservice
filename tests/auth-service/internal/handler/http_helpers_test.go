package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/service"
	"metarang/shared/pkg/helpers"
)

func TestEffectiveHTTPMethod(t *testing.T) {
	t.Run("non-post passthrough", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/x", nil)
		if got := handler.EffectiveHTTPMethod(r); got != http.MethodGet {
			t.Fatalf("got %s", got)
		}
	})

	t.Run("query spoof", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/x?_method=PUT", nil)
		if got := handler.EffectiveHTTPMethod(r); got != http.MethodPut {
			t.Fatalf("got %s", got)
		}
	})

	t.Run("form spoof", func(t *testing.T) {
		body := strings.NewReader("_method=DELETE")
		r := httptest.NewRequest(http.MethodPost, "/x", body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if got := handler.EffectiveHTTPMethod(r); got != http.MethodDelete {
			t.Fatalf("got %s", got)
		}
	})

	t.Run("empty spoof ignored", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/x?_method=%20", nil)
		if got := handler.EffectiveHTTPMethod(r); got != http.MethodPost {
			t.Fatalf("got %s", got)
		}
	})
}

func TestSetProjectLocale(t *testing.T) {
	t.Cleanup(func() { handler.SetProjectLocale("en") })

	handler.SetProjectLocale("fa")
	if got := handler.GetProjectLocaleForTest(); got != "fa" {
		t.Fatalf("got %s", got)
	}
	handler.SetProjectLocale(" EN ")
	if got := handler.GetProjectLocaleForTest(); got != "en" {
		t.Fatalf("got %s", got)
	}
	handler.SetProjectLocale("de")
	if got := handler.GetProjectLocaleForTest(); got != "en" {
		t.Fatalf("got %s", got)
	}
}

func TestWriteJSONForTest(t *testing.T) {
	cases := []struct {
		name     string
		data     interface{}
		skipWrap bool
		wantKey  string
	}{
		{"nil", nil, false, "data"},
		{"plain map wrap", map[string]interface{}{"a": 1}, false, "data"},
		{"already data", map[string]interface{}{"data": 1}, false, "data"},
		{"error map", map[string]interface{}{"error": "x"}, false, "error"},
		{"validation", map[string]interface{}{"message": "m", "errors": map[string]string{}}, false, "message"},
		{"url string map", map[string]string{"url": "http://x"}, false, "url"},
		{"link string map", map[string]string{"link": "http://x"}, false, "link"},
		{"error string map", map[string]string{"error": "e"}, false, "error"},
		{"skip wrap", map[string]interface{}{"a": 1}, true, "a"},
		{"slice wrap", []int{1, 2}, false, "data"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			if tc.skipWrap {
				handler.WriteJSONForTest(rr, http.StatusOK, tc.data, true)
			} else {
				handler.WriteJSONForTest(rr, http.StatusOK, tc.data)
			}
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			if _, ok := body[tc.wantKey]; !ok {
				t.Fatalf("body missing %q: %v", tc.wantKey, body)
			}
		})
	}
}

func TestWriteErrorAndGRPCError(t *testing.T) {
	rr := httptest.NewRecorder()
	handler.WriteErrorForTest(rr, http.StatusBadRequest, "boom")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", rr.Code)
	}

	cases := []struct {
		err  error
		code int
	}{
		{errors.New("plain"), http.StatusInternalServerError},
		{status.Error(codes.Unauthenticated, "u"), http.StatusUnauthorized},
		{status.Error(codes.NotFound, "n"), http.StatusNotFound},
		{status.Error(codes.PermissionDenied, "p"), http.StatusForbidden},
		{status.Error(codes.AlreadyExists, "a"), http.StatusConflict},
		{status.Error(codes.FailedPrecondition, "f"), http.StatusPreconditionFailed},
		{status.Error(codes.Unavailable, "x"), http.StatusServiceUnavailable},
		{status.Error(codes.ResourceExhausted, "rate limited"), http.StatusTooManyRequests},
		{status.Error(codes.Internal, "i"), http.StatusInternalServerError},
		{status.Error(codes.InvalidArgument, "plain msg"), http.StatusUnprocessableEntity},
		{status.Error(codes.InvalidArgument, helpers.EncodeValidationError(map[string]string{"f": "bad"})), http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		rr := httptest.NewRecorder()
		handler.WriteGRPCErrorWithLocaleForTest(rr, tc.err, "en")
		if rr.Code != tc.code {
			t.Fatalf("err=%v got=%d want=%d", tc.err, rr.Code, tc.code)
		}
	}
}

func TestGetClientIPAndExtractID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	if ip := handler.GetClientIPForTest(r); ip != "1.1.1.1" {
		t.Fatalf("xff=%s", ip)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Real-IP", "3.3.3.3")
	if ip := handler.GetClientIPForTest(r); ip != "3.3.3.3" {
		t.Fatalf("xri=%s", ip)
	}

	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "4.4.4.4:1234"
	if ip := handler.GetClientIPForTest(r); ip != "4.4.4.4" {
		t.Fatalf("remote=%s", ip)
	}

	if id := handler.ExtractIDFromPathForTest("/api/users/55/photos", "/api/users/"); id != "55/photos" {
		t.Fatalf("id=%s", id)
	}
	if id := handler.ExtractIDFromPathForTest("/other", "/api/users/"); id != "" {
		t.Fatalf("id=%s", id)
	}
	if id := handler.ExtractIDFromPathForTest("/api/users/9?x=1", "/api/users/"); id != "9" {
		t.Fatalf("id=%s", id)
	}
}

func TestPaginationAndPublicBaseURL(t *testing.T) {
	t.Setenv("APP_URL", "https://gateway.example")
	r := httptest.NewRequest(http.MethodGet, "http://upstream/api/users?search=a", nil)
	links := handler.BuildSimplePaginationLinksForTest(r, 2, true)
	if links["prev"] == nil || links["next"] == nil || links["first"] == nil {
		t.Fatalf("links=%v", links)
	}
	if !strings.Contains(links["first"].(string), "https://gateway.example") {
		t.Fatalf("first=%v", links["first"])
	}

	os.Unsetenv("APP_URL")
	r = httptest.NewRequest(http.MethodGet, "http://host/api/x", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	base := handler.PublicBaseURLForTest(r)
	if base != "https://host" {
		t.Fatalf("base=%s", base)
	}

	links = handler.BuildSimplePaginationLinksForTest(r, 1, false)
	if links["prev"] != nil || links["next"] != nil {
		t.Fatalf("links=%v", links)
	}
}

func TestDecodeRequestBodyAndQuery(t *testing.T) {
	type payload struct {
		Name  string `json:"name" form:"name"`
		Age   int    `json:"age" form:"age"`
		Flag  bool   `json:"flag" form:"flag"`
		Count uint64 `json:"count" form:"count"`
	}

	t.Run("json body", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"ada","age":30,"flag":true,"count":3}`)
		r := httptest.NewRequest(http.MethodPost, "/", body)
		r.Header.Set("Content-Type", "application/json")
		var p payload
		require.NoError(t, handler.DecodeRequestBodyForTest(r, &p))
		if p.Name != "ada" || p.Age != 30 || !p.Flag || p.Count != 3 {
			t.Fatalf("%+v", p)
		}
	})

	t.Run("form urlencoded", func(t *testing.T) {
		body := strings.NewReader("name=bob&age=20&flag=yes&count=2")
		r := httptest.NewRequest(http.MethodPost, "/", body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var p payload
		require.NoError(t, handler.DecodeRequestBodyForTest(r, &p))
		if p.Name != "bob" || p.Age != 20 || !p.Flag || p.Count != 2 {
			t.Fatalf("%+v", p)
		}
	})

	t.Run("query only", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/?name=cara&age=1&flag=1&count=9", nil)
		var p payload
		require.NoError(t, handler.DecodeRequestForTest(r, &p))
		if p.Name != "cara" || p.Age != 1 || !p.Flag || p.Count != 9 {
			t.Fatalf("%+v", p)
		}
	})

	t.Run("empty body content length 0", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/?name=d", http.NoBody)
		r.ContentLength = 0
		var p payload
		require.NoError(t, handler.DecodeRequestBodyForTest(r, &p))
		if p.Name != "d" {
			t.Fatalf("%+v", p)
		}
	})

	t.Run("requestHasBody chunked", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
		r.ContentLength = -1
		if !handler.RequestHasBodyForTest(r) {
			t.Fatal("expected body")
		}
		r2 := httptest.NewRequest(http.MethodPost, "/", nil)
		r2.Body = nil
		if handler.RequestHasBodyForTest(r2) {
			t.Fatal("expected no body")
		}
	})
}

func TestFlexibleUnmarshal(t *testing.T) {
	s, err := handler.UnmarshalFlexibleStringForTest([]byte(`"۱۲۳"`))
	require.NoError(t, err)
	if s != "123" {
		t.Fatalf("got %q", s)
	}
	s, err = handler.UnmarshalFlexibleStringForTest([]byte(`45`))
	require.NoError(t, err)
	if s != "45" {
		t.Fatalf("got %q", s)
	}
	s, err = handler.UnmarshalFlexibleStringForTest([]byte(`null`))
	require.NoError(t, err)
	if s != "" {
		t.Fatalf("got %q", s)
	}
	s, err = handler.UnmarshalFlexibleStringForTest([]byte(`  `))
	require.NoError(t, err)
	if s != "" {
		t.Fatalf("got %q", s)
	}
	s, err = handler.UnmarshalFlexibleStringForTest([]byte(`3.14`))
	require.NoError(t, err)
	if s != "3.14" && s != "3" {
		t.Fatalf("got %q", s)
	}
	_, err = handler.UnmarshalFlexibleStringForTest([]byte(`{"a":1}`))
	if err == nil {
		t.Fatal("expected flexible string error")
	}

	n, err := handler.UnmarshalFlexibleInt32ForTest([]byte(`"۱۵"`))
	require.NoError(t, err)
	if n != 15 {
		t.Fatalf("got %d", n)
	}
	n, err = handler.UnmarshalFlexibleInt32ForTest([]byte(`22`))
	require.NoError(t, err)
	if n != 22 {
		t.Fatalf("got %d", n)
	}
	n, err = handler.UnmarshalFlexibleInt32ForTest([]byte(`null`))
	require.NoError(t, err)
	if n != 0 {
		t.Fatalf("got %d", n)
	}
	n, err = handler.UnmarshalFlexibleInt32ForTest([]byte(`""`))
	require.NoError(t, err)
	if n != 0 {
		t.Fatalf("got %d", n)
	}
	n, err = handler.UnmarshalFlexibleInt32ForTest([]byte(`9.7`))
	require.NoError(t, err)
	if n != 9 {
		t.Fatalf("got %d", n)
	}
	_, err = handler.UnmarshalFlexibleInt32ForTest([]byte(`"abc"`))
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = handler.UnmarshalFlexibleInt32ForTest([]byte(`true`))
	if err == nil {
		t.Fatal("expected int error for bool")
	}
}

func TestMapServiceErrorToValidationFields(t *testing.T) {
	errs := []error{
		service.ErrInvalidFname,
		service.ErrInvalidLname,
		service.ErrInvalidMelliCode,
		service.ErrInvalidBirthdate,
		service.ErrInvalidProvince,
		service.ErrProvinceRequired,
		service.ErrInvalidGender,
		service.ErrGenderRequired,
		service.ErrVerifyTextIDRequired,
		service.ErrVerifyTextIDNotFound,
		service.ErrVideoRequired,
		service.ErrMelliCardRequired,
		service.ErrMelliCodeNotUnique,
		service.ErrInvalidBankName,
		service.ErrInvalidShabaNum,
		service.ErrInvalidCardNum,
		service.ErrShabaNumNotUnique,
		service.ErrCardNumNotUnique,
		service.ErrInvalidOccupation,
		service.ErrInvalidEducation,
		service.ErrInvalidMemory,
		service.ErrInvalidLovedCity,
		service.ErrInvalidLovedCountry,
		service.ErrInvalidLovedLanguage,
		service.ErrInvalidProblemSolving,
		service.ErrInvalidPrediction,
		service.ErrInvalidAbout,
		service.ErrInvalidPassionKey,
		service.ErrInvalidCheckoutDays,
		service.ErrInvalidAutomaticLogout,
		service.ErrInvalidProfileSetting,
		service.ErrInvalidPrivacyKey,
		service.ErrInvalidPrivacyValue,
		service.ErrInvalidCitizenCode,
		service.ErrInvalidImage,
		service.ErrInvalidOptions,
	}
	for _, e := range errs {
		fields, ok := handler.MapServiceErrorToValidationFieldsForTest(e, "en")
		if !ok || len(fields) == 0 {
			t.Fatalf("expected mapping for %v", e)
		}
	}
	if _, ok := handler.MapServiceErrorToValidationFieldsForTest(errors.New("other"), "en"); ok {
		t.Fatal("expected false")
	}
	err := handler.ReturnValidationErrorForTest(map[string]string{"x": "y"})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("unexpected: %v", err)
	}
}
