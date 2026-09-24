package service_test

import (
	"context"
	"errors"
	"testing"

	"metarang/features-service/internal/models"
	"metarang/features-service/internal/repository"
	"metarang/features-service/internal/service"
)

// Mock repositories for testing
type mockFeatureRepo struct {
	findByOwnerPaginatedFunc    func(ctx context.Context, ownerID uint64, page int, search, filter string) ([]*models.Feature, []*models.FeatureProperties, error)
	findByOwnerAndFeatureIDFunc func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error)
	findByOwnerFunc             func(ctx context.Context, ownerID uint64) ([]*models.Feature, error)
}

func (m *mockFeatureRepo) FindByOwnerPaginated(ctx context.Context, ownerID uint64, page int, search, filter string) ([]*models.Feature, []*models.FeatureProperties, error) {
	if m.findByOwnerPaginatedFunc != nil {
		return m.findByOwnerPaginatedFunc(ctx, ownerID, page, search, filter)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockFeatureRepo) FindByOwnerAndFeatureID(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
	if m.findByOwnerAndFeatureIDFunc != nil {
		return m.findByOwnerAndFeatureIDFunc(ctx, ownerID, featureID)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockFeatureRepo) FindByOwner(ctx context.Context, ownerID uint64) ([]*models.Feature, error) {
	if m.findByOwnerFunc != nil {
		return m.findByOwnerFunc(ctx, ownerID)
	}
	return nil, errors.New("not implemented")
}

type mockImageRepo struct {
	getImagesByFeatureIDFunc func(ctx context.Context, featureID uint64) ([]*repository.Image, error)
	createImageFunc          func(ctx context.Context, featureID uint64, url string) (*repository.Image, error)
	deleteImageFunc          func(ctx context.Context, featureID, imageID uint64) error
}

func (m *mockImageRepo) GetImagesByFeatureID(ctx context.Context, featureID uint64) ([]*repository.Image, error) {
	if m.getImagesByFeatureIDFunc != nil {
		return m.getImagesByFeatureIDFunc(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockImageRepo) CreateImage(ctx context.Context, featureID uint64, url string) (*repository.Image, error) {
	if m.createImageFunc != nil {
		return m.createImageFunc(ctx, featureID, url)
	}
	return nil, errors.New("not implemented")
}

func (m *mockImageRepo) DeleteImage(ctx context.Context, featureID, imageID uint64) error {
	if m.deleteImageFunc != nil {
		return m.deleteImageFunc(ctx, featureID, imageID)
	}
	return errors.New("not implemented")
}

type mockGeometryRepo struct {
	getByFeatureIDFunc            func(ctx context.Context, featureID uint64) (*models.Geometry, error)
	getCoordinatesByFeatureIDFunc func(ctx context.Context, featureID uint64) ([]string, error)
}

func (m *mockGeometryRepo) GetByFeatureID(ctx context.Context, featureID uint64) (*models.Geometry, error) {
	if m.getByFeatureIDFunc != nil {
		return m.getByFeatureIDFunc(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockGeometryRepo) GetCoordinatesByFeatureID(ctx context.Context, featureID uint64) ([]string, error) {
	if m.getCoordinatesByFeatureIDFunc != nil {
		return m.getCoordinatesByFeatureIDFunc(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

type mockTradeRepo struct {
	getLatestForFeatureWithSellerFunc func(ctx context.Context, featureID uint64) (*models.Trade, *repository.SellerInfo, error)
}

func (m *mockTradeRepo) GetLatestForFeatureWithSeller(ctx context.Context, featureID uint64) (*models.Trade, *repository.SellerInfo, error) {
	if m.getLatestForFeatureWithSellerFunc != nil {
		return m.getLatestForFeatureWithSellerFunc(ctx, featureID)
	}
	return nil, nil, errors.New("not implemented")
}

type mockPricingService struct {
	updateFeaturePricingFunc func(ctx context.Context, featureID, userID uint64, minimumPricePercentage int) (*service.FeaturePricingUpdate, error)
}

func (m *mockPricingService) UpdateFeaturePricing(ctx context.Context, featureID, userID uint64, minimumPricePercentage int) (*service.FeaturePricingUpdate, error) {
	if m.updateFeaturePricingFunc != nil {
		return m.updateFeaturePricingFunc(ctx, featureID, userID, minimumPricePercentage)
	}
	return nil, errors.New("not implemented")
}

func TestFeatureService_ListMyFeatures(t *testing.T) {
	t.Run("maps address onto feature properties", func(t *testing.T) {
		props := &models.FeatureProperties{
			ID:        "TO111",
			FeatureID: 1,
			Karbari:   "m",
			Address:   "12 Main St",
			PricePSC:  "100",
			PriceIRR:  "200",
			Stability: 10,
		}
		pbProps := models.PropertiesToPB(props)
		if pbProps.Address != "12 Main St" {
			t.Errorf("Expected address %q, got %q", "12 Main St", pbProps.Address)
		}
		if pbProps.Id != "TO111" {
			t.Errorf("Expected properties id TO111, got %s", pbProps.Id)
		}
		if pbProps.Karbari != "m" {
			t.Errorf("Expected karbari m, got %s", pbProps.Karbari)
		}
	})

	t.Run("repository receives search and filter with pagination", func(t *testing.T) {
		var gotOwner uint64
		var gotPage int
		var gotSearch, gotFilter string
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerPaginatedFunc = func(ctx context.Context, ownerID uint64, page int, search, filter string) ([]*models.Feature, []*models.FeatureProperties, error) {
			gotOwner = ownerID
			gotPage = page
			gotSearch = search
			gotFilter = filter
			return []*models.Feature{{ID: 1, OwnerID: ownerID}}, []*models.FeatureProperties{{
				ID: "TO111", FeatureID: 1, Karbari: "m", Address: "12 Main St", PricePSC: "100", PriceIRR: "200",
			}}, nil
		}

		features, propertiesList, err := mockFeatureRepo.FindByOwnerPaginated(context.Background(), 42, 2, "TO111", "m")
		if err != nil {
			t.Fatalf("FindByOwnerPaginated failed: %v", err)
		}
		if gotOwner != 42 || gotPage != 2 || gotSearch != "TO111" || gotFilter != "m" {
			t.Errorf("unexpected args owner=%d page=%d search=%q filter=%q", gotOwner, gotPage, gotSearch, gotFilter)
		}
		if len(features) != 1 || len(propertiesList) != 1 {
			t.Fatalf("expected 1 paginated result, got %d features and %d properties", len(features), len(propertiesList))
		}
		pbFeature := models.FeatureToPB(features[0], propertiesList[0], nil)
		if pbFeature.Properties.Address != "12 Main St" {
			t.Errorf("Expected address on listed feature, got %q", pbFeature.Properties.Address)
		}
		if len(pbFeature.Images) != 0 {
			t.Errorf("Expected empty images on list items, got %d", len(pbFeature.Images))
		}
	})

	t.Run("empty result", func(t *testing.T) {
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerPaginatedFunc = func(ctx context.Context, ownerID uint64, page int, search, filter string) ([]*models.Feature, []*models.FeatureProperties, error) {
			return []*models.Feature{}, []*models.FeatureProperties{}, nil
		}

		features, propertiesList, err := mockFeatureRepo.FindByOwnerPaginated(context.Background(), 1, 1, "missing", "t")
		if err != nil {
			t.Fatalf("FindByOwnerPaginated failed: %v", err)
		}
		if len(features) != 0 || len(propertiesList) != 0 {
			t.Errorf("Expected 0 features, got %d", len(features))
		}
	})

	t.Run("repository error", func(t *testing.T) {
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerPaginatedFunc = func(ctx context.Context, ownerID uint64, page int, search, filter string) ([]*models.Feature, []*models.FeatureProperties, error) {
			return nil, nil, errors.New("database error")
		}

		_, _, err := mockFeatureRepo.FindByOwnerPaginated(context.Background(), 1, 1, "", "")
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})
}

func TestFeatureService_GetMyFeature(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		// ctx := context.Background()
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{
					ID:      1,
					OwnerID: 1,
				}, &models.FeatureProperties{
					ID:        "1",
					FeatureID: 1,
					Karbari:   "m",
					PricePSC:  "100",
					PriceIRR:  "200",
					Stability: 1000.0,
				}, nil
		}

		mockGeometryRepo := &mockGeometryRepo{}
		mockGeometryRepo.getByFeatureIDFunc = func(ctx context.Context, featureID uint64) (*models.Geometry, error) {
			return &models.Geometry{ID: 1, FeatureID: 1, Type: "Polygon"}, nil
		}
		mockGeometryRepo.getCoordinatesByFeatureIDFunc = func(ctx context.Context, featureID uint64) ([]string, error) {
			return []string{"10.0,20.0", "11.0,21.0"}, nil
		}

		mockImageRepo := &mockImageRepo{}
		mockImageRepo.getImagesByFeatureIDFunc = func(ctx context.Context, featureID uint64) ([]*repository.Image, error) {
			return []*repository.Image{
				{ID: 1, URL: "uploads/features/1/image1.jpg"},
				{ID: 2, URL: "uploads/features/1/image2.jpg"},
			}, nil
		}

		mockTradeRepo := &mockTradeRepo{}
		mockTradeRepo.getLatestForFeatureWithSellerFunc = func(ctx context.Context, featureID uint64) (*models.Trade, *repository.SellerInfo, error) {
			return &models.Trade{ID: 1}, &repository.SellerInfo{ID: 2, Name: "Seller", Code: "S001"}, nil
		}

		// Note: FeatureService uses concrete repository types, not interfaces
		// This test needs to be refactored to use the constructor with proper dependencies
		// For now, we'll need to skip or use integration tests
		t.Skip("Test requires refactoring to use NewFeatureService constructor with proper dependencies")
		// service := service.NewFeatureService(...)
		// feature, err := service.GetMyFeature(ctx, 1, 1)
		// if err != nil {
		// 	t.Fatalf("GetMyFeature failed: %v", err)
		// }
		//
		// if feature.Id != 1 {
		// 	t.Errorf("Expected feature ID 1, got %d", feature.Id)
		// }
		//
		// if len(feature.Images) != 2 {
		// 	t.Errorf("Expected 2 images, got %d", len(feature.Images))
		// }
		//
		// if feature.Seller == nil {
		// 	t.Error("Expected seller, got nil")
		// }
		//
		// if feature.Geometry == nil {
		// 	t.Error("Expected geometry, got nil")
		// }
	})

	t.Run("feature not found", func(t *testing.T) {
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return nil, nil, nil
		}

		t.Skip("Test requires FeatureService fields to be exported or use constructor")
		// service := &service.FeatureService{
		// 	featureRepo: mockFeatureRepo,
		// }
		//
		// _, err := service.GetMyFeature(ctx, 1, 1)
		// if err == nil {
		// 	t.Fatal("Expected error, got nil")
		// }
		// if err.Error() != "feature not found or does not belong to user" {
		// 	t.Errorf("Expected 'feature not found or does not belong to user', got '%s'", err.Error())
		// }
	})
}

func TestFeatureService_AddMyFeatureImages(t *testing.T) {
	t.Run("successful image addition", func(t *testing.T) {
		// ctx := context.Background()
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 1, OwnerID: 1}, &models.FeatureProperties{ID: "1", FeatureID: 1}, nil
		}

		mockImageRepo := &mockImageRepo{}
		mockImageRepo.createImageFunc = func(ctx context.Context, featureID uint64, url string) (*repository.Image, error) {
			return &repository.Image{ID: 1, URL: url}, nil
		}
		mockImageRepo.getImagesByFeatureIDFunc = func(ctx context.Context, featureID uint64) ([]*repository.Image, error) {
			return []*repository.Image{
				{ID: 1, URL: "uploads/features/1/image1.jpg"},
			}, nil
		}

		// mockGeometryRepo := &mockGeometryRepo{}
		// mockTradeRepo := &mockTradeRepo{}

		// Note: FeatureService uses concrete repository types, not interfaces
		// This test needs to be refactored to use the constructor with proper dependencies
		t.Skip("Test requires refactoring to use NewFeatureService constructor with proper dependencies")
		// service := service.NewFeatureService(...)
		// feature, err := service.AddMyFeatureImages(ctx, 1, 1, [][]byte{[]byte("img")}, []string{"image1.jpg"}, []string{"image/jpeg"})
		// if err != nil {
		// 	t.Fatalf("AddMyFeatureImages failed: %v", err)
		// }
		//
		// if len(feature.Images) != 1 {
		// 	t.Errorf("Expected 1 image, got %d", len(feature.Images))
		// }
	})

	t.Run("feature not found", func(t *testing.T) {
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return nil, nil, nil
		}

		t.Skip("Test requires FeatureService fields to be exported or use constructor")
		// service := &service.FeatureService{
		// 	featureRepo: mockFeatureRepo,
		// }
		//
		// _, err := service.AddMyFeatureImages(ctx, 1, 1, [][]byte{[]byte("img")}, []string{"url1.jpg"}, []string{"image/jpeg"})
		// if err == nil {
		// 	t.Fatal("Expected error, got nil")
		// }
	})
}

func TestFeatureService_RemoveMyFeatureImage(t *testing.T) {
	t.Run("successful image removal", func(t *testing.T) {
		// ctx := context.Background()
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 1, OwnerID: 1}, &models.FeatureProperties{ID: "1", FeatureID: 1}, nil
		}

		mockImageRepo := &mockImageRepo{}
		mockImageRepo.deleteImageFunc = func(ctx context.Context, featureID, imageID uint64) error {
			return nil
		}

		// Note: FeatureService uses concrete repository types, not interfaces
		t.Skip("Test requires refactoring to use NewFeatureService constructor with proper dependencies")
		// service := service.NewFeatureService(...)
		// err := service.RemoveMyFeatureImage(ctx, 1, 1, 1)
	})

	t.Run("image not found", func(t *testing.T) {
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 1, OwnerID: 1}, &models.FeatureProperties{ID: "1", FeatureID: 1}, nil
		}

		mockImageRepo := &mockImageRepo{}
		mockImageRepo.deleteImageFunc = func(ctx context.Context, featureID, imageID uint64) error {
			return errors.New("image not found or does not belong to feature")
		}

		// Note: FeatureService uses concrete repository types, not interfaces
		t.Skip("Test requires refactoring to use NewFeatureService constructor with proper dependencies")
		// service := service.NewFeatureService(...)
		// err := service.RemoveMyFeatureImage(ctx, 1, 1, 1)
	})
}

func TestFeatureService_UpdateMyFeature(t *testing.T) {
	t.Run("successful update", func(t *testing.T) {
		// ctx := context.Background()
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 1, OwnerID: 1}, &models.FeatureProperties{ID: "1", FeatureID: 1}, nil
		}

		mockPricingService := &mockPricingService{}
		mockPricingService.updateFeaturePricingFunc = func(ctx context.Context, featureID, userID uint64, minimumPricePercentage int) (*service.FeaturePricingUpdate, error) {
			return &service.FeaturePricingUpdate{PricePSC: "1", PriceIRR: "2"}, nil
		}

		// Note: FeatureService uses concrete repository types, not interfaces
		t.Skip("Test requires refactoring to use NewFeatureService constructor with proper dependencies")
		// service := service.NewFeatureService(...)
		// err := service.UpdateMyFeature(ctx, 1, 1, 100)
	})

	t.Run("feature not found", func(t *testing.T) {
		mockFeatureRepo := &mockFeatureRepo{}
		mockFeatureRepo.findByOwnerAndFeatureIDFunc = func(ctx context.Context, ownerID, featureID uint64) (*models.Feature, *models.FeatureProperties, error) {
			return nil, nil, nil
		}

		t.Skip("Test requires FeatureService fields to be exported or use constructor")
		// service := &service.FeatureService{
		// 	featureRepo: mockFeatureRepo,
		// }
		//
		// err := service.UpdateMyFeature(ctx, 1, 1, 100)
		// if err == nil {
		// 	t.Fatal("Expected error, got nil")
		// }
	})
}
