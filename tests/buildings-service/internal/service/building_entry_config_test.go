package service_test

import (
	"errors"
	"strings"
	"testing"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/service"
	pb "metarang/shared/pb/features"
	levelspb "metarang/shared/pb/levels"
)

func TestBuildingEntryConfigAndCoupons(t *testing.T) {
	repo := &entryRepoMock{}
	svc := service.NewBuildingEntryService(repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()}, nil, nil)
	ctx := userCtx(1)
	config, err := svc.SetConfig(ctx, &pb.SetBuildingEntryConfigRequest{
		FeatureId: 7, FeePsc: "2.5", FeeIrr: "10", About: " Museum ", IsActive: true,
	})
	if err != nil || config.FeePsc != "2.50" || config.About != "Museum" {
		t.Fatalf("config=%+v err=%v", config, err)
	}
	config, err = svc.GetConfig(ctx, 7)
	if err != nil || config.FeatureId != 7 || !config.IsActive {
		t.Fatalf("read config=%+v err=%v", config, err)
	}
	coupon, err := svc.CreateCoupon(ctx, &pb.CreateBuildingEntryCouponRequest{
		FeatureId: 7, Code: " SAVE20 ", DiscountPercentage: 20, MaxUsageCount: 5,
	})
	if err != nil || coupon.Code != "SAVE20" || coupon.MaxUsageCount != 5 {
		t.Fatalf("coupon=%+v err=%v", coupon, err)
	}
	coupons, err := svc.ListCoupons(ctx, 7)
	if err != nil || len(coupons) != 1 || coupons[0].Code != "SAVE20" {
		t.Fatalf("coupons=%+v err=%v", coupons, err)
	}
}

func TestBuildingEntryConfigValidation(t *testing.T) {
	repo := &entryRepoMock{}
	svc := service.NewBuildingEntryService(repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()}, nil, nil)
	ctx := userCtx(1)
	for _, tc := range []struct {
		name string
		req  *pb.SetBuildingEntryConfigRequest
		want string
	}{
		{"fee", &pb.SetBuildingEntryConfigRequest{FeatureId: 7, FeePsc: "bad"}, "fee_psc"},
		{"about", &pb.SetBuildingEntryConfigRequest{FeatureId: 7, About: strings.Repeat("a", 1001)}, "about"},
		{"scope", &pb.SetBuildingEntryConfigRequest{FeatureId: 7, LevelScopeType: "invalid"}, "scope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SetConfig(ctx, tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	for _, req := range []*pb.CreateBuildingEntryCouponRequest{
		{FeatureId: 7, Code: "bad code", DiscountPercentage: 20, MaxUsageCount: 5},
		{FeatureId: 7, Code: "GOOD", DiscountPercentage: 0, MaxUsageCount: 5},
		{FeatureId: 7, Code: "GOOD", DiscountPercentage: 20, MaxUsageCount: 0},
	} {
		if _, err := svc.CreateCoupon(ctx, req); err == nil {
			t.Fatalf("coupon accepted: %+v", req)
		}
	}
	if _, err := svc.GetConfig(ctx, 7); err == nil {
		t.Fatal("missing config accepted")
	}
}

func TestBuildingEntryScopedConfigAndFailures(t *testing.T) {
	repo := &entryRepoMock{}
	feature := &entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}}
	buildings := &entryBuildingMock{buildings: finishedBuilding()}
	levels := &entryLevelsMock{levels: []*levelspb.Level{{Slug: "bronze", Score: 10}}}
	svc := service.NewBuildingEntryService(repo, feature, buildings, levels, nil)
	for _, scope := range []string{"exact", "and_upper", "and_lower"} {
		config, err := svc.SetConfig(userCtx(1), &pb.SetBuildingEntryConfigRequest{
			FeatureId: 7, FeePsc: "1", FeeIrr: "0", LevelScopeType: scope, LevelSlug: "bronze",
		})
		if err != nil || config.LevelScopeType != scope {
			t.Fatalf("scope=%q config=%+v err=%v", scope, config, err)
		}
	}
	for _, tc := range []struct{ name, scope, slug, want string }{
		{"missing slug", "exact", "", "invalid level scope"},
		{"unknown level", "exact", "missing", "invalid level_slug"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SetConfig(userCtx(1), &pb.SetBuildingEntryConfigRequest{FeatureId: 7, LevelScopeType: tc.scope, LevelSlug: tc.slug})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	withoutLevels := service.NewBuildingEntryService(repo, feature, buildings, nil, nil)
	if _, err := withoutLevels.SetConfig(userCtx(1), &pb.SetBuildingEntryConfigRequest{FeatureId: 7, LevelScopeType: "exact", LevelSlug: "bronze"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("levels error=%v", err)
	}
	repo.saveErr = errors.New("database unavailable")
	if _, err := svc.SetConfig(userCtx(1), &pb.SetBuildingEntryConfigRequest{FeatureId: 7}); err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("save error=%v", err)
	}
	buildings.buildings = nil
	if _, err := svc.SetConfig(userCtx(1), &pb.SetBuildingEntryConfigRequest{FeatureId: 7}); err == nil || !strings.Contains(err.Error(), "building not found") {
		t.Fatalf("building error=%v", err)
	}
	if _, err := svc.CreateCoupon(userCtx(1), &pb.CreateBuildingEntryCouponRequest{FeatureId: 7, Code: "GOOD", DiscountPercentage: 20, MaxUsageCount: 1}); err == nil || !strings.Contains(err.Error(), "building not found") {
		t.Fatalf("coupon building error=%v", err)
	}
}
