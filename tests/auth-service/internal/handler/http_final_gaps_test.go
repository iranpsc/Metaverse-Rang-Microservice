package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

func TestHTTPPersonalInfoKYCSearchWalletErrors(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()

	pi := &mockPersonalInfoService{}
	pi.getFunc = func(context.Context, uint64) (*models.PersonalInfo, error) {
		return &models.PersonalInfo{
			Occupation:     sql.NullString{String: "eng", Valid: true},
			Education:      sql.NullString{String: "bs", Valid: true},
			Memory:         sql.NullString{String: "m", Valid: true},
			LovedCity:      sql.NullString{String: "c", Valid: true},
			LovedCountry:   sql.NullString{String: "ir", Valid: true},
			LovedLanguage:  sql.NullString{String: "fa", Valid: true},
			ProblemSolving: sql.NullString{String: "ps", Valid: true},
			Prediction:     sql.NullString{String: "pr", Valid: true},
			About:          sql.NullString{String: "a", Valid: true},
			Passions:       map[string]bool{"music": true},
		}, nil
	}
	piServer := handler.RegisterPersonalInfoHandler(s, pi)

	kycMock := &mockKYCService{}
	kycMock.getKYCFunc = func(context.Context, uint64) (*models.KYC, error) {
		return &models.KYC{
			ID: 1, UserID: 1, Fname: "a", Lname: "b", MelliCode: "0123456789",
			MelliCard: "/c.png", Province: "Tehran", Status: -1,
			Video:     sql.NullString{String: "/v.mp4", Valid: true},
			Gender:    sql.NullString{String: "male", Valid: true},
			Birthdate: sql.NullTime{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			Errors:    sql.NullString{String: `["bad"]`, Valid: true},
		}, nil
	}
	kycServer := handler.NewKYCHandler(kycMock, "https://gw")

	walletMock := &mockWalletConnectionService{}
	walletMock.getLinkNonceFunc = func(context.Context, uint64, string) (string, error) {
		return "", status.Error(codes.InvalidArgument, "bad address")
	}
	walletServer := handler.RegisterWalletConnectionHandler(s, walletMock, "en")

	clients := handler.NewLocalClients(
		&pb.UnimplementedAuthServiceServer{},
		&pb.UnimplementedUserServiceServer{},
		kycServer, &pb.UnimplementedCitizenServiceServer{}, piServer,
		&pb.UnimplementedProfileLimitationServiceServer{},
		&pb.UnimplementedProfilePhotoServiceServer{},
		&pb.UnimplementedSettingsServiceServer{},
		&pb.UnimplementedUserEventsServiceServer{},
		handler.NewSearchHandler(stubSearchService{}),
		walletServer,
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")
	httpW := handler.NewHTTPWalletHandler(clients.WalletConnection, "en")

	r := withUser(httptest.NewRequest(http.MethodGet, "/api/personal-info", nil), 1)
	rr := httptest.NewRecorder()
	httpH.GetPersonalInfo(rr, r)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "occupation") {
		t.Fatalf("pi code=%d body=%s", rr.Code, rr.Body.String())
	}

	r = withUser(httptest.NewRequest(http.MethodGet, "/api/kyc", nil), 1)
	rr = httptest.NewRecorder()
	httpH.GetKYC(rr, r)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "errors") {
		t.Fatalf("kyc code=%d body=%s", rr.Code, rr.Body.String())
	}

	body := bytes.NewBufferString(`{"searchTerm":"ali"}`)
	r = withUser(httptest.NewRequest(http.MethodPost, "/api/search/users", body), 1)
	r.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	httpH.SearchUsers(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("search users code=%d body=%s", rr.Code, rr.Body.String())
	}
	body = bytes.NewBufferString(`{"searchTerm":"feat"}`)
	r = withUser(httptest.NewRequest(http.MethodPost, "/api/search/features", body), 1)
	r.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	httpH.SearchFeatures(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("search features code=%d", rr.Code)
	}
	body = bytes.NewBufferString(`{"searchTerm":"isic"}`)
	r = withUser(httptest.NewRequest(http.MethodPost, "/api/search/isic-codes", body), 1)
	r.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	httpH.SearchIsicCodes(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("search isic code=%d", rr.Code)
	}

	addr := "0x1111111111111111111111111111111111111111"
	r = withUser(httptest.NewRequest(http.MethodGet, "/api/wallet/link/nonce?address="+addr, nil), 1)
	rr = httptest.NewRecorder()
	httpW.GetLinkNonce(rr, r)
	if rr.Code < 400 {
		t.Fatalf("expected wallet grpc error mapping, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPAuthAccountCorePaths(t *testing.T) {
	authSvc := &mockAuthService{}
	authSvc.registerFunc = func(context.Context, string, string) (string, error) { return "https://oauth", nil }
	authSvc.redirectFunc = func(context.Context, string, string) (string, string, error) {
		return "https://oauth", "state", nil
	}
	authSvc.callbackFunc = func(context.Context, string, string, string, bool) (*service.CallbackResult, error) {
		return &service.CallbackResult{Token: "t", ExpiresAt: 3600, RedirectURL: "/"}, nil
	}
	authSvc.getMeFunc = func(context.Context, string) (*service.UserDetails, error) {
		return &service.UserDetails{
			ID: 1, Name: "n", Code: "c", AutomaticLogout: 30,
			HasWallet: true, WalletAddress: "0x1",
			Level: &service.LevelInfo{ID: 1, Title: "L", Description: "d", Score: 10, Slug: "l"},
		}, nil
	}
	authSvc.logoutFunc = func(context.Context, uint64, string, string) error { return nil }
	authSvc.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return &models.User{ID: 1, Email: "a@x.com"}, nil
	}
	s := grpc.NewServer()
	defer s.Stop()
	tokenRepo := &mockTokenRepository{}
	tokenRepo.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return &models.User{ID: 1, Email: "a@x.com"}, nil
	}
	authServer := handler.NewAuthHandler(authSvc, tokenRepo, &mockProfilePhotoService{}, "en")
	clients := handler.NewLocalClients(
		authServer,
		&pb.UnimplementedUserServiceServer{},
		&pb.UnimplementedKYCServiceServer{},
		&pb.UnimplementedCitizenServiceServer{},
		&pb.UnimplementedPersonalInfoServiceServer{},
		&pb.UnimplementedProfileLimitationServiceServer{},
		&pb.UnimplementedProfilePhotoServiceServer{},
		&pb.UnimplementedSettingsServiceServer{},
		&pb.UnimplementedUserEventsServiceServer{},
		&pb.UnimplementedSearchServiceServer{},
		&pb.UnimplementedWalletConnectionServiceServer{},
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")

	rr := httptest.NewRecorder()
	httpH.Register(rr, httptest.NewRequest(http.MethodGet, "/api/auth/register?back_url=/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("register %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	httpH.Redirect(rr, httptest.NewRequest(http.MethodGet, "/api/auth/redirect?redirect_to=x&back_url=/", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("redirect %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	httpH.Callback(rr, httptest.NewRequest(http.MethodGet, "/api/auth/callback?state=s&code=c", nil))
	if rr.Code != http.StatusOK && rr.Code != http.StatusFound {
		t.Fatalf("callback %d %s", rr.Code, rr.Body.String())
	}
	r := withUser(httptest.NewRequest(http.MethodGet, "/api/auth/me", nil), 1)
	r.Header.Set("Authorization", "Bearer tok")
	rr = httptest.NewRecorder()
	httpH.GetMe(rr, r)
	if rr.Code != http.StatusOK {
		t.Fatalf("me %d %s", rr.Code, rr.Body.String())
	}
	r = withUser(httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil), 1)
	rr = httptest.NewRecorder()
	httpH.Logout(rr, r)
	if rr.Code >= 500 {
		t.Fatalf("logout %d %s", rr.Code, rr.Body.String())
	}
	body := strings.NewReader(`{"token":"t"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/validate", body)
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	httpH.ValidateToken(rr, req)
}

func TestHTTPPersonalInfoAndWalletErrorBranches(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()

	pi := &mockPersonalInfoService{}
	pi.getFunc = func(context.Context, uint64) (*models.PersonalInfo, error) {
		return &models.PersonalInfo{}, nil
	}
	pi.updateFunc = func(context.Context, uint64, string, string, string, string, string, string, string, string, string, map[string]bool) error {
		return status.Error(codes.Internal, "boom")
	}
	piServer := handler.RegisterPersonalInfoHandler(s, pi)

	userServer := handler.RegisterUserHandler(
		s,
		&mockUserService{},
		&mockProfileLimitationService{},
		&mockHelperService{wallet: &service.WalletInfo{Psc: "1", Irr: "2"}},
	)

	clients := handler.NewLocalClients(
		&pb.UnimplementedAuthServiceServer{},
		userServer,
		&pb.UnimplementedKYCServiceServer{},
		&pb.UnimplementedCitizenServiceServer{},
		piServer,
		&pb.UnimplementedProfileLimitationServiceServer{},
		&pb.UnimplementedProfilePhotoServiceServer{},
		&pb.UnimplementedSettingsServiceServer{},
		&pb.UnimplementedUserEventsServiceServer{},
		&pb.UnimplementedSearchServiceServer{},
		&pb.UnimplementedWalletConnectionServiceServer{},
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")

	t.Run("get personal info empty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		httpH.GetPersonalInfo(rr, withUser(httptest.NewRequest(http.MethodGet, "/api/personal-info", nil), 1))
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"data":[]`) {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("get personal info unauth", func(t *testing.T) {
		rr := httptest.NewRecorder()
		httpH.GetPersonalInfo(rr, httptest.NewRequest(http.MethodGet, "/api/personal-info", nil))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("update personal info method not allowed", func(t *testing.T) {
		rr := httptest.NewRecorder()
		httpH.UpdatePersonalInfo(rr, withUser(httptest.NewRequest(http.MethodGet, "/api/personal-info", nil), 1))
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("update personal info unauth", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/api/personal-info", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		httpH.UpdatePersonalInfo(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("update personal info empty body", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withUser(httptest.NewRequest(http.MethodPut, "/api/personal-info", strings.NewReader("")), 1)
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = -1
		httpH.UpdatePersonalInfo(rr, req)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "request body is required") {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("update personal info invalid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withUser(httptest.NewRequest(http.MethodPut, "/api/personal-info", strings.NewReader(`{`)), 1)
		req.Header.Set("Content-Type", "application/json")
		httpH.UpdatePersonalInfo(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("update personal info grpc error", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withUser(httptest.NewRequest(http.MethodPut, "/api/personal-info", strings.NewReader(`{"occupation":"x"}`)), 1)
		req.Header.Set("Content-Type", "application/json")
		httpH.UpdatePersonalInfo(rr, req)
		if rr.Code < 400 {
			t.Fatalf("expected error, code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("wallet method not allowed", func(t *testing.T) {
		rr := httptest.NewRecorder()
		httpH.GetAuthenticatedUserWallet(rr, withUser(httptest.NewRequest(http.MethodPost, "/api/wallet", nil), 1))
		if rr.Code != http.StatusMethodNotAllowed {
			t.Fatalf("code=%d", rr.Code)
		}
	})

	t.Run("wallet unauth", func(t *testing.T) {
		rr := httptest.NewRecorder()
		httpH.GetAuthenticatedUserWallet(rr, httptest.NewRequest(http.MethodGet, "/api/wallet", nil))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("code=%d", rr.Code)
		}
	})
}

func TestHTTPAuthErrorMatrix(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()

	authSvc := &mockAuthService{}
	authSvc.registerFunc = func(context.Context, string, string) (string, error) {
		return "", status.Error(codes.Internal, "boom")
	}
	authSvc.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return nil, status.Error(codes.Unauthenticated, "bad token")
	}
	authSvc.requestAccountSecurityFunc = func(context.Context, uint64, int32, string) error {
		return status.Error(codes.InvalidArgument, "bad")
	}
	tokenRepo := &mockTokenRepository{}
	authServer := handler.NewAuthHandler(authSvc, tokenRepo, &mockProfilePhotoService{}, "en")
	kycServer := handler.NewKYCHandler(&mockKYCService{}, "https://gw")
	settingsServer := handler.RegisterSettingsHandler(s, &settingsSvcMock{})
	eventsServer := handler.RegisterUserEventsHandler(s, &mockUserEventsService{}, &mockUserRepo{
		findByIDFunc: func(context.Context, uint64) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
	})
	userServer := handler.RegisterUserHandler(s, &mockUserService{}, &mockProfileLimitationService{}, &mockHelperService{})
	photoServer := handler.NewProfilePhotoHandler(&mockProfilePhotoService{})
	plServer := handler.NewProfileLimitationHandler(&mockProfileLimitationService{})

	clients := handler.NewLocalClients(
		authServer, userServer, kycServer,
		&pb.UnimplementedCitizenServiceServer{},
		&pb.UnimplementedPersonalInfoServiceServer{},
		plServer, photoServer, settingsServer, eventsServer,
		&pb.UnimplementedSearchServiceServer{},
		&pb.UnimplementedWalletConnectionServiceServer{},
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")

	unauth := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{"create bank", httpH.CreateBankAccount, httptest.NewRequest(http.MethodPost, "/api/bank-accounts", strings.NewReader(`{}`))},
		{"update bank", httpH.UpdateBankAccount, httptest.NewRequest(http.MethodPut, "/api/bank-accounts/1", strings.NewReader(`{}`))},
		{"get bank", httpH.GetBankAccount, httptest.NewRequest(http.MethodGet, "/api/bank-accounts/1", nil)},
		{"delete bank", httpH.DeleteBankAccount, httptest.NewRequest(http.MethodDelete, "/api/bank-accounts/1", nil)},
		{"list bank", httpH.ListBankAccounts, httptest.NewRequest(http.MethodGet, "/api/bank-accounts", nil)},
		{"update profile", httpH.UpdateProfile, httptest.NewRequest(http.MethodPut, "/api/user/profile", strings.NewReader(`{}`))},
		{"get user", httpH.GetUser, httptest.NewRequest(http.MethodGet, "/api/user", nil)},
		{"get kyc", httpH.GetKYC, httptest.NewRequest(http.MethodGet, "/api/kyc", nil)},
		{"update general", httpH.UpdateGeneralSettings, httptest.NewRequest(http.MethodPut, "/api/general-settings/1", strings.NewReader(`{}`))},
		{"update privacy", httpH.UpdatePrivacySettings, httptest.NewRequest(http.MethodPost, "/api/privacy", strings.NewReader(`{}`))},
		{"list photos", httpH.ListProfilePhotos, httptest.NewRequest(http.MethodGet, "/api/profilePhotos", nil)},
		{"delete photo", httpH.DeleteProfilePhoto, httptest.NewRequest(http.MethodDelete, "/api/profilePhotos/1", nil)},
		{"report event", httpH.ReportUserEvent, httptest.NewRequest(http.MethodPost, "/api/events/report/1", strings.NewReader(`{}`))},
		{"send report", httpH.SendReportResponse, httptest.NewRequest(http.MethodPost, "/api/events/report/response/1", strings.NewReader(`{}`))},
		{"close report", httpH.CloseEventReport, httptest.NewRequest(http.MethodPost, "/api/events/report/close/1", nil)},
		{"get event", httpH.GetUserEvent, httptest.NewRequest(http.MethodGet, "/api/events/1", nil)},
		{"account security", httpH.RequestAccountSecurity, httptest.NewRequest(http.MethodPost, "/api/account/security", strings.NewReader(`{}`))},
		{"verify security", httpH.VerifyAccountSecurity, httptest.NewRequest(http.MethodPost, "/api/account/security/verify", strings.NewReader(`{}`))},
		{"send mobile", httpH.SendMobileChangeCode, httptest.NewRequest(http.MethodPost, "/api/mobile/send", strings.NewReader(`{}`))},
		{"verify mobile", httpH.VerifyMobileChange, httptest.NewRequest(http.MethodPost, "/api/mobile/verify", strings.NewReader(`{}`))},
		{"update pl", httpH.UpdateProfileLimitation, httptest.NewRequest(http.MethodPut, "/api/profile-limitations/1", strings.NewReader(`{}`))},
		{"delete pl", httpH.DeleteProfileLimitation, httptest.NewRequest(http.MethodDelete, "/api/profile-limitations/1", nil)},
	}
	for _, tc := range unauth {
		t.Run("unauth_"+tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tc.fn(rr, tc.req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
			}
		})
	}

	emptyJSON := func(method, url string) *http.Request {
		r := withUser(httptest.NewRequest(method, url, strings.NewReader("")), 1)
		r.Header.Set("Content-Type", "application/json")
		r.ContentLength = -1
		return r
	}
	badJSON := func(method, url string) *http.Request {
		r := withUser(httptest.NewRequest(method, url, strings.NewReader(`{`)), 1)
		r.Header.Set("Content-Type", "application/json")
		return r
	}

	bodyCases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		req  *http.Request
	}{
		{"register empty", httpH.Register, emptyJSON(http.MethodPost, "/api/auth/register")},
		{"register bad", httpH.Register, badJSON(http.MethodPost, "/api/auth/register")},
		{"validate empty", httpH.ValidateToken, emptyJSON(http.MethodPost, "/api/auth/validate")},
		{"validate bad", httpH.ValidateToken, badJSON(http.MethodPost, "/api/auth/validate")},
		{"create bank empty", httpH.CreateBankAccount, emptyJSON(http.MethodPost, "/api/bank-accounts")},
		{"create bank bad", httpH.CreateBankAccount, badJSON(http.MethodPost, "/api/bank-accounts")},
		{"update bank empty", httpH.UpdateBankAccount, emptyJSON(http.MethodPut, "/api/bank-accounts/1")},
		{"update profile empty", httpH.UpdateProfile, emptyJSON(http.MethodPut, "/api/user/profile")},
		{"update profile bad", httpH.UpdateProfile, badJSON(http.MethodPut, "/api/user/profile")},
		{"update general empty", httpH.UpdateGeneralSettings, emptyJSON(http.MethodPut, "/api/general-settings/1")},
		{"update general bad", httpH.UpdateGeneralSettings, badJSON(http.MethodPut, "/api/general-settings/1")},
		{"update privacy empty", httpH.UpdatePrivacySettings, emptyJSON(http.MethodPost, "/api/privacy")},
		{"update privacy bad", httpH.UpdatePrivacySettings, badJSON(http.MethodPost, "/api/privacy")},
		{"report empty", httpH.ReportUserEvent, emptyJSON(http.MethodPost, "/api/events/report/1")},
		{"report bad", httpH.ReportUserEvent, badJSON(http.MethodPost, "/api/events/report/1")},
		{"send report empty", httpH.SendReportResponse, emptyJSON(http.MethodPost, "/api/events/report/response/1")},
		{"account security empty", httpH.RequestAccountSecurity, emptyJSON(http.MethodPost, "/api/account/security")},
		{"account security bad", httpH.RequestAccountSecurity, badJSON(http.MethodPost, "/api/account/security")},
		{"send mobile empty", httpH.SendMobileChangeCode, emptyJSON(http.MethodPost, "/api/mobile/send")},
		{"send mobile bad", httpH.SendMobileChangeCode, badJSON(http.MethodPost, "/api/mobile/send")},
		{"verify mobile empty", httpH.VerifyMobileChange, emptyJSON(http.MethodPost, "/api/mobile/verify")},
		{"verify mobile bad", httpH.VerifyMobileChange, badJSON(http.MethodPost, "/api/mobile/verify")},
	}
	for _, tc := range bodyCases {
		t.Run("body_"+tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			tc.fn(rr, tc.req)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
			}
		})
	}

	t.Run("general settings invalid id", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withUser(httptest.NewRequest(http.MethodPut, "/api/general-settings/abc", strings.NewReader(`{}`)), 1)
		req.Header.Set("Content-Type", "application/json")
		httpH.UpdateGeneralSettings(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("general settings missing id", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := withUser(httptest.NewRequest(http.MethodPut, "/api/general-settings/", strings.NewReader(`{}`)), 1)
		req.Header.Set("Content-Type", "application/json")
		httpH.UpdateGeneralSettings(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})
}
