package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
)

type fakeCitizenRepository struct {
	userInfo  *models.CitizenUserInfo
	profile   *models.CitizenProfile
	referrals []*models.CitizenReferral
	meta      *models.PaginationMeta
	orders    []*models.ReferrerOrder
	chart     *models.ReferralChartData
	err       error
	ordersErr error
}

func (f *fakeCitizenRepository) GetCitizenUserInfoByCode(context.Context, string) (*models.CitizenUserInfo, error) {
	return f.userInfo, f.err
}
func (f *fakeCitizenRepository) GetCitizenByCode(context.Context, string) (*models.CitizenProfile, error) {
	return f.profile, f.err
}
func (f *fakeCitizenRepository) GetCitizenReferrals(context.Context, uint64, string, int, int) ([]*models.CitizenReferral, *models.PaginationMeta, error) {
	return f.referrals, f.meta, f.err
}
func (f *fakeCitizenRepository) GetCitizenReferralOrders(context.Context, uint64) ([]*models.ReferrerOrder, error) {
	return f.orders, f.ordersErr
}
func (f *fakeCitizenRepository) GetCitizenReferralChartData(context.Context, uint64, string) (*models.ReferralChartData, error) {
	return f.chart, f.err
}
func (f *fakeCitizenRepository) GetCitizenLevels(context.Context, uint64) (*models.CitizenLevel, []*models.CitizenLevel, error) {
	return nil, nil, nil
}

var _ repository.CitizenRepository = (*fakeCitizenRepository)(nil)

type stubHelper struct {
	pct      float64
	err      error
	level    *service.LevelInfo
	levelErr error
}

func (s *stubHelper) GetHourlyProfitTimePercentage(context.Context, uint64) (float64, error) {
	return 0, nil
}
func (s *stubHelper) GetScorePercentageToNextLevel(context.Context, uint64, int32) (float64, error) {
	return s.pct, s.err
}
func (s *stubHelper) GetUserLevel(context.Context, uint64) (*service.LevelInfo, error) {
	if s.levelErr != nil {
		return nil, s.levelErr
	}
	return s.level, nil
}
func (s *stubHelper) GetUserWallet(context.Context, uint64) (*service.WalletInfo, error) {
	return nil, errors.New("n/a")
}
func (s *stubHelper) CreateWallet(context.Context, uint64) error        { return nil }
func (s *stubHelper) CreateUserVariables(context.Context, uint64) error { return nil }
func (s *stubHelper) Close() error                                      { return nil }

var _ service.HelperService = (*stubHelper)(nil)

func TestCitizenService(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepository(map[uint64]*models.User{
		1: {ID: 1, Code: "hm-1", Score: 40},
	})
	users.getUserLatestLevelFunc = func(context.Context, uint64) (*repository.UserLevel, error) {
		return &repository.UserLevel{ID: 2, Name: "L", Slug: "l", Score: 10, Image: "/l.png"}, nil
	}
	users.getLevelsBelowScoreFunc = func(context.Context, int32) ([]*repository.UserLevel, error) {
		return []*repository.UserLevel{{ID: 1, Name: "P", Slug: "p", Score: 1}}, nil
	}

	t.Run("user info and profile", func(t *testing.T) {
		repo := &fakeCitizenRepository{
			userInfo: &models.CitizenUserInfo{UserID: 1, Privacy: map[string]int32{"score": 1}},
			profile: &models.CitizenProfile{
				ID: 1, Code: "hm-1", Score: 40,
				Privacy: map[string]bool{"level": true},
			},
		}
		svc := service.NewCitizenService(repo, users, &stubHelper{pct: 12.5, level: &service.LevelInfo{Slug: "citizen", Score: 25, Title: "Citizen"}}, "https://app.example")
		info, err := svc.GetCitizenUserInfo(ctx, "hm-1")
		if err != nil || info.UserID != 1 {
			t.Fatalf("%v %v", info, err)
		}
		profile, err := svc.GetCitizenProfile(ctx, "hm-1")
		if err != nil || profile == nil || profile.CurrentLevel == nil || len(profile.AchievedLevels) != 1 {
			t.Fatalf("%+v err=%v", profile, err)
		}
		if svc.ScorePercentageToNextLevel(ctx, 1, 40) != 12.5 {
			t.Fatal("score %")
		}
		level, err := svc.GetCitizenLevel(ctx, "hm-1")
		if err != nil || level == nil || level.Slug != "citizen" || level.Score != 40 {
			t.Fatalf("level=%+v err=%v", level, err)
		}
		level, err = svc.GetCitizenLevel(ctx, "missing")
		if err != nil || level != nil {
			t.Fatalf("expected missing citizen, got %+v err=%v", level, err)
		}
		errHelper := service.NewCitizenService(repo, users, &stubHelper{levelErr: errors.New("levels down")}, "https://app.example")
		if _, err := errHelper.GetCitizenLevel(ctx, "hm-1"); err == nil {
			t.Fatal("expected levels error")
		}
	})

	t.Run("loads levels when the privacy key is absent", func(t *testing.T) {
		repo := &fakeCitizenRepository{profile: &models.CitizenProfile{
			ID: 1, Code: "hm-1", Privacy: map[string]bool{"name": true},
		}}
		svc := service.NewCitizenService(repo, users, nil, "https://app.example")
		profile, err := svc.GetCitizenProfile(ctx, "hm-1")
		if err != nil || profile.CurrentLevel == nil || len(profile.AchievedLevels) != 1 {
			t.Fatalf("profile=%+v err=%v", profile, err)
		}
	})

	t.Run("referrals chart helpers", func(t *testing.T) {
		repo := &fakeCitizenRepository{
			referrals: []*models.CitizenReferral{{ID: 9, Code: "hm-9"}},
			meta:      &models.PaginationMeta{CurrentPage: 1},
			orders:    []*models.ReferrerOrder{{ID: 1, Amount: 100, CreatedAt: time.Now()}},
			chart: &models.ReferralChartData{
				TotalReferralsCount:       "7",
				TotalReferralOrdersAmount: "3333",
			},
		}
		svc := service.NewCitizenService(repo, users, nil, "https://app.example/")
		refs, meta, err := svc.GetCitizenReferrals(ctx, "hm-1", "", 1)
		if err != nil || len(refs) != 1 || meta == nil || len(refs[0].ReferrerOrders) != 1 {
			t.Fatalf("refs=%v meta=%v err=%v", refs, meta, err)
		}
		repo.ordersErr = errors.New("skip")
		refs, _, err = svc.GetCitizenReferrals(ctx, "hm-1", "", 1)
		if err != nil || len(refs[0].ReferrerOrders) != 0 {
			t.Fatalf("expected empty orders, err=%v", err)
		}
		_, _, err = svc.GetCitizenReferrals(ctx, "missing", "", 1)
		if err != nil {
			t.Fatal(err)
		}
		chart, err := svc.GetCitizenReferralChart(ctx, "hm-1", "yearly")
		if err != nil || chart == nil || chart.TotalReferralsCount != "7" || chart.TotalReferralOrdersAmount != "3333" {
			t.Fatalf("%v %v", chart, err)
		}
		chart, err = svc.GetCitizenReferralChart(ctx, "hm-1", "invalid")
		if err != nil || chart == nil {
			t.Fatalf("%v %v", chart, err)
		}
		if svc.AbsoluteURL("") != "" || svc.AbsoluteURL("https://x") != "https://x" {
			t.Fatal("absolute")
		}
		if svc.PassionIconURL("music") == "" || svc.NationalityFlagURL() == "" {
			t.Fatal("urls")
		}
		if svc.CitizenPosition() == "" || svc.CitizenAvatar() == "" {
			t.Fatal("constants")
		}
		if service.FormatRegisteredAt(time.Time{}) != "" {
			t.Fatal("zero registered")
		}
		if service.FormatRegisteredAt(time.Now()) == "" {
			t.Fatal("registered")
		}
		if svc.ScorePercentageToNextLevel(ctx, 1, 1) != 0 {
			t.Fatal("nil helper")
		}
		level, err := svc.GetCitizenLevel(ctx, "hm-1")
		if err != nil || level == nil || level.Slug != "" {
			t.Fatalf("nil helper level=%+v err=%v", level, err)
		}
	})
}
