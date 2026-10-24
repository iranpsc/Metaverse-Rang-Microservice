package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/buildings-service/internal/handler"
	pb "metarang/shared/pb/features"
)

type mockEntryHTTPAPI struct {
	get      func(*pb.GetBuildingEntryConfigRequest) (*pb.GetBuildingEntryConfigResponse, error)
	set      func(*pb.SetBuildingEntryConfigRequest) (*pb.SetBuildingEntryConfigResponse, error)
	create   func(*pb.CreateBuildingEntryCouponRequest) (*pb.BuildingEntryCouponResponse, error)
	listErr  error
	enterErr error
	exitErr  error
}

func (m mockEntryHTTPAPI) GetBuildingEntryConfig(_ context.Context, r *pb.GetBuildingEntryConfigRequest) (*pb.GetBuildingEntryConfigResponse, error) {
	if m.get != nil {
		return m.get(r)
	}
	return &pb.GetBuildingEntryConfigResponse{Config: &pb.BuildingEntryConfig{FeatureId: r.FeatureId, FeePsc: "2.00", IsActive: true}}, nil
}
func (m mockEntryHTTPAPI) SetBuildingEntryConfig(_ context.Context, r *pb.SetBuildingEntryConfigRequest) (*pb.SetBuildingEntryConfigResponse, error) {
	if m.set != nil {
		return m.set(r)
	}
	return &pb.SetBuildingEntryConfigResponse{Config: &pb.BuildingEntryConfig{FeatureId: r.FeatureId, FeePsc: r.FeePsc}}, nil
}
func (m mockEntryHTTPAPI) CreateBuildingEntryCoupon(_ context.Context, r *pb.CreateBuildingEntryCouponRequest) (*pb.BuildingEntryCouponResponse, error) {
	if m.create != nil {
		return m.create(r)
	}
	return &pb.BuildingEntryCouponResponse{Coupon: &pb.BuildingEntryCoupon{FeatureId: r.FeatureId, Code: r.Code, MaxUsageCount: r.MaxUsageCount}}, nil
}
func (m mockEntryHTTPAPI) ListBuildingEntryCoupons(context.Context, *pb.ListBuildingEntryCouponsRequest) (*pb.ListBuildingEntryCouponsResponse, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return &pb.ListBuildingEntryCouponsResponse{Coupons: []*pb.BuildingEntryCoupon{{Id: 7, Code: "SAVE"}}}, nil
}
func (m mockEntryHTTPAPI) EnterBuilding(context.Context, *pb.EnterBuildingRequest) (*pb.EnterBuildingResponse, error) {
	if m.enterErr != nil {
		return nil, m.enterErr
	}
	return &pb.EnterBuildingResponse{Message: "entered"}, nil
}
func (m mockEntryHTTPAPI) ExitBuilding(context.Context, *pb.ExitBuildingRequest) (*pb.ExitBuildingResponse, error) {
	if m.exitErr != nil {
		return nil, m.exitErr
	}
	return &pb.ExitBuildingResponse{Message: "exited"}, nil
}

func TestHTTPEntryRoutes(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	h.SetEntry(mockEntryHTTPAPI{get: func(r *pb.GetBuildingEntryConfigRequest) (*pb.GetBuildingEntryConfigResponse, error) {
		if r.FeatureId != 42 {
			t.Fatalf("feature id=%d", r.FeatureId)
		}
		return nil, status.Error(codes.NotFound, "missing")
	}})
	cases := []struct {
		name     string
		method   string
		body     string
		call     func(http.ResponseWriter, *http.Request)
		contains string
	}{
		{"set", http.MethodPost, `{"fee_psc":2.5,"is_active":true}`, h.SetEntryConfig, `"fee_psc":"2.5"`},
		{"create coupon", http.MethodPost, `{"code":"SAVE","usage_count":5,"discount_percentage":20}`, h.CreateEntryCoupon, `"code":"SAVE"`},
		{"list coupons", http.MethodGet, "", h.ListEntryCoupons, `"code":"SAVE"`},
		{"enter", http.MethodPost, `{"coupon_code":"SAVE"}`, h.EnterBuilding, "entered"},
		{"exit", http.MethodPost, "", h.ExitBuilding, "exited"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := requestWithUser(httptest.NewRequest(tc.method, "/api/features/42/entry", strings.NewReader(tc.body)), 8)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.contains) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	h.SetEntry(mockEntryHTTPAPI{})
	req := requestWithUser(httptest.NewRequest(http.MethodGet, "/api/features/42/entry", nil), 8)
	rec := httptest.NewRecorder()
	h.GetEntryConfig(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"is_active":true`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPEntryValidationAndUnavailable(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	cases := []struct {
		name   string
		method string
		url    string
		body   string
		call   func(http.ResponseWriter, *http.Request)
		want   int
	}{
		{"missing entry", http.MethodGet, "/api/features/42/entry", "", h.GetEntryConfig, 503},
		{"invalid id", http.MethodGet, "/api/features/no/entry", "", h.GetEntryConfig, 400},
		{"set invalid body", http.MethodPost, "/api/features/42/entry", "{", h.SetEntryConfig, 422},
		{"coupon missing usage", http.MethodPost, "/api/features/42/entry", `{"discount_percentage":20}`, h.CreateEntryCoupon, 422},
		{"coupon missing discount", http.MethodPost, "/api/features/42/entry", `{"usage_count":5}`, h.CreateEntryCoupon, 422},
		{"enter invalid body", http.MethodPost, "/api/features/42/entry", "{", h.EnterBuilding, 422},
		{"wrong method", http.MethodGet, "/api/features/42/entry", "", h.ExitBuilding, 404},
		{"set wrong method", http.MethodGet, "/api/features/42/entry", "", h.SetEntryConfig, 404},
		{"get wrong method", http.MethodPost, "/api/features/42/entry", "", h.GetEntryConfig, 404},
		{"coupon wrong method", http.MethodGet, "/api/features/42/entry", "", h.CreateEntryCoupon, 404},
		{"list wrong method", http.MethodPost, "/api/features/42/entry", "", h.ListEntryCoupons, 404},
		{"enter wrong method", http.MethodGet, "/api/features/42/entry", "", h.EnterBuilding, 404},
		{"set invalid id", http.MethodPost, "/api/features/no/entry", `{}`, h.SetEntryConfig, 400},
		{"coupon invalid id", http.MethodPost, "/api/features/no/entry", `{}`, h.CreateEntryCoupon, 400},
		{"list invalid id", http.MethodGet, "/api/features/no/entry", "", h.ListEntryCoupons, 400},
		{"enter invalid id", http.MethodPost, "/api/features/no/entry", "", h.EnterBuilding, 400},
		{"exit invalid id", http.MethodPost, "/api/features/no/entry", "", h.ExitBuilding, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.SetEntry(mockEntryHTTPAPI{})
			if tc.name == "missing entry" {
				h.SetEntry(nil)
			}
			req := requestWithUser(httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body)), 8)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestHTTPEntryUpstreamErrorsAndExistingConfig(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	bad := status.Error(codes.Unavailable, "upstream unavailable")
	cases := []struct {
		name   string
		method string
		body   string
		entry  mockEntryHTTPAPI
		call   func(http.ResponseWriter, *http.Request)
	}{
		{"get", http.MethodGet, "", mockEntryHTTPAPI{get: func(*pb.GetBuildingEntryConfigRequest) (*pb.GetBuildingEntryConfigResponse, error) { return nil, bad }}, h.GetEntryConfig},
		{"set", http.MethodPost, `{}`, mockEntryHTTPAPI{get: func(*pb.GetBuildingEntryConfigRequest) (*pb.GetBuildingEntryConfigResponse, error) { return nil, bad }}, h.SetEntryConfig},
		{"coupon", http.MethodPost, `{"usage_count":1,"discount_percentage":10}`, mockEntryHTTPAPI{create: func(*pb.CreateBuildingEntryCouponRequest) (*pb.BuildingEntryCouponResponse, error) { return nil, bad }}, h.CreateEntryCoupon},
		{"list", http.MethodGet, "", mockEntryHTTPAPI{listErr: bad}, h.ListEntryCoupons},
		{"enter", http.MethodPost, "", mockEntryHTTPAPI{enterErr: bad}, h.EnterBuilding},
		{"exit", http.MethodPost, "", mockEntryHTTPAPI{exitErr: bad}, h.ExitBuilding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.SetEntry(tc.entry)
			req := requestWithUser(httptest.NewRequest(tc.method, "/api/features/42/entry", strings.NewReader(tc.body)), 8)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	h.SetEntry(mockEntryHTTPAPI{get: func(*pb.GetBuildingEntryConfigRequest) (*pb.GetBuildingEntryConfigResponse, error) {
		return &pb.GetBuildingEntryConfigResponse{Config: &pb.BuildingEntryConfig{FeePsc: "3.00", About: "Old", IsActive: true}}, nil
	}, set: func(req *pb.SetBuildingEntryConfigRequest) (*pb.SetBuildingEntryConfigResponse, error) {
		if req.FeePsc != "3.00" || req.About != "Old" || !req.IsActive {
			t.Fatalf("partial config=%+v", req)
		}
		return nil, bad
	}})
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/features/42/entry", strings.NewReader(`{}`)), 8)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.SetEntryConfig(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHTTPEntryAcceptsFormBody(t *testing.T) {
	h := handler.NewHTTPBuildingsHandler(&mockHTTPBuildingAPI{})
	h.SetEntry(mockEntryHTTPAPI{set: func(req *pb.SetBuildingEntryConfigRequest) (*pb.SetBuildingEntryConfigResponse, error) {
		if req.FeatureId != 42 || req.FeePsc != "4.50" || req.About != "museum" {
			t.Fatalf("form config=%+v", req)
		}
		return &pb.SetBuildingEntryConfigResponse{Config: &pb.BuildingEntryConfig{FeatureId: 42, FeePsc: req.FeePsc}}, nil
	}})
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/api/features/42/entry", strings.NewReader("fee_psc=4.50&about=museum")), 8)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.SetEntryConfig(rec, req)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "4.50") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
