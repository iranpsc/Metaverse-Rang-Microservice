package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
)

type mockHelperService struct {
	wallet *service.WalletInfo
	level  *service.LevelInfo
}

func (m *mockHelperService) GetHourlyProfitTimePercentage(context.Context, uint64) (float64, error) {
	return 0, nil
}
func (m *mockHelperService) GetScorePercentageToNextLevel(context.Context, uint64, int32) (float64, error) {
	return 12.5, nil
}
func (m *mockHelperService) GetUserLevel(context.Context, uint64) (*service.LevelInfo, error) {
	return m.level, nil
}
func (m *mockHelperService) GetUserWallet(context.Context, uint64) (*service.WalletInfo, error) {
	return m.wallet, nil
}
func (m *mockHelperService) CreateWallet(context.Context, uint64) error        { return nil }
func (m *mockHelperService) CreateUserVariables(context.Context, uint64) error { return nil }
func (m *mockHelperService) Close() error                                      { return nil }

func multipartRequest(t *testing.T, method, url, field, filename, contentType string, data []byte, extra map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range extra {
		_ = w.WriteField(k, v)
	}
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	r := httptest.NewRequest(method, url, &buf)
	r.Header.Set("Content-Type", w.FormDataContentType())
	if contentType != "" {
		// Content-Type on file part is set by CreateFormFile as application/octet-stream usually
		_ = contentType
	}
	return r
}

func TestHTTPAuthHandler_CoverageBatch(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()

	authSvc := &mockAuthService{}
	authSvc.requestAccountSecurityFunc = func(context.Context, uint64, int32, string) error { return nil }
	authSvc.verifyAccountSecurityFunc = func(context.Context, uint64, string, string, string) error { return nil }
	authSvc.validateTokenFunc = func(context.Context, string) (*models.User, error) {
		return &models.User{ID: 1, Email: "a@x.com"}, nil
	}
	photo := &mockProfilePhotoService{gatewayURL: "https://cdn"}
	photo.listPhotosFunc = func(context.Context, uint64) ([]*models.Image, error) {
		return []*models.Image{{ID: 9, URL: "/p.jpg"}}, nil
	}
	photo.uploadPhotoFunc = func(context.Context, uint64, []byte, string, string) (*models.Image, error) {
		return &models.Image{ID: 10, URL: "/up.jpg"}, nil
	}
	photo.getPhotoFunc = func(context.Context, uint64) (*models.Image, error) {
		return &models.Image{ID: 9, URL: "/p.jpg"}, nil
	}
	photo.deletePhotoFunc = func(context.Context, uint64, uint64) error { return nil }
	authServer := handler.NewAuthHandler(authSvc, &mockTokenRepository{}, photo, "en")

	kycMock := &mockKYCService{}
	kycMock.createBankAccountFunc = func(_ context.Context, userID uint64, bankName, shaba, card string) (*models.BankAccount, error) {
		return &models.BankAccount{
			ID: 3, BankableID: userID, BankName: bankName, ShabaNum: shaba, CardNum: card,
			Status: 0, Errors: sql.NullString{String: `["bad"]`, Valid: true},
		}, nil
	}
	kycMock.getBankAccountFunc = func(context.Context, uint64, uint64) (*models.BankAccount, error) {
		return &models.BankAccount{ID: 3, BankName: "Tejarat", ShabaNum: testShebaNum, CardNum: testCardNum, Status: 1,
			Errors: sql.NullString{String: `{"x":1}`, Valid: true}}, nil
	}
	kycMock.updateBankAccountFunc = func(context.Context, uint64, uint64, string, string, string) (*models.BankAccount, error) {
		return &models.BankAccount{ID: 3, BankName: "Melli", ShabaNum: testShebaNum, CardNum: testCardNum}, nil
	}
	kycMock.deleteBankAccountFunc = func(context.Context, uint64, uint64) error { return nil }
	kycMock.listBankAccountsFunc = func(context.Context, uint64) ([]*models.BankAccount, error) {
		return []*models.BankAccount{{ID: 3, BankName: "Tejarat", ShabaNum: testShebaNum, CardNum: testCardNum,
			Errors: sql.NullString{String: "plain", Valid: true}}}, nil
	}
	kycMock.submitKYCFunc = func(context.Context, uint64, service.KYCSubmission) (*models.KYC, error) {
		return &models.KYC{ID: 1, UserID: 1, Fname: "a", Lname: "b", MelliCode: "0123456789"}, nil
	}
	kycServer := handler.NewKYCHandler(kycMock, "https://gw")

	userMock := &mockUserService{}
	userMock.getUserFunc = func(context.Context, uint64) (*models.User, error) {
		return &models.User{
			ID: 1, Name: "n", Email: "a@x.com", Code: "c",
			EmailVerifiedAt: sql.NullTime{Time: time.Now(), Valid: true},
		}, nil
	}
	userMock.updateProfileFunc = func(context.Context, uint64, string, string, string) (*models.User, error) {
		return &models.User{ID: 1, Name: "nn", Email: "b@x.com", Code: "c"}, nil
	}
	userMock.getUserLevelsFunc = func(context.Context, uint64) (*service.UserLevelsData, error) {
		return &service.UserLevelsData{
			LatestLevel: &service.LevelDetail{ID: 1, Name: "L", Score: 10, Slug: "l", Image: "/i.png", GemPngFile: "/gems/l.png"},
			PreviousLevels: []*service.LevelDetail{
				{ID: 0, Name: "P", Score: 0, Slug: "p", GemPngFile: "/gems/p.png"},
			},
			ScorePercentageToNextLevel: 33,
		}, nil
	}
	userMock.getUserProfileFunc = func(context.Context, uint64, *uint64) (*service.UserProfileData, error) {
		name := "n"
		reg := "1400/01/01"
		fc, fg := int32(1), int32(2)
		return &service.UserProfileData{
			ID: 1, Name: &name, Code: "c", RegisteredAt: &reg,
			ProfileImages: []string{"/p.jpg"}, FollowersCount: &fc, FollowingCount: &fg,
		}, nil
	}
	userMock.getUserFeaturesCountFunc = func(context.Context, uint64) (*service.UserFeaturesCountData, error) {
		return &service.UserFeaturesCountData{MaskoniFeaturesCount: 1, TejariFeaturesCount: 2, AmoozeshiFeaturesCount: 3}, nil
	}
	userMock.listUsersFunc = func(context.Context, string, string, int32) ([]*service.UserListItem, int32, int32, error) {
		return []*service.UserListItem{{
			ID: 1, Name: "n", Code: "c",
			CurrentLevel:   &service.LevelSummary{ID: 1, Name: "L", Score: 10, Slug: "l"},
			PreviousLevels: []*service.LevelSummary{{ID: 0, Name: "P", Score: 0, Slug: "p"}},
		}}, 1, 20, nil
	}

	plMock := &mockProfileLimitationService{}
	plMock.getBetweenUsersFunc = func(context.Context, uint64, uint64) (*models.ProfileLimitation, error) {
		return &models.ProfileLimitation{
			ID: 7, LimiterUserID: 1, LimitedUserID: 2,
			Options: models.DefaultOptions(),
			Note:    sql.NullString{String: "n", Valid: true},
		}, nil
	}
	plMock.createFunc = func(context.Context, uint64, uint64, models.ProfileLimitationOptions, service.NoteUpdate) (*models.ProfileLimitation, error) {
		return &models.ProfileLimitation{ID: 8, LimiterUserID: 1, LimitedUserID: 2, Options: models.DefaultOptions()}, nil
	}
	plMock.updateFunc = func(context.Context, uint64, uint64, models.ProfileLimitationOptions, service.NoteUpdate) (*models.ProfileLimitation, error) {
		return &models.ProfileLimitation{ID: 8, LimiterUserID: 1, LimitedUserID: 2, Options: models.DefaultOptions()}, nil
	}
	plMock.deleteFunc = func(context.Context, uint64, uint64) error { return nil }

	helper := &mockHelperService{
		wallet: &service.WalletInfo{Psc: "1", Irr: "2", Red: "3", Blue: "4", Yellow: "5", Satisfaction: "6", Effect: 7},
		level:  &service.LevelInfo{ID: 1, Title: "L", Description: "d", Score: 10, Slug: "l"},
	}
	userServer := handler.RegisterUserHandler(s, userMock, plMock, helper)

	eventsMock := &mockUserEventsService{}
	eventsMock.listUserEventsFunc = func(context.Context, uint64, int32) ([]*models.UserEvent, string, string, error) {
		return []*models.UserEvent{{ID: 1, UserID: 1, Event: "e", IP: "1.1.1.1", Device: "d", CreatedAt: time.Now(), UpdatedAt: time.Now()}}, "/n", "", nil
	}
	eventsMock.getUserEventFunc = func(context.Context, uint64, uint64) (*models.UserEvent, *models.UserEventReport, []*models.UserEventReportResponse, error) {
		return &models.UserEvent{ID: 1, UserID: 1, Event: "e", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil, nil, nil
	}
	eventsMock.reportUserEventFunc = func(context.Context, uint64, uint64, *string, string) (*models.UserEventReport, error) {
		return &models.UserEventReport{ID: 1, UserEventID: 1, EventDescription: "d", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}
	eventsMock.sendReportResponseFunc = func(context.Context, uint64, uint64, string, string) (*models.UserEventReportResponse, error) {
		return &models.UserEventReportResponse{ID: 1, UserEventReportID: 1, Response: "r", CreatedAt: time.Now(), UpdatedAt: time.Now()}, nil
	}
	eventsMock.closeEventReportFunc = func(context.Context, uint64, uint64) error { return nil }
	eventsServer := handler.RegisterUserEventsHandler(s, eventsMock, &mockUserRepo{
		findByIDFunc: func(context.Context, uint64) (*models.User, error) { return &models.User{ID: 1, Name: "n"}, nil },
	})

	citizenServer := handler.RegisterCitizenHandler(s, &richCitizenService{})

	piServer := handler.RegisterPersonalInfoHandler(s, &mockPersonalInfoService{})
	settingsServer := handler.RegisterSettingsHandler(s, &settingsSvcMock{})
	plServer := handler.NewProfileLimitationHandler(plMock)
	ppServer := handler.NewProfilePhotoHandler(photo)
	searchServer := handler.NewSearchHandler(stubSearchService{})
	walletServer := handler.RegisterWalletConnectionHandler(s, &mockWalletConnectionService{}, "en")

	clients := handler.NewLocalClients(
		authServer, userServer, kycServer, citizenServer, piServer,
		plServer, ppServer, settingsServer, eventsServer, searchServer, walletServer,
	)
	httpH := handler.NewHTTPAuthHandler(clients, nil, "en")

	t.Run("account security", func(t *testing.T) {
		body := bytes.NewBufferString(`{"time":15,"phone":"09123456789"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/account/security", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.RequestAccountSecurity(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
		body = bytes.NewBufferString(`{"code":"123456"}`)
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/account/security/verify", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.VerifyAccountSecurity(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("mobile change", func(t *testing.T) {
		body := bytes.NewBufferString(`{"mobile":"09121112233"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/send", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.SendMobileChangeCode(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("send code=%d body=%s", rr.Code, rr.Body.String())
		}
		body = bytes.NewBufferString(`{"code":"123456"}`)
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/mobile/verify", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.VerifyMobileChange(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("verify code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("bank accounts crud", func(t *testing.T) {
		body := bytes.NewBufferString(`{"bank_name":"Tejarat","shaba_num":"` + testShebaNum + `","card_num":"` + testCardNum + `"}`)
		r := withUser(httptest.NewRequest(http.MethodPost, "/api/bank-accounts", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.CreateBankAccount(rr, r)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/bank-accounts/3", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetBankAccount(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("get code=%d", rr.Code)
		}
		body = bytes.NewBufferString(`{"bank_name":"Melli","shaba_num":"` + testShebaNum + `","card_num":"` + testCardNum + `"}`)
		r = withUser(httptest.NewRequest(http.MethodPut, "/api/bank-accounts/3", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.UpdateBankAccount(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("update code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodDelete, "/api/bank-accounts/3", nil), 1)
		rr = httptest.NewRecorder()
		httpH.DeleteBankAccount(rr, r)
		if rr.Code != http.StatusNoContent {
			t.Fatalf("delete code=%d", rr.Code)
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/bank-accounts", nil), 1)
		rr = httptest.NewRecorder()
		httpH.ListBankAccounts(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("list code=%d", rr.Code)
		}
	})

	t.Run("events", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/events?page=1", nil), 1)
		rr := httptest.NewRecorder()
		httpH.ListUserEvents(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("list code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/events/1", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetUserEvent(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("get code=%d", rr.Code)
		}
		body := bytes.NewBufferString(`{"event_description":"d"}`)
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/events/report/1", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.ReportUserEvent(rr, r)
		if rr.Code != http.StatusCreated {
			t.Fatalf("report code=%d body=%s", rr.Code, rr.Body.String())
		}
		body = bytes.NewBufferString(`{"response":"ok"}`)
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/events/report/response/1", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.SendReportResponse(rr, r)
		if rr.Code != http.StatusCreated {
			t.Fatalf("response code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/events/report/close/1", nil), 1)
		rr = httptest.NewRecorder()
		httpH.CloseEventReport(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("close code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("photos", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/profilePhotos", nil), 1)
		rr := httptest.NewRecorder()
		httpH.ListProfilePhotos(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("list code=%d", rr.Code)
		}
		r = withUser(multipartRequest(t, http.MethodPost, "/api/profilePhotos", "image", "a.png", "image/png", []byte("img"), nil), 1)
		rr = httptest.NewRecorder()
		httpH.UploadProfilePhoto(rr, r)
		if rr.Code != http.StatusCreated {
			t.Fatalf("upload code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/profilePhotos/9", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetProfilePhoto(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("get code=%d", rr.Code)
		}
		r = withUser(httptest.NewRequest(http.MethodDelete, "/api/profilePhotos/9", nil), 1)
		rr = httptest.NewRecorder()
		httpH.DeleteProfilePhoto(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("delete code=%d", rr.Code)
		}
	})

	t.Run("profile routes", func(t *testing.T) {
		body := bytes.NewBufferString(`{"name":"nn","email":"b@x.com","phone":"09120000000"}`)
		r := withUser(httptest.NewRequest(http.MethodPut, "/api/user/profile", body), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.UpdateProfile(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("update profile code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/users/1/levels", nil), 1)
		rr = httptest.NewRecorder()
		httpH.HandleUsersRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("levels code=%d body=%s", rr.Code, rr.Body.String())
		}
		var levelsResp struct {
			Data struct {
				LatestLevel map[string]interface{}   `json:"latest_level"`
				Previous    []map[string]interface{} `json:"previous_levels"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &levelsResp); err != nil {
			t.Fatalf("decode levels response: %v", err)
		}
		latestGem, ok := levelsResp.Data.LatestLevel["gem"].(map[string]interface{})
		if !ok || latestGem["png_file"] != "/gems/l.png" {
			t.Fatalf("latest level gem=%v", levelsResp.Data.LatestLevel["gem"])
		}
		if len(levelsResp.Data.Previous) != 1 {
			t.Fatalf("previous levels=%v", levelsResp.Data.Previous)
		}
		prevGem, ok := levelsResp.Data.Previous[0]["gem"].(map[string]interface{})
		if !ok || prevGem["png_file"] != "/gems/p.png" {
			t.Fatalf("previous level gem=%v", levelsResp.Data.Previous[0]["gem"])
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/users/1/profile", nil), 1)
		rr = httptest.NewRecorder()
		httpH.HandleUsersRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("profile code=%d", rr.Code)
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/users/1/features/count", nil), 1)
		rr = httptest.NewRecorder()
		httpH.HandleUsersRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("features code=%d", rr.Code)
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/users/1/wallet", nil), 1)
		rr = httptest.NewRecorder()
		httpH.HandleUsersRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("wallet code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/wallet", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetAuthenticatedUserWallet(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("auth wallet code=%d", rr.Code)
		}
	})

	t.Run("profile limitations http", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodGet, "/api/users/2/profile-limitations", nil), 1)
		rr := httptest.NewRecorder()
		httpH.GetProfileLimitations(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("get code=%d body=%s", rr.Code, rr.Body.String())
		}
		createBody := `{"limited_user_id":2,"options":{"follow":true,"send_message":true,"share":true,"send_ticket":true,"view_profile_images":true,"view_features_locations":true},"note":"hi"}`
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/profile-limitations", strings.NewReader(createBody)), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.CreateProfileLimitation(rr, r)
		if rr.Code != http.StatusCreated {
			t.Fatalf("create code=%d body=%s", rr.Code, rr.Body.String())
		}
		upd := `{"options":{"follow":false,"send_message":true,"share":true,"send_ticket":true,"view_profile_images":true,"view_features_locations":true}}`
		r = withUser(httptest.NewRequest(http.MethodPut, "/api/profile-limitations/8", strings.NewReader(upd)), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.UpdateProfileLimitation(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("update code=%d body=%s", rr.Code, rr.Body.String())
		}
		r = withUser(httptest.NewRequest(http.MethodDelete, "/api/profile-limitations/8", nil), 1)
		rr = httptest.NewRecorder()
		httpH.DeleteProfileLimitation(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("delete code=%d", rr.Code)
		}
	})

	t.Run("settings updates", func(t *testing.T) {
		body := `{"announcements_sms":true,"announcements_email":false,"reports_sms":true,"reports_email":false,"login_verification_sms":true,"login_verification_email":false,"transactions_sms":true,"transactions_email":false,"trades_sms":true,"trades_email":false}`
		r := withUser(httptest.NewRequest(http.MethodPut, "/api/general-settings/1", strings.NewReader(body)), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.UpdateGeneralSettings(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("general code=%d body=%s", rr.Code, rr.Body.String())
		}
		body = `{"key":"score","value":true}`
		r = withUser(httptest.NewRequest(http.MethodPost, "/api/privacy", strings.NewReader(body)), 1)
		r.Header.Set("Content-Type", "application/json")
		rr = httptest.NewRecorder()
		httpH.UpdatePrivacySettings(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("privacy code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("citizen referrals", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/referrals?page=1", nil)
		rr := httptest.NewRecorder()
		httpH.HandleCitizenRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("referrals code=%d body=%s", rr.Code, rr.Body.String())
		}
		var listBody map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &listBody); err != nil {
			t.Fatalf("referrals json: %v", err)
		}
		list, _ := listBody["data"].([]interface{})
		if len(list) != 7 {
			t.Fatalf("expected 7 referrals, got %d body=%s", len(list), rr.Body.String())
		}

		r = httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/referrals/chart?range=yearly", nil)
		rr = httptest.NewRecorder()
		httpH.HandleCitizenRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("chart code=%d body=%s", rr.Code, rr.Body.String())
		}
		var chartBody map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &chartBody); err != nil {
			t.Fatalf("chart json: %v", err)
		}
		data, _ := chartBody["data"].(map[string]interface{})
		if data == nil {
			t.Fatalf("missing data: %s", rr.Body.String())
		}
		if data["total_referrals_count"] != "7" || data["total_referral_orders_amount"] != "3333" {
			t.Fatalf("chart totals must match referral list count, got %v", data)
		}
		points, _ := data["chart_data"].([]interface{})
		if len(points) != 2 {
			t.Fatalf("chart_data=%v", data["chart_data"])
		}
		point, _ := points[1].(map[string]interface{})
		if point["year"] != "1401/10" || point["month"] != nil || point["day"] != nil {
			t.Fatalf("point=%v", point)
		}
		if point["total_referrals_count"] != float64(1) || point["total_referral_orders_amount"] != float64(3333) {
			t.Fatalf("point totals=%v", point)
		}

		r = httptest.NewRequest(http.MethodGet, "/api/user/hm-1/level", nil)
		rr = httptest.NewRecorder()
		httpH.GetUserLevelByCode(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("user level code=%d body=%s", rr.Code, rr.Body.String())
		}
		var levelBody map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &levelBody); err != nil {
			t.Fatalf("user level json: %v", err)
		}
		level, _ := levelBody["level"].(map[string]interface{})
		if level["slug"] != "citizen" || level["score"] != "75" {
			t.Fatalf("level=%v body=%s", level, rr.Body.String())
		}

		r = httptest.NewRequest(http.MethodGet, "/api/citizen/hm-1/referrals/chart?range=daily", nil)
		rr = httptest.NewRecorder()
		httpH.HandleCitizenRoutes(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("daily chart code=%d body=%s", rr.Code, rr.Body.String())
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &chartBody); err != nil {
			t.Fatalf("daily chart json: %v", err)
		}
		data, _ = chartBody["data"].(map[string]interface{})
		points, _ = data["chart_data"].([]interface{})
		point, _ = points[0].(map[string]interface{})
		if point["day"] != "1402/10/11" || point["year"] != nil {
			t.Fatalf("daily point=%v", point)
		}
	})

	t.Run("personal info update", func(t *testing.T) {
		body := `{"occupation":"eng","education":"bs","memory":"m","loved_city":"c","loved_country":"ir","loved_language":"fa","problem_solving":"p","prediction":"pr","about":"a","passions":{"music":true}}`
		r := withUser(httptest.NewRequest(http.MethodPut, "/api/personal-info", strings.NewReader(body)), 1)
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		httpH.UpdatePersonalInfo(rr, r)
		if rr.Code != http.StatusOK && rr.Code != http.StatusNoContent {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("kyc update multipart", func(t *testing.T) {
		r := withUser(multipartRequest(t, http.MethodPut, "/api/kyc", "melli_card", "card.png", "image/png", []byte("img"), map[string]string{
			"fname": "a", "lname": "b", "melli_code": "0123456789", "birthdate": "1403/01/15",
			"province": "Tehran", "verify_text_id": "1", "gender": "male",
			"video[path]": "tmp", "video[name]": "v.mp4",
		}), 1)
		rr := httptest.NewRecorder()
		httpH.UpdateKYC(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("require verified email and grpc error locale", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		})
		r := withUser(httptest.NewRequest(http.MethodGet, "/x", nil), 1)
		rr := httptest.NewRecorder()
		httpH.RequireVerifiedEmail(next).ServeHTTP(rr, r)
		if rr.Code != http.StatusOK {
			t.Fatalf("verified code=%d", rr.Code)
		}

		userMock.getUserFunc = func(context.Context, uint64) (*models.User, error) {
			return &models.User{ID: 1, Email: "a@x.com"}, nil
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/x", nil), 1)
		rr = httptest.NewRecorder()
		httpH.RequireVerifiedEmail(next).ServeHTTP(rr, r)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("unverified code=%d", rr.Code)
		}

		userMock.getUserLevelsFunc = func(context.Context, uint64) (*service.UserLevelsData, error) {
			return nil, context.DeadlineExceeded
		}
		r = withUser(httptest.NewRequest(http.MethodGet, "/api/users/1/levels", nil), 1)
		rr = httptest.NewRecorder()
		httpH.GetUserLevels(rr, r)
		if rr.Code < 400 {
			t.Fatalf("expected error code, got %d", rr.Code)
		}
	})

	_ = timestamppb.Now()
}

type richCitizenService struct{}

func (richCitizenService) GetCitizenUserInfo(context.Context, string) (*models.CitizenUserInfo, error) {
	return &models.CitizenUserInfo{UserID: 1}, nil
}
func (richCitizenService) GetCitizenLevel(context.Context, string) (*models.CitizenLevel, error) {
	return &models.CitizenLevel{Slug: "citizen", Score: 75}, nil
}
func (richCitizenService) GetCitizenProfile(context.Context, string) (*models.CitizenProfile, error) {
	return &models.CitizenProfile{ID: 1, Code: "hm-1", Name: "n"}, nil
}
func (richCitizenService) GetCitizenReferrals(context.Context, string, string, int32) ([]*models.CitizenReferral, *models.PaginationMeta, error) {
	refs := make([]*models.CitizenReferral, 0, 7)
	for i := 0; i < 6; i++ {
		refs = append(refs, &models.CitizenReferral{ID: uint64(10 + i), Code: "hm-2", Name: "r"})
	}
	refs = append(refs, &models.CitizenReferral{
		ID: 7, Code: "hm-2000004", Name: "r", Image: "/i.jpg",
		ReferrerOrders: []*models.ReferrerOrder{{ID: 2, Amount: 3333}},
	})
	return refs, &models.PaginationMeta{CurrentPage: 1, NextPageURL: "?page=2"}, nil
}
func (richCitizenService) GetCitizenReferralChart(context.Context, string, string) (*models.ReferralChartData, error) {
	return &models.ReferralChartData{
		TotalReferralsCount: "7", TotalReferralOrdersAmount: "3333",
		ChartData: []*models.ChartDataPoint{
			{Label: "1402/10/11", Count: 6, TotalAmount: 0},
			{Label: "1401/10", Count: 1, TotalAmount: 3333},
		},
	}, nil
}
func (richCitizenService) AbsoluteURL(path string) string { return "https://app" + path }
func (richCitizenService) PassionIconURL(string) string   { return "https://app/p.png" }
func (richCitizenService) NationalityFlagURL() string     { return "https://app/f.png" }
func (richCitizenService) CitizenPosition() string        { return "pos" }
func (richCitizenService) CitizenAvatar() string          { return "https://app/a.png" }
func (richCitizenService) ScorePercentageToNextLevel(context.Context, uint64, int32) float64 {
	return 1
}

var _ service.CitizenService = (*richCitizenService)(nil)
