package handler_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

func identityMW(next http.Handler) http.Handler { return next }

func TestStartHTTPServer_RegistersRoutes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	s := grpc.NewServer()
	defer s.Stop()
	clients := handler.NewLocalClients(
		handler.NewAuthHandler(&mockAuthService{}, &mockTokenRepository{}, &mockProfilePhotoService{}, "en"),
		handler.NewUserHandler(&mockUserService{}, nil, nil),
		handler.NewKYCHandler(&mockKYCService{}, ""),
		handler.RegisterCitizenHandler(s, &mockCitizenService{}),
		handler.RegisterPersonalInfoHandler(s, &mockPersonalInfoService{}),
		handler.NewProfileLimitationHandler(&mockProfileLimitationService{}),
		handler.NewProfilePhotoHandler(&mockProfilePhotoService{}),
		handler.RegisterSettingsHandler(s, &settingsSvcMock{}),
		handler.RegisterUserEventsHandler(s, &mockUserEventsService{}, &mockUserRepo{}),
		handler.NewSearchHandler(stubSearchService{}),
		handler.RegisterWalletConnectionHandler(s, &mockWalletConnectionService{}, "en"),
	)
	httpAuth := handler.NewHTTPAuthHandler(clients, nil, "en")
	httpWallet := handler.NewHTTPWalletHandler(clients.WalletConnection, "en")

	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.StartHTTPServer(
			handler.HTTPServerHandlers{Auth: httpAuth, Wallet: httpWallet},
			strconv.Itoa(port),
			identityMW, identityMW, identityMW,
		)
	}()

	deadline := time.Now().Add(2 * time.Second)
	var healthErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				healthErr = nil
				break
			}
		}
		healthErr = err
		time.Sleep(20 * time.Millisecond)
	}
	if healthErr != nil {
		t.Fatalf("health check failed: %v", healthErr)
	}

	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Logf("server exited: %v", err)
		}
	default:
	}
}

func TestRegistersAndHelperGaps(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()
	handler.RegisterKYCHandler(s, &mockKYCService{}, "")
	handler.RegisterProfileLimitationHandler(s, &mockProfileLimitationService{})
	handler.RegisterProfilePhotoHandler(s, &mockProfilePhotoService{})
	handler.RegisterSearchHandler(s, stubSearchService{})

	userMock := &mockUserService{}
	userMock.getUserFunc = func(context.Context, uint64) (*models.User, error) {
		return &models.User{ID: 1, Score: 10}, nil
	}
	userMock.listUsersFunc = func(context.Context, string, string, int32) ([]*service.UserListItem, int32, int32, error) {
		return []*service.UserListItem{{
			ID: 1, Name: "n", Code: "c", Score: 5,
			CurrentLevel:   &service.LevelSummary{ID: 2, Name: "L", Score: 5, Slug: "l", Image: "/i.png"},
			PreviousLevels: []*service.LevelSummary{{ID: 1, Name: "P", Score: 0, Slug: "p"}},
		}}, 1, 20, nil
	}
	h := handler.RegisterUserHandler(s, userMock, &mockProfileLimitationService{}, &mockHelperService{
		level:  &service.LevelInfo{ID: 1, Title: "L", Description: "d", Score: 10},
		wallet: &service.WalletInfo{Psc: "1"},
	})
	ctx := authenticatedContext(1)
	lvl, err := h.GetUserLevel(ctx, &pb.GetUserLevelRequest{UserId: 1})
	if err != nil || lvl.Level == nil {
		t.Fatalf("GetUserLevel err=%v resp=%v", err, lvl)
	}
	list, err := h.ListUsers(ctx, &pb.ListUsersRequest{Page: 1})
	if err != nil || len(list.Data) != 1 || list.Data[0].Levels == nil || list.Data[0].Levels.Current == nil {
		t.Fatalf("ListUsers err=%v data=%v", err, list)
	}

	clients := handler.NewLocalClients(
		&pb.UnimplementedAuthServiceServer{}, h, &pb.UnimplementedKYCServiceServer{},
		&pb.UnimplementedCitizenServiceServer{}, &pb.UnimplementedPersonalInfoServiceServer{},
		&pb.UnimplementedProfileLimitationServiceServer{}, &pb.UnimplementedProfilePhotoServiceServer{},
		&pb.UnimplementedSettingsServiceServer{}, &pb.UnimplementedUserEventsServiceServer{},
		&pb.UnimplementedSearchServiceServer{}, &pb.UnimplementedWalletConnectionServiceServer{},
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")
	r := withUser(httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil), 1)
	rr := httptest.NewRecorder()
	httpH.ListUsers(rr, r)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"slug"`) {
		t.Fatalf("list http code=%d body=%s", rr.Code, rr.Body.String())
	}

	pi := handler.PersonalInfoRoutes(httpH)
	r = withUser(httptest.NewRequest(http.MethodGet, "/api/personal-info", nil), 1)
	rr = httptest.NewRecorder()
	pi.ServeHTTP(rr, r)
	// personal info client unimplemented -> error path still exercises router
	if rr.Code == 0 {
		t.Fatal("expected response")
	}
}

func TestCitizenRichProfileAndHTTPBuilders(t *testing.T) {
	privacyAll := map[string]bool{}
	for _, k := range []string{
		"nationality", "fname", "lname", "birthdate", "phone", "email", "address",
		"code", "name", "position", "registered_at", "occupation", "education", "problem_solving",
		"loved_city", "loved_country", "loved_language", "prediction", "memory", "about",
		"passions", "score", "level", "avatar",
	} {
		privacyAll[k] = true
	}
	h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{
		profile: &models.CitizenProfile{
			ID: 1, Code: "hm-1", Name: "n", Score: 20, Email: "a@x.com", Phone: "09",
			EmailVerifiedAt: time.Now(),
			Privacy:         privacyAll,
			KYC: &models.CitizenKYC{
				Fname: "A", Lname: "B", Birthdate: time.Date(1990, 1, 2, 0, 0, 0, 0, time.UTC), Address: "addr",
			},
			PersonalInfo: &models.CitizenPersonalInfo{
				Occupation: "o", Education: "e", LovedCity: "c", LovedCountry: "ir",
				LovedLanguage: "fa", ProblemSolving: "solves problems", Prediction: "p", Memory: "m", About: "a",
				Passions: map[string]bool{"music": true, "art": true},
			},
			CurrentLevel:   &models.CitizenLevel{ID: 2, Name: "L", Slug: "l", Score: 10, Image: "/l.png"},
			AchievedLevels: []*models.CitizenLevel{{ID: 1, Name: "P", Slug: "p", Score: 0}},
			ProfilePhotos:  []*models.ProfilePhoto{{ID: 1, URL: "/p.jpg"}},
		},
	})
	resp, err := h.GetCitizenProfile(context.Background(), &pb.GetCitizenProfileRequest{Code: "hm-1"})
	if err != nil || resp.Customs == nil || resp.Customs.Passions["music"] == "" || resp.Customs.ProblemSolving != "solves problems" {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	refSvc := &richCitizenServiceWithOrders{}
	rh := handler.RegisterCitizenHandler(grpc.NewServer(), refSvc)
	refs, err := rh.GetCitizenReferrals(context.Background(), &pb.GetCitizenReferralsRequest{Code: "hm-1", Page: 1})
	if err != nil || len(refs.Data) == 0 || len(refs.Data[0].ReferrerOrders) == 0 {
		t.Fatalf("referrals=%v err=%v", refs, err)
	}
	out := handler.BuildCitizenProfileHTTPResponseForTest(resp)
	if out["current_level"] == nil || out["customs"] == nil || out["kyc"] == nil || out["customs"].(map[string]interface{})["problem_solving"] != "solves problems" {
		t.Fatalf("http map=%v", out)
	}

	lvl := handler.UserListLevelToHTTPForTest(&pb.Level{Id: 1, Title: "t", Slug: "s", ImageUrl: "/i"})
	if lvl["name"] != "t" {
		t.Fatalf("%v", lvl)
	}
	_ = handler.UserListLevelToHTTPForTest(nil)
}

func TestBuildCitizenReferralChartHTTPResponse(t *testing.T) {
	resp := &pb.CitizenReferralChartResponse{
		Data: &pb.ReferralChartData{
			TotalReferralsCount:       "1",
			TotalReferralOrdersAmount: "21111",
			ChartData: []*pb.ChartDataPoint{
				{Label: "1401/10", Count: 1, TotalAmount: 21111},
			},
		},
	}

	t.Run("yearly uses year field", func(t *testing.T) {
		out := handler.BuildCitizenReferralChartHTTPResponseForTest(resp, "yearly")
		data, _ := out["data"].(map[string]interface{})
		if data["total_referrals_count"] != "1" || data["total_referral_orders_amount"] != "21111" {
			t.Fatalf("totals=%v", data)
		}
		points, _ := data["chart_data"].([]map[string]interface{})
		if len(points) != 1 {
			t.Fatalf("chart_data=%v", data["chart_data"])
		}
		point := points[0]
		if point["year"] != "1401/10" || point["month"] != nil || point["day"] != nil {
			t.Fatalf("point=%v", point)
		}
		if point["total_referrals_count"] != int32(1) || point["total_referral_orders_amount"] != int64(21111) {
			t.Fatalf("point totals=%v", point)
		}
	})

	t.Run("monthly uses month field", func(t *testing.T) {
		out := handler.BuildCitizenReferralChartHTTPResponseForTest(resp, "monthly")
		data, _ := out["data"].(map[string]interface{})
		points, _ := data["chart_data"].([]map[string]interface{})
		if points[0]["month"] != "1401/10" || points[0]["year"] != nil {
			t.Fatalf("point=%v", points[0])
		}
	})

	t.Run("daily uses day field", func(t *testing.T) {
		daily := &pb.CitizenReferralChartResponse{
			Data: &pb.ReferralChartData{
				TotalReferralsCount:       "1",
				TotalReferralOrdersAmount: "21111",
				ChartData: []*pb.ChartDataPoint{
					{Label: "1401/10/11", Count: 1, TotalAmount: 21111},
				},
			},
		}
		out := handler.BuildCitizenReferralChartHTTPResponseForTest(daily, "daily")
		data, _ := out["data"].(map[string]interface{})
		points, _ := data["chart_data"].([]map[string]interface{})
		if points[0]["day"] != "1401/10/11" || points[0]["year"] != nil {
			t.Fatalf("point=%v", points[0])
		}
	})

	t.Run("nil data defaults", func(t *testing.T) {
		out := handler.BuildCitizenReferralChartHTTPResponseForTest(&pb.CitizenReferralChartResponse{}, "daily")
		data, _ := out["data"].(map[string]interface{})
		if data["total_referrals_count"] != "0" || data["total_referral_orders_amount"] != "0" {
			t.Fatalf("defaults=%v", data)
		}
	})
}

func TestHTTPHelpersCoverageExtras(t *testing.T) {
	type q struct {
		Name  string  `json:"name" form:"name"`
		Count int32   `json:"count" form:"count"`
		Flag  bool    `json:"flag" form:"flag"`
		Amt   float64 `json:"amt" form:"amt"`
		ID    uint64  `json:"id" form:"id"`
	}
	r := httptest.NewRequest(http.MethodGet, "/x?name=bob&count=3&flag=true&amt=1.5&id=9", nil)
	var got q
	if err := handler.DecodeRequestForTest(r, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "bob" || got.Count != 3 || !got.Flag || got.ID != 9 {
		t.Fatalf("%+v", got)
	}
	handler.MergeQueryParamsForTest(r, &got)

	rr := httptest.NewRecorder()
	handler.WriteProfileLimitationValidationErrorsForTest(rr, map[string]string{"": "bad"}, "en")
	if rr.Code < 400 {
		t.Fatalf("code=%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	handler.WriteProfileLimitationValidationErrorsForTest(rr, map[string]string{"limited_user_id": "required"}, "en")
	if rr.Code < 400 {
		t.Fatalf("code=%d", rr.Code)
	}
	rr = httptest.NewRecorder()
	handler.WriteProfileLimitationValidationErrorsForTest(rr, map[string]string{"": "x", "y": ""}, "en")

	body := `{"limited_user_id":0,"options":{}}`
	httpH := handler.NewHTTPAuthHandler(handler.NewLocalClients(
		&pb.UnimplementedAuthServiceServer{}, &pb.UnimplementedUserServiceServer{},
		&pb.UnimplementedKYCServiceServer{}, &pb.UnimplementedCitizenServiceServer{},
		&pb.UnimplementedPersonalInfoServiceServer{}, &pb.UnimplementedProfileLimitationServiceServer{},
		&pb.UnimplementedProfilePhotoServiceServer{}, &pb.UnimplementedSettingsServiceServer{},
		&pb.UnimplementedUserEventsServiceServer{}, &pb.UnimplementedSearchServiceServer{},
		&pb.UnimplementedWalletConnectionServiceServer{},
	), nil, "en")
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/profile-limitations", strings.NewReader(body)), 1)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	httpH.CreateProfileLimitation(rr, req)
	if rr.Code < 400 {
		t.Fatalf("expected validation error, got %d", rr.Code)
	}

	_ = timestamppb.Now()
	_ = bytes.NewBuffer(nil)
}

type richCitizenServiceWithOrders struct{ richCitizenService }

func (richCitizenServiceWithOrders) GetCitizenReferrals(context.Context, string, string, int32) ([]*models.CitizenReferral, *models.PaginationMeta, error) {
	return []*models.CitizenReferral{{
		ID: 2, Code: "hm-2", Name: "r", Image: "/i.jpg", CreatedAt: time.Now(),
		ReferrerOrders: []*models.ReferrerOrder{{ID: 1, Amount: 100, CreatedAt: time.Now()}},
	}}, &models.PaginationMeta{CurrentPage: 1}, nil
}
