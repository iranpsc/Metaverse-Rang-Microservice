package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	sharedauth "metarang/shared/pkg/auth"
)

func authInjectMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), sharedauth.UserContextKey{}, &sharedauth.UserContext{
			UserID: 1, Token: "tok", Email: "a@x.com",
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestStartHTTPServer_LiveRouteCoverage(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	base := "http://127.0.0.1:" + strconv.Itoa(port)

	s := grpc.NewServer()
	defer s.Stop()

	authSvc := &mockAuthService{}
	authSvc.registerFunc = func(context.Context, string, string) (string, error) { return "https://o", nil }
	authSvc.getMeFunc = func(context.Context, string) (*service.UserDetails, error) {
		return &service.UserDetails{ID: 1, Name: "n", Code: "c", AutomaticLogout: 30}, nil
	}
	authSvc.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return &models.User{ID: 1, Email: "a@x.com"}, nil
	}
	authSvc.logoutFunc = func(context.Context, uint64, string, string) error { return nil }
	authSvc.requestAccountSecurityFunc = func(context.Context, uint64, int32, string) error { return nil }
	tokenRepo := &mockTokenRepository{}
	tokenRepo.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return &models.User{ID: 1}, nil
	}
	photo := &mockProfilePhotoService{gatewayURL: "https://cdn"}
	photo.listPhotosFunc = func(context.Context, uint64) ([]*models.Image, error) {
		return []*models.Image{{ID: 1, URL: "/p.jpg"}}, nil
	}

	kyc := &mockKYCService{}
	kyc.getKYCFunc = func(context.Context, uint64) (*models.KYC, error) {
		return &models.KYC{ID: 1, UserID: 1, Fname: "a", Lname: "b", MelliCode: "1"}, nil
	}
	kyc.listBankAccountsFunc = func(context.Context, uint64) ([]*models.BankAccount, error) {
		return []*models.BankAccount{}, nil
	}
	kyc.createBankAccountFunc = func(context.Context, uint64, string, string, string) (*models.BankAccount, error) {
		return &models.BankAccount{ID: 1, BankName: "b", ShabaNum: testShebaNum, CardNum: testCardNum}, nil
	}

	user := &mockUserService{}
	user.listUsersFunc = func(context.Context, string, string, int32) ([]*service.UserListItem, int32, int32, error) {
		return []*service.UserListItem{{ID: 1, Name: "n", Code: "c"}}, 1, 20, nil
	}
	user.getUserFunc = func(context.Context, uint64) (*models.User, error) {
		return &models.User{
			ID: 1, Name: "n", Email: "a@x.com", Code: "c",
			EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
		}, nil
	}
	user.getUserLevelsFunc = func(context.Context, uint64) (*service.UserLevelsData, error) {
		return &service.UserLevelsData{}, nil
	}
	user.getUserProfileFunc = func(context.Context, uint64, *uint64) (*service.UserProfileData, error) {
		return &service.UserProfileData{ID: 1, Code: "c"}, nil
	}
	user.getUserFeaturesCountFunc = func(context.Context, uint64) (*service.UserFeaturesCountData, error) {
		return &service.UserFeaturesCountData{}, nil
	}
	user.updateProfileFunc = func(context.Context, uint64, string, string, string) (*models.User, error) {
		return &models.User{ID: 1, Name: "nn", Code: "c"}, nil
	}

	events := &mockUserEventsService{}
	events.listUserEventsFunc = func(context.Context, uint64, int32) ([]*models.UserEvent, string, string, error) {
		return []*models.UserEvent{}, "", "", nil
	}
	events.getUserEventFunc = func(context.Context, uint64, uint64) (*models.UserEvent, *models.UserEventReport, []*models.UserEventReportResponse, error) {
		return &models.UserEvent{ID: 1, UserID: 1, Event: "e", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil, nil, nil
	}
	events.reportUserEventFunc = func(context.Context, uint64, uint64, *string, string) (*models.UserEventReport, error) {
		return &models.UserEventReport{ID: 1, UserEventID: 1, EventDescription: "d", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}
	events.sendReportResponseFunc = func(context.Context, uint64, uint64, string, string) (*models.UserEventReportResponse, error) {
		return &models.UserEventReportResponse{ID: 1, Response: "r", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}
	events.closeEventReportFunc = func(context.Context, uint64, uint64) error { return nil }

	pl := &mockProfileLimitationService{}
	pl.createFunc = func(context.Context, uint64, uint64, models.ProfileLimitationOptions, service.NoteUpdate) (*models.ProfileLimitation, error) {
		return &models.ProfileLimitation{ID: 1, LimiterUserID: 1, LimitedUserID: 2, Options: models.DefaultOptions()}, nil
	}
	pl.updateFunc = func(context.Context, uint64, uint64, models.ProfileLimitationOptions, service.NoteUpdate) (*models.ProfileLimitation, error) {
		return &models.ProfileLimitation{ID: 1, LimiterUserID: 1, LimitedUserID: 2, Options: models.DefaultOptions()}, nil
	}
	pl.deleteFunc = func(context.Context, uint64, uint64) error { return nil }
	pl.getBetweenUsersFunc = func(context.Context, uint64, uint64) (*models.ProfileLimitation, error) {
		return nil, nil
	}

	clients := handler.NewLocalClients(
		handler.NewAuthHandler(authSvc, tokenRepo, photo, "en"),
		handler.RegisterUserHandler(s, user, pl, &mockHelperService{wallet: &service.WalletInfo{Psc: "1"}}),
		handler.NewKYCHandler(kyc, "https://gw"),
		handler.RegisterCitizenHandler(s, &richCitizenService{}),
		handler.RegisterPersonalInfoHandler(s, &mockPersonalInfoService{}),
		handler.NewProfileLimitationHandler(pl),
		handler.NewProfilePhotoHandler(photo),
		handler.RegisterSettingsHandler(s, &settingsSvcMock{}),
		handler.RegisterUserEventsHandler(s, events, &mockUserRepo{
			findByIDFunc: func(context.Context, uint64) (*models.User, error) { return &models.User{ID: 1}, nil },
		}),
		handler.NewSearchHandler(stubSearchService{}),
		handler.RegisterWalletConnectionHandler(s, &mockWalletConnectionService{}, "en"),
	)
	httpAuth := handler.NewHTTPAuthHandler(clients, nil, "en")
	httpWallet := handler.NewHTTPWalletHandler(clients.WalletConnection, "en")

	go func() {
		_ = handler.StartHTTPServer(
			handler.HTTPServerHandlers{Auth: httpAuth, Wallet: httpWallet},
			strconv.Itoa(port),
			authInjectMW, authInjectMW, identityMW,
		)
	}()

	waitHTTP(t, base+"/health")

	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	get := func(path string) {
		resp, err := client.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	postJSON := func(path, body string) {
		resp, err := client.Post(base+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	do := func(method, path, body string) {
		var rdr io.Reader
		if body != "" {
			rdr = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, base+path, rdr)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	get("/api/auth/register?back_url=/")
	get("/api/auth/me")
	postJSON("/api/auth/logout", "")
	postJSON("/api/auth/validate", `{"token":"t"}`)
	postJSON("/api/account/security", `{"time":15}`)
	postJSON("/api/mobile/send", `{"mobile":"09121112233"}`)
	postJSON("/api/mobile/verify", `{"code":"123456"}`)
	get("/api/users")
	get("/api/user")
	get("/api/user/wallet")
	get("/api/user/hm-1/level")
	do(http.MethodPut, "/api/user/profile", `{"name":"n","email":"a@x.com","phone":"09120000000"}`)
	get("/api/users/1/levels")
	get("/api/users/1/profile")
	get("/api/users/1/features/count")
	get("/api/users/1/wallet")
	get("/api/users/2/profile-limitations")
	get("/api/citizen/hm-1")
	get("/api/citizen/hm-1/referrals")
	get("/api/citizen/hm-1/referrals/chart?range=daily")
	postJSON("/api/search/users", `{"searchTerm":"a"}`)
	postJSON("/api/search/features", `{"searchTerm":"a"}`)
	postJSON("/api/search/isic-codes", `{"searchTerm":"a"}`)
	get("/api/kyc")
	get("/api/bank-accounts")
	postJSON("/api/bank-accounts", `{"bank_name":"Tejarat","shaba_num":"`+testShebaNum+`","card_num":"`+testCardNum+`"}`)
	get("/api/personal-info")
	do(http.MethodPut, "/api/personal-info", `{"occupation":"eng","passions":{"music":true}}`)
	createPL := `{"limited_user_id":2,"options":{"follow":true,"send_message":true,"share":true,"send_ticket":true,"view_profile_images":true,"view_features_locations":true}}`
	postJSON("/api/profile-limitations", createPL)
	do(http.MethodPut, "/api/profile-limitations/1", `{"options":{"follow":false,"send_message":true,"share":true,"send_ticket":true,"view_profile_images":true,"view_features_locations":true}}`)
	do(http.MethodDelete, "/api/profile-limitations/1", "")
	get("/api/profilePhotos")
	get("/api/settings")
	postJSON("/api/settings", `{"checkout_days_count":5,"automatic_logout":30}`)
	get("/api/general-settings")
	do(http.MethodPut, "/api/general-settings/1", `{"announcements_sms":true}`)
	get("/api/privacy")
	postJSON("/api/privacy", `{"key":"score","value":1}`)
	get("/api/events")
	get("/api/events/1")
	postJSON("/api/events/report/1", `{"event_description":"d"}`)
	postJSON("/api/events/report/response/1", `{"response":"ok"}`)
	postJSON("/api/events/report/close/1", "")
	addr := "0x1111111111111111111111111111111111111111"
	get("/api/wallet/link/nonce?address=" + addr)
	postJSON("/api/wallet/link", `{"address":"`+addr+`","signature":"sig"}`)
	get("/api/wallet/security/nonce?address=" + addr)
	postJSON("/api/wallet/security/verify", `{"address":"`+addr+`","signature":"sig","duration":15}`)
	postJSON("/api/wallets/registered", `{"wallet_address":"0xtf"}`)

	_ = bytes.NewBuffer(nil)
}

func waitHTTP(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server not ready: %s", url)
}
