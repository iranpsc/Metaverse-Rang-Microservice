package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/service"
	commercialpb "metarang/shared/pb/commercial"
	pb "metarang/shared/pb/features"
)

func validBuildRequest() *pb.BuildFeatureRequest {
	return &pb.BuildFeatureRequest{
		FeatureId:            10,
		BuildingModelId:      "1001",
		LaunchedSatisfaction: "20",
		Rotation:             "0",
		Position:             "1,2",
	}
}

func newBuildErrorService() (*service.BuildingService, *mockBuildingRepository, *mockFeatureRepository, *mockHourlyProfitRepository, *mockCommercialClient) {
	buildingRepo := &mockBuildingRepository{
		hasBuildingFunc: func(context.Context, uint64) (bool, error) { return false, nil },
		findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
			return &pb.BuildingModel{Attributes: "[]"}, nil
		},
		findByFeatureIDFunc: func(context.Context, uint64) ([]*pb.Building, error) {
			return nil, nil
		},
		createBuildingFunc: func(context.Context, uint64, uint64, string, string, string, string, string, time.Time, time.Time, float64) error {
			return nil
		},
	}
	featureRepo := &mockFeatureRepository{
		findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 10, OwnerID: 42}, &models.FeatureProperties{Area: 100, Density: 1, Karbari: "m"}, nil
		},
	}
	profitRepo := &mockHourlyProfitRepository{
		deactivateFunc: func(context.Context, uint64) error { return nil },
		activateFunc:   func(context.Context, uint64) error { return nil },
	}
	commercial := &mockCommercialClient{
		getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
			return &commercialpb.WalletResponse{Satisfaction: "100"}, nil
		},
		deductBalanceFunc: func(context.Context, uint64, string, float64) error { return nil },
		addBalanceFunc:    func(context.Context, uint64, string, float64) error { return nil },
	}
	svc := service.NewBuildingService(buildingRepo, featureRepo, &mockGeometryRepository{}, profitRepo, nil)
	svc.SetCommercialClient(commercial)
	return svc, buildingRepo, featureRepo, profitRepo, commercial
}

func TestBuildingService_BuildFeature_ErrorsBeforeCharging(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*service.BuildingService, *mockBuildingRepository, *mockFeatureRepository)
		ctx   context.Context
	}{
		{
			name: "feature lookup failure",
			setup: func(_ *service.BuildingService, _ *mockBuildingRepository, featureRepo *mockFeatureRepository) {
				featureRepo.findByIDFunc = func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
					return nil, nil, errors.New("database unavailable")
				}
			},
			ctx: authContext(42),
		},
		{
			name:  "missing authentication context",
			setup: func(*service.BuildingService, *mockBuildingRepository, *mockFeatureRepository) {},
			ctx:   context.Background(),
		},
		{
			name: "feature ownership failure",
			setup: func(_ *service.BuildingService, _ *mockBuildingRepository, featureRepo *mockFeatureRepository) {
				featureRepo.findByIDFunc = func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
					return &models.Feature{ID: 10, OwnerID: 99}, &models.FeatureProperties{Area: 100, Density: 1, Karbari: "m"}, nil
				}
			},
			ctx: authContext(42),
		},
		{
			name: "building existence query failure",
			setup: func(_ *service.BuildingService, buildingRepo *mockBuildingRepository, _ *mockFeatureRepository) {
				buildingRepo.hasBuildingFunc = func(context.Context, uint64) (bool, error) { return false, errors.New("database down") }
			},
			ctx: authContext(42),
		},
		{
			name: "feature already has building",
			setup: func(_ *service.BuildingService, buildingRepo *mockBuildingRepository, _ *mockFeatureRepository) {
				buildingRepo.hasBuildingFunc = func(context.Context, uint64) (bool, error) { return true, nil }
			},
			ctx: authContext(42),
		},
		{
			name: "model lookup failure",
			setup: func(_ *service.BuildingService, buildingRepo *mockBuildingRepository, _ *mockFeatureRepository) {
				buildingRepo.findModelFunc = func(context.Context, string) (*pb.BuildingModel, error) {
					return nil, errors.New("catalog unavailable")
				}
			},
			ctx: authContext(42),
		},
		{
			name: "model not found",
			setup: func(_ *service.BuildingService, buildingRepo *mockBuildingRepository, _ *mockFeatureRepository) {
				buildingRepo.findModelFunc = func(context.Context, string) (*pb.BuildingModel, error) { return nil, nil }
			},
			ctx: authContext(42),
		},
		{
			name: "feature properties missing",
			setup: func(_ *service.BuildingService, _ *mockBuildingRepository, featureRepo *mockFeatureRepository) {
				featureRepo.findByIDFunc = func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
					return &models.Feature{ID: 10, OwnerID: 42}, nil, nil
				}
			},
			ctx: authContext(42),
		},
		{
			name:  "invalid launched satisfaction",
			setup: func(*service.BuildingService, *mockBuildingRepository, *mockFeatureRepository) {},
			ctx:   authContext(42),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, buildingRepo, featureRepo, _, commercial := newBuildErrorService()
			tc.setup(svc, buildingRepo, featureRepo)
			req := validBuildRequest()
			if tc.name == "invalid launched satisfaction" {
				req.LaunchedSatisfaction = "not-a-number"
			}
			if tc.name == "missing authentication context" {
				// Keep the otherwise valid request and deliberately omit auth context.
			}

			deducted := false
			commercial.deductBalanceFunc = func(context.Context, uint64, string, float64) error {
				deducted = true
				return nil
			}
			_, err := svc.BuildFeature(tc.ctx, req)
			if err == nil {
				t.Fatal("expected build error")
			}
			if deducted {
				t.Fatal("wallet must not be charged before pre-charge validation completes")
			}
		})
	}
}

func TestBuildingService_BuildFeature_WalletAndPersistenceErrors(t *testing.T) {
	t.Run("nil commercial client", func(t *testing.T) {
		svc, _, _, _, _ := newBuildErrorService()
		svc = service.NewBuildingService(
			&mockBuildingRepository{
				hasBuildingFunc: func(context.Context, uint64) (bool, error) { return false, nil },
				findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
					return &pb.BuildingModel{Attributes: "[]"}, nil
				},
			},
			&mockFeatureRepository{findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
				return &models.Feature{ID: 10, OwnerID: 42}, &models.FeatureProperties{Area: 100, Density: 1, Karbari: "m"}, nil
			}},
			&mockGeometryRepository{},
			&mockHourlyProfitRepository{},
			nil,
		)
		_, err := svc.BuildFeature(authContext(42), validBuildRequest())
		if err == nil || !contains(err.Error(), "commercial client") {
			t.Fatalf("err=%v, want commercial client error", err)
		}
	})

	tests := []struct {
		name  string
		setup func(*mockCommercialClient, *mockHourlyProfitRepository, *mockBuildingRepository)
	}{
		{
			name: "wallet lookup failure",
			setup: func(commercial *mockCommercialClient, _ *mockHourlyProfitRepository, _ *mockBuildingRepository) {
				commercial.getWalletFunc = func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
					return nil, errors.New("commercial unavailable")
				}
			},
		},
		{
			name: "malformed wallet satisfaction",
			setup: func(commercial *mockCommercialClient, _ *mockHourlyProfitRepository, _ *mockBuildingRepository) {
				commercial.getWalletFunc = func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
					return &commercialpb.WalletResponse{Satisfaction: "NaN-value"}, nil
				}
			},
		},
		{
			name: "wallet deduction failure",
			setup: func(commercial *mockCommercialClient, _ *mockHourlyProfitRepository, _ *mockBuildingRepository) {
				commercial.deductBalanceFunc = func(context.Context, uint64, string, float64) error { return errors.New("service_unavailable") }
			},
		},
		{
			name: "insufficient satisfaction",
			setup: func(commercial *mockCommercialClient, _ *mockHourlyProfitRepository, _ *mockBuildingRepository) {
				commercial.getWalletFunc = func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
					return &commercialpb.WalletResponse{Satisfaction: "0"}, nil
				}
			},
		},
		{
			name: "ISIC persistence failure",
			setup: func(_ *mockCommercialClient, _ *mockHourlyProfitRepository, buildingRepo *mockBuildingRepository) {
				buildingRepo.firstOrCreateIsicCodeFunc = func(context.Context, string) (uint64, error) { return 0, errors.New("ISIC insert failed") }
			},
		},
		{
			name: "profit deactivation failure",
			setup: func(_ *mockCommercialClient, profit *mockHourlyProfitRepository, _ *mockBuildingRepository) {
				profit.deactivateFunc = func(context.Context, uint64) error { return errors.New("database unavailable") }
			},
		},
		{
			name: "building insert failure",
			setup: func(_ *mockCommercialClient, _ *mockHourlyProfitRepository, buildingRepo *mockBuildingRepository) {
				buildingRepo.createBuildingFunc = func(context.Context, uint64, uint64, string, string, string, string, string, time.Time, time.Time, float64) error {
					return errors.New("insert failed")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, buildingRepo, _, profitRepo, commercial := newBuildErrorService()
			tc.setup(commercial, profitRepo, buildingRepo)
			req := validBuildRequest()
			if tc.name == "ISIC persistence failure" {
				req.Information = &pb.BuildingInformation{ActivityLine: "shop"}
			}
			_, err := svc.BuildFeature(authContext(42), req)
			if err == nil {
				t.Fatal("expected build error")
			}
		})
	}
}

func TestBuildingService_BuildFeature_RollbackCompensationErrorsAreReturned(t *testing.T) {
	t.Run("refund failure after profit deactivation failure", func(t *testing.T) {
		svc, _, _, profitRepo, commercial := newBuildErrorService()
		profitRepo.deactivateFunc = func(context.Context, uint64) error { return errors.New("deactivate failed") }
		commercial.addBalanceFunc = func(context.Context, uint64, string, float64) error { return errors.New("refund failed") }

		_, err := svc.BuildFeature(authContext(42), validBuildRequest())
		if err == nil || !contains(err.Error(), "refund failed") {
			t.Fatalf("err=%v, want refund failure", err)
		}
	})

	t.Run("reactivation and refund failures after insert failure", func(t *testing.T) {
		svc, buildingRepo, _, profitRepo, commercial := newBuildErrorService()
		buildingRepo.createBuildingFunc = func(context.Context, uint64, uint64, string, string, string, string, string, time.Time, time.Time, float64) error {
			return errors.New("insert failed")
		}
		profitRepo.activateFunc = func(context.Context, uint64) error { return errors.New("reactivate failed") }
		commercial.addBalanceFunc = func(context.Context, uint64, string, float64) error { return errors.New("refund failed") }

		_, err := svc.BuildFeature(authContext(42), validBuildRequest())
		if err == nil || !contains(err.Error(), "reactivate failed") || !contains(err.Error(), "refund failed") {
			t.Fatalf("err=%v, want both compensation failures", err)
		}
	})
}

func TestBuildingService_BuildFeature_PostInsertReadFailureDoesNotFailBuild(t *testing.T) {
	svc, buildingRepo, _, _, _ := newBuildErrorService()
	buildingRepo.findByFeatureIDFunc = func(context.Context, uint64) ([]*pb.Building, error) {
		return nil, errors.New("read after insert failed")
	}

	feature, err := svc.BuildFeature(authContext(42), validBuildRequest())
	if err != nil {
		t.Fatalf("BuildFeature() error = %v", err)
	}
	if feature == nil || feature.Id != 10 {
		t.Fatalf("unexpected feature response: %+v", feature)
	}
}
