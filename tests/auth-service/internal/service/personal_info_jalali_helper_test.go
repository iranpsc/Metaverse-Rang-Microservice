package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
)

type fakePersonalInfoRepository struct {
	info      *models.PersonalInfo
	err       error
	upsertErr error
}

func (f *fakePersonalInfoRepository) FindByUserID(context.Context, uint64) (*models.PersonalInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.info, nil
}

func (f *fakePersonalInfoRepository) Upsert(_ context.Context, info *models.PersonalInfo) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.info = info
	return nil
}

var _ repository.PersonalInfoRepository = (*fakePersonalInfoRepository)(nil)

func TestPersonalInfoService(t *testing.T) {
	ctx := context.Background()

	t.Run("get nil and existing", func(t *testing.T) {
		repo := &fakePersonalInfoRepository{}
		svc := service.NewPersonalInfoService(repo)
		info, err := svc.GetPersonalInfo(ctx, 1)
		if err != nil || info != nil {
			t.Fatalf("info=%v err=%v", info, err)
		}
		repo.info = &models.PersonalInfo{UserID: 1}
		info, err = svc.GetPersonalInfo(ctx, 1)
		if err != nil || info == nil {
			t.Fatalf("info=%v err=%v", info, err)
		}
	})

	t.Run("get error", func(t *testing.T) {
		repo := &fakePersonalInfoRepository{err: errors.New("db")}
		svc := service.NewPersonalInfoService(repo)
		_, err := svc.GetPersonalInfo(ctx, 1)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("update create and validation", func(t *testing.T) {
		repo := &fakePersonalInfoRepository{}
		svc := service.NewPersonalInfoService(repo)
		err := svc.UpdatePersonalInfo(ctx, 1, "job", "edu", "", "", "", "", "", "", "", map[string]bool{"music": true})
		if err != nil {
			t.Fatal(err)
		}
		if repo.info == nil || !repo.info.Passions["music"] || !repo.info.Occupation.Valid {
			t.Fatalf("%+v", repo.info)
		}

		err = svc.UpdatePersonalInfo(ctx, 1, strings.Repeat("a", 256), "", "", "", "", "", "", "", "", nil)
		if !errors.Is(err, service.ErrInvalidOccupation) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", strings.Repeat("a", 256), "", "", "", "", "", "", "", nil)
		if !errors.Is(err, service.ErrInvalidEducation) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", strings.Repeat("a", 10001), "", "", "", "", "", "", nil)
		if !errors.Is(err, service.ErrInvalidMemory) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", strings.Repeat("a", 256), "", "", "", "", "", nil)
		if !errors.Is(err, service.ErrInvalidLovedCity) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", strings.Repeat("a", 256), "", "", "", "", nil)
		if !errors.Is(err, service.ErrInvalidLovedCountry) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", "", strings.Repeat("a", 256), "", "", "", nil)
		if !errors.Is(err, service.ErrInvalidLovedLanguage) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", "", "", strings.Repeat("a", 10001), "", "", nil)
		if !errors.Is(err, service.ErrInvalidProblemSolving) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", "", "", "", strings.Repeat("a", 10001), "", nil)
		if !errors.Is(err, service.ErrInvalidPrediction) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", "", "", "", "", strings.Repeat("a", 10001), nil)
		if !errors.Is(err, service.ErrInvalidAbout) {
			t.Fatalf("err=%v", err)
		}
		err = svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", "", "", "", "", "", map[string]bool{"bad": true})
		if !errors.Is(err, service.ErrInvalidPassionKey) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("update merges passions and upsert error", func(t *testing.T) {
		repo := &fakePersonalInfoRepository{
			info: &models.PersonalInfo{
				ID: 9, UserID: 1,
				Passions: map[string]bool{"music": false, "art": true},
			},
		}
		svc := service.NewPersonalInfoService(repo)
		err := svc.UpdatePersonalInfo(ctx, 1, "", "", "", "", "", "", "", "", "", map[string]bool{"music": true})
		if err != nil {
			t.Fatal(err)
		}
		if !repo.info.Passions["music"] || !repo.info.Passions["art"] || repo.info.ID != 9 {
			t.Fatalf("%+v", repo.info)
		}

		repo.upsertErr = errors.New("fail")
		err = svc.UpdatePersonalInfo(ctx, 1, "x", "", "", "", "", "", "", "", "", nil)
		if err == nil {
			t.Fatal("expected upsert error")
		}
	})
}

func TestFormatJalali(t *testing.T) {
	tm := time.Date(2024, 3, 20, 12, 30, 0, 0, time.UTC)
	if service.FormatJalaliDate(tm) == "" {
		t.Fatal("empty date")
	}
	if service.FormatJalaliDateTime(tm) == "" {
		t.Fatal("empty datetime")
	}
}

func TestResolvePublicURLAndUploadID(t *testing.T) {
	if got := service.ResolvePublicURL("https://gw", "/uploads/a.jpg"); !strings.Contains(got, "https://gw") {
		t.Fatalf("got=%s", got)
	}
	if got := service.ResolvePublicURL("", "https://cdn/x"); got != "https://cdn/x" {
		t.Fatalf("got=%s", got)
	}
	if got := service.PrependGatewayURL("https://gw/", "path/x"); !strings.Contains(got, "path/x") {
		t.Fatalf("got=%s", got)
	}
	if got := service.PrependGatewayURL("", ""); got != "" {
		t.Fatalf("got=%s", got)
	}
	id := service.NewUploadID("kyc", 7)
	if !strings.Contains(id, "kyc") || !strings.Contains(id, "7") {
		t.Fatalf("id=%s", id)
	}
}

func TestHelperServiceEmptyAddrs(t *testing.T) {
	svc := service.NewHelperService("", "", "")
	defer svc.Close()
	pct, err := svc.GetHourlyProfitTimePercentage(context.Background(), 1)
	if err != nil || pct != 0 {
		t.Fatalf("pct=%v err=%v", pct, err)
	}
	score, err := svc.GetScorePercentageToNextLevel(context.Background(), 1, 10)
	if err != nil || score != 0 {
		t.Fatalf("score=%v err=%v", score, err)
	}
	lvl, err := svc.GetUserLevel(context.Background(), 1)
	if err != nil || lvl != nil {
		t.Fatalf("lvl=%v err=%v", lvl, err)
	}
	_, err = svc.GetUserWallet(context.Background(), 1)
	if err == nil {
		t.Fatal("expected wallet unavailable")
	}
	if err := svc.CreateWallet(context.Background(), 1); err == nil {
		t.Fatal("expected create wallet error")
	}
	if err := svc.CreateUserVariables(context.Background(), 1); err == nil {
		t.Fatal("expected create variables error")
	}
}
