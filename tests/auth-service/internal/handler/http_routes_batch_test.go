package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
	sharedauth "metarang/shared/pkg/auth"
)

func withUser(r *http.Request, id uint64) *http.Request {
	ctx := context.WithValue(r.Context(), sharedauth.UserContextKey{}, &sharedauth.UserContext{
		UserID: id, Token: "tok", Email: "a@x.com",
	})
	return r.WithContext(ctx)
}

type stubSearchService struct{}

func (stubSearchService) SearchUsers(context.Context, string) ([]*service.SearchUserResult, error) {
	lvl, photo := "L1", "/p.jpg"
	return []*service.SearchUserResult{{
		ID: 1, Code: "hm-1", Name: "Ali", Followers: 3, Level: &lvl, Photo: &photo,
	}}, nil
}
func (stubSearchService) SearchFeatures(context.Context, string) ([]*service.SearchFeatureResult, error) {
	return []*service.SearchFeatureResult{{
		ID: 1, FeaturePropertiesID: "1", Address: "addr", Karbari: "m",
		PricePsc: "1", PriceIrr: "2", OwnerCode: "hm-1",
		Coordinates: []*service.FeatureCoordinate{{ID: 1, X: 1.1, Y: 2.2}},
	}}, nil
}
func (stubSearchService) SearchIsicCodes(context.Context, string) ([]*service.IsicCodeResult, error) {
	return []*service.IsicCodeResult{{ID: 1, Name: "code", Code: 11}}, nil
}

func TestHTTPAuthHandler_SettingsKYCSearch(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()

	authSvc := &mockAuthService{}
	authSvc.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return &models.User{ID: 1, Email: "a@x.com"}, nil
	}
	authSvc.getMeFunc = func(context.Context, string) (*service.UserDetails, error) {
		return &service.UserDetails{ID: 1, Name: "n", Code: "c", AutomaticLogout: 30}, nil
	}
	tokenRepo := &mockTokenRepository{}
	photo := &mockProfilePhotoService{gatewayURL: "https://cdn"}
	authServer := handler.NewAuthHandler(authSvc, tokenRepo, photo, "en")

	settingsServer := handler.RegisterSettingsHandler(s, &settingsSvcMock{})
	kycMock := &mockKYCService{}
	kycMock.getKYCFunc = func(context.Context, uint64) (*models.KYC, error) {
		return &models.KYC{ID: 1, UserID: 1, Fname: "a", Lname: "b", MelliCode: "001"}, nil
	}
	kycMock.listBankAccountsFunc = func(context.Context, uint64) ([]*models.BankAccount, error) {
		return []*models.BankAccount{}, nil
	}
	kycServer := handler.NewKYCHandler(kycMock, "https://gw")

	userMock := &mockUserService{}
	userMock.listUsersFunc = func(context.Context, string, string, int32) ([]*service.UserListItem, int32, int32, error) {
		return []*service.UserListItem{{ID: 1, Name: "n", Code: "c"}}, 1, 20, nil
	}
	userMock.getUserFunc = func(context.Context, uint64) (*models.User, error) {
		return &models.User{ID: 1, Name: "n", Email: "a@x.com", Code: "c"}, nil
	}
	userMock.getUserProfileFunc = func(context.Context, uint64, *uint64) (*service.UserProfileData, error) {
		name := "n"
		return &service.UserProfileData{ID: 1, Name: &name, Code: "c"}, nil
	}
	userMock.getUserLevelsFunc = func(context.Context, uint64) (*service.UserLevelsData, error) {
		return &service.UserLevelsData{}, nil
	}
	userMock.getUserFeaturesCountFunc = func(context.Context, uint64) (*service.UserFeaturesCountData, error) {
		return &service.UserFeaturesCountData{}, nil
	}
	userServer := handler.NewUserHandler(userMock, nil, nil)

	searchServer := handler.NewSearchHandler(stubSearchService{})
	piServer := handler.RegisterPersonalInfoHandler(s, &mockPersonalInfoService{})
	citizenServer := handler.RegisterCitizenHandler(s, &mockCitizenService{
		userInfo: &models.CitizenUserInfo{UserID: 1},
		profile:  &models.CitizenProfile{ID: 1, Code: "hm-1", Name: "n"},
	})
	eventsServer := handler.RegisterUserEventsHandler(s, &mockUserEventsService{}, &mockUserRepo{
		findByIDFunc: func(context.Context, uint64) (*models.User, error) {
			return &models.User{ID: 1}, nil
		},
	})
	plServer := handler.NewProfileLimitationHandler(&mockProfileLimitationService{})
	ppServer := handler.NewProfilePhotoHandler(photo)

	clients := handler.NewLocalClients(
		authServer, userServer, kycServer, citizenServer, piServer,
		plServer, ppServer, settingsServer, eventsServer, searchServer,
		&pb.UnimplementedWalletConnectionServiceServer{},
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")

	t.Run("settings", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/settings", nil), 1)
		rr := httptest.NewRecorder()
		httpH.GetSettings(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
		var settingsPayload map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &settingsPayload); err != nil {
			t.Fatalf("decode settings: %v", err)
		}
		if _, ok := settingsPayload["available_reset_mobile_resets"]; !ok {
			t.Fatalf("expected available_reset_mobile_resets in settings response, got %s", rr.Body.String())
		}

		body := bytes.NewBufferString(`{"checkout_days_count":5,"automatic_logout":30}`)
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/settings", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.UpdateSettings(rr, r)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}

		r = withUser(httptest.NewRequest(http.MethodGet, "/api/general-settings", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetGeneralSettings(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}

		r = withUser(httptest.NewRequest(http.MethodGet, "/api/privacy-settings", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetPrivacySettings(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("kyc get", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/kyc", nil), 1)
		rr := httptest.NewRecorder()
		httpH.GetKYC(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("search", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/search/users?q=a", nil), 1)
		rr := httptest.NewRecorder()
		httpH.SearchUsers(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/search/features?q=a", nil), 1)
		rr = httptest.NewRecorder()
		httpH.SearchFeatures(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/search/isic-codes?q=a", nil), 1)
		rr = httptest.NewRecorder()
		httpH.SearchIsicCodes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("users list and profile", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil), 1)
		rr := httptest.NewRecorder()
		httpH.ListUsers(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/user", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetUser(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("personal info", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/personal-info", nil), 1)
		rr := httptest.NewRecorder()
		httpH.GetPersonalInfo(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("citizen", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1", nil)
		rr := httptest.NewRecorder()
		httpH.HandleCitizenRoutes(rr, r)
		if rr.Code >= 500 {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("bank accounts list", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/bank-accounts", nil), 1)
		rr := httptest.NewRecorder()
		httpH.ListBankAccounts(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})
}
