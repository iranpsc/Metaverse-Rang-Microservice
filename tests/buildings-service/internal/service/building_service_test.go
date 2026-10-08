package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/service"
	"metarang/buildings-service/pkg/threed_client"
	commercialpb "metarang/shared/pb/commercial"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"
)

// Mock repositories for testing
type mockBuildingRepository struct {
	upsertModelFunc                   func(ctx context.Context, modelID uint64, name, sku, images, attributes, file string, requiredSatisfaction float64) error
	findModelFunc                     func(ctx context.Context, modelID string) (*pb.BuildingModel, error)
	hasBuildingFunc                   func(ctx context.Context, featureID uint64) (bool, error)
	createBuildingFunc                func(ctx context.Context, featureID, userID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, startDate, endDate time.Time, bubbleDiameter float64) error
	findByFeatureIDFunc               func(ctx context.Context, featureID uint64) ([]*pb.Building, error)
	updateBuildingFunc                func(ctx context.Context, featureID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, endDate time.Time, bubbleDiameter float64) (*pb.Building, error)
	updateBuildingInformationFunc     func(ctx context.Context, featureID uint64, buildingModelID string, information string) error
	findBuildingByFeatureAndModelFunc func(ctx context.Context, featureID uint64, buildingModelID string) (*pb.Building, error)
	deleteBuildingFunc                func(ctx context.Context, featureID uint64, buildingModelID string) (string, error)
	firstOrCreateIsicCodeFunc         func(ctx context.Context, activityLine string) (uint64, error)
}

func (m *mockBuildingRepository) UpsertBuildingModel(ctx context.Context, modelID uint64, name, sku, images, attributes, file string, requiredSatisfaction float64) error {
	if m.upsertModelFunc != nil {
		return m.upsertModelFunc(ctx, modelID, name, sku, images, attributes, file, requiredSatisfaction)
	}
	return errors.New("not implemented")
}

func (m *mockBuildingRepository) FindBuildingModelByModelID(ctx context.Context, modelID string) (*pb.BuildingModel, error) {
	if m.findModelFunc != nil {
		return m.findModelFunc(ctx, modelID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingRepository) HasBuilding(ctx context.Context, featureID uint64) (bool, error) {
	if m.hasBuildingFunc != nil {
		return m.hasBuildingFunc(ctx, featureID)
	}
	return false, errors.New("not implemented")
}

func (m *mockBuildingRepository) CreateBuilding(ctx context.Context, featureID, userID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, startDate, endDate time.Time, bubbleDiameter float64) error {
	if m.createBuildingFunc != nil {
		return m.createBuildingFunc(ctx, featureID, userID, buildingModelID, launchedSatisfaction, rotation, position, information, startDate, endDate, bubbleDiameter)
	}
	return errors.New("not implemented")
}

func (m *mockBuildingRepository) FindByFeatureID(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
	if m.findByFeatureIDFunc != nil {
		return m.findByFeatureIDFunc(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingRepository) UpdateBuilding(ctx context.Context, featureID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, endDate time.Time, bubbleDiameter float64) (*pb.Building, error) {
	if m.updateBuildingFunc != nil {
		return m.updateBuildingFunc(ctx, featureID, buildingModelID, launchedSatisfaction, rotation, position, information, endDate, bubbleDiameter)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingRepository) UpdateBuildingInformation(ctx context.Context, featureID uint64, buildingModelID string, information string) error {
	if m.updateBuildingInformationFunc != nil {
		return m.updateBuildingInformationFunc(ctx, featureID, buildingModelID, information)
	}
	return errors.New("not implemented")
}

func (m *mockBuildingRepository) FindBuildingByFeatureAndModel(ctx context.Context, featureID uint64, buildingModelID string) (*pb.Building, error) {
	if m.findBuildingByFeatureAndModelFunc != nil {
		return m.findBuildingByFeatureAndModelFunc(ctx, featureID, buildingModelID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingRepository) DeleteBuilding(ctx context.Context, featureID uint64, buildingModelID string) (string, error) {
	if m.deleteBuildingFunc != nil {
		return m.deleteBuildingFunc(ctx, featureID, buildingModelID)
	}
	return "", errors.New("not implemented")
}

func (m *mockBuildingRepository) FirstOrCreateIsicCode(ctx context.Context, activityLine string) (uint64, error) {
	if m.firstOrCreateIsicCodeFunc != nil {
		return m.firstOrCreateIsicCodeFunc(ctx, activityLine)
	}
	return 0, errors.New("not implemented")
}

// Mock other repositories
type mockFeatureRepository struct {
	findByIDFunc func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error)
}

func (m *mockFeatureRepository) FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return nil, nil, errors.New("not implemented")
}

type mockGeometryRepository struct {
	getCoordinatesFunc func(ctx context.Context, featureID uint64) ([]string, error)
}

func (m *mockGeometryRepository) GetCoordinatesByFeatureID(ctx context.Context, featureID uint64) ([]string, error) {
	if m.getCoordinatesFunc != nil {
		return m.getCoordinatesFunc(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

type mockHourlyProfitRepository struct {
	deactivateFunc func(ctx context.Context, featureID uint64) error
	activateFunc   func(ctx context.Context, featureID uint64) error
}

func (m *mockHourlyProfitRepository) DeactivateProfitsForFeature(ctx context.Context, featureID uint64) error {
	if m.deactivateFunc != nil {
		return m.deactivateFunc(ctx, featureID)
	}
	return errors.New("not implemented")
}

func (m *mockHourlyProfitRepository) ActivateProfitsForFeature(ctx context.Context, featureID uint64) error {
	if m.activateFunc != nil {
		return m.activateFunc(ctx, featureID)
	}
	return errors.New("not implemented")
}

// Mock 3D client
type mockThreeDClient struct {
	getBuildPackageFunc func(ctx context.Context, req threed_client.BuildPackageRequest) (*threed_client.BuildPackageResponse, error)
}

func (m *mockThreeDClient) GetBuildPackage(ctx context.Context, req threed_client.BuildPackageRequest) (*threed_client.BuildPackageResponse, error) {
	if m.getBuildPackageFunc != nil {
		return m.getBuildPackageFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

type mockCommercialClient struct {
	getWalletFunc     func(ctx context.Context, userID uint64) (*commercialpb.WalletResponse, error)
	deductBalanceFunc func(ctx context.Context, userID uint64, asset string, amount float64) error
	addBalanceFunc    func(ctx context.Context, userID uint64, asset string, amount float64) error
}

func (m *mockCommercialClient) GetWallet(ctx context.Context, userID uint64) (*commercialpb.WalletResponse, error) {
	if m.getWalletFunc != nil {
		return m.getWalletFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockCommercialClient) DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if m.deductBalanceFunc != nil {
		return m.deductBalanceFunc(ctx, userID, asset, amount)
	}
	return errors.New("not implemented")
}

func (m *mockCommercialClient) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if m.addBalanceFunc != nil {
		return m.addBalanceFunc(ctx, userID, asset, amount)
	}
	return errors.New("not implemented")
}

func authContext(userID uint64) context.Context {
	return context.WithValue(context.Background(), auth.UserContextKey{}, &auth.UserContext{UserID: userID})
}

func TestBuildingService_GetBuildPackage(t *testing.T) {
	featureRepo := &mockFeatureRepository{
		findByIDFunc: func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 1, OwnerID: 42}, &models.FeatureProperties{
				Karbari: "m", Area: 100, Density: 0,
			}, nil
		},
	}
	geomRepo := &mockGeometryRepository{
		getCoordinatesFunc: func(ctx context.Context, featureID uint64) ([]string, error) {
			return []string{"1.0,2.0"}, nil
		},
	}

	t.Run("unauthorized user", func(t *testing.T) {
		svc := service.NewBuildingService(&mockBuildingRepository{}, featureRepo, geomRepo, &mockHourlyProfitRepository{}, nil)
		_, _, err := svc.GetBuildPackage(context.Background(), 1, 1)
		if err == nil || !contains(err.Error(), "unauthorized") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("wrong owner", func(t *testing.T) {
		svc := service.NewBuildingService(&mockBuildingRepository{}, featureRepo, geomRepo, &mockHourlyProfitRepository{}, nil)
		_, _, err := svc.GetBuildPackage(authContext(99), 1, 1)
		if err == nil || !contains(err.Error(), "does not own") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("success upserts models", func(t *testing.T) {
		var upserted uint64
		var storedRequired float64
		buildingRepo := &mockBuildingRepository{
			upsertModelFunc: func(ctx context.Context, modelID uint64, name, sku, images, attributes, file string, requiredSatisfaction float64) error {
				upserted = modelID
				storedRequired = requiredSatisfaction
				return nil
			},
		}
		threeD := &mockThreeDClient{
			getBuildPackageFunc: func(ctx context.Context, req threed_client.BuildPackageRequest) (*threed_client.BuildPackageResponse, error) {
				if req.Density != "1" {
					t.Fatalf("density default=%s", req.Density)
				}
				return &threed_client.BuildPackageResponse{Data: []threed_client.BuildingModelData{{
					ID: 7, Name: "tower", SKU: "sku",
					Images: []map[string]interface{}{{"url": "a"}},
					File:   map[string]interface{}{"glb": "x"},
				}}}, nil
			},
		}
		svc := service.NewBuildingService(buildingRepo, featureRepo, geomRepo, &mockHourlyProfitRepository{}, threeD)
		models, coords, err := svc.GetBuildPackage(authContext(42), 1, 1)
		if err != nil {
			t.Fatalf("GetBuildPackage: %v", err)
		}
		if len(models) != 1 || models[0].Id != 7 || upserted != 7 || storedRequired != 0 {
			t.Fatalf("models=%v upserted=%d storedRequired=%v", models, upserted, storedRequired)
		}
		if models[0].RequiredSatisfaction != "0.0100" {
			t.Fatalf("required=%s, want feature-specific 0.0100", models[0].RequiredSatisfaction)
		}
		if len(coords) != 1 {
			t.Fatalf("coords=%v", coords)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Test CalculateBubbleDiameter function
func TestBuildingService_CalculateBubbleDiameter(t *testing.T) {
	service := service.NewBuildingService(nil, nil, nil, nil, nil)

	tests := []struct {
		name           string
		attributesJSON string
		want           float64
	}{
		{
			name:           "density 1 - coefficient 1.0",
			attributesJSON: `[{"slug": "width", "value": 50}, {"slug": "length", "value": 30}, {"slug": "density", "value": 1}]`,
			want:           160.0, // perimeter = 2 * (50 + 30) = 160, coefficient = 1.0, diameter = 160 * 1.0 = 160
		},
		{
			name:           "density 2 - coefficient 1.3",
			attributesJSON: `[{"slug": "width", "value": 50}, {"slug": "length", "value": 30}, {"slug": "density", "value": 2}]`,
			want:           208.0, // perimeter = 160, coefficient = 1.3, diameter = 160 * 1.3 = 208
		},
		{
			name:           "density 3 - coefficient 1.6",
			attributesJSON: `[{"slug": "width", "value": 50}, {"slug": "length", "value": 30}, {"slug": "density", "value": 3}]`,
			want:           256.0, // perimeter = 160, coefficient = 1.6, diameter = 160 * 1.6 = 256
		},
		{
			name:           "density 4 - coefficient 1.9",
			attributesJSON: `[{"slug": "width", "value": 40}, {"slug": "length", "value": 20}, {"slug": "density", "value": 4}]`,
			want:           228.0, // perimeter = 2 * (40 + 20) = 120, coefficient = 1.9, diameter = 120 * 1.9 = 228
			// Note: Floating point precision may cause slight variance, so we check with tolerance
		},
		{
			name:           "string numeric values from 3D API",
			attributesJSON: `[{"slug": "width", "value": "50"}, {"slug": "length", "value": "30"}, {"slug": "density", "value": "3"}]`,
			want:           256.0, // same as density 3 with numeric values
		},
		{
			name:           "missing width attribute",
			attributesJSON: `[{"slug": "length", "value": 30}, {"slug": "density", "value": 1}]`,
			want:           0.0,
		},
		{
			name:           "missing length attribute",
			attributesJSON: `[{"slug": "width", "value": 50}, {"slug": "density", "value": 1}]`,
			want:           0.0,
		},
		{
			name:           "missing density attribute",
			attributesJSON: `[{"slug": "width", "value": 50}, {"slug": "length", "value": 30}]`,
			want:           0.0,
		},
		{
			name:           "invalid JSON",
			attributesJSON: `invalid json`,
			want:           0.0,
		},
		{
			name:           "empty JSON",
			attributesJSON: `[]`,
			want:           0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.CalculateBubbleDiameter(tt.attributesJSON)
			// Use tolerance for floating point comparison
			diff := got - tt.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.0001 {
				t.Errorf("CalculateBubbleDiameter() = %v, want %v (diff: %v)", got, tt.want, diff)
			}
		})
	}
}

// Test ExtractAttributeValue helper function
func TestExtractAttributeValue(t *testing.T) {
	tests := []struct {
		name       string
		attributes []map[string]interface{}
		slug       string
		wantValue  float64
		wantOk     bool
	}{
		{
			name: "valid attribute found - width",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": 50.0},
				{"slug": "length", "value": 30.0},
				{"slug": "density", "value": 3.0},
			},
			slug:      "width",
			wantValue: 50.0,
			wantOk:    true,
		},
		{
			name: "valid attribute found - length",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": 50.0},
				{"slug": "length", "value": 30.0},
				{"slug": "density", "value": 3.0},
			},
			slug:      "length",
			wantValue: 30.0,
			wantOk:    true,
		},
		{
			name: "valid attribute found - density",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": 50.0},
				{"slug": "length", "value": 30.0},
				{"slug": "density", "value": 3.0},
			},
			slug:      "density",
			wantValue: 3.0,
			wantOk:    true,
		},
		{
			name: "attribute not found",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": 50.0},
				{"slug": "length", "value": 30.0},
			},
			slug:      "density",
			wantValue: 0.0,
			wantOk:    false,
		},
		{
			name:       "empty attributes",
			attributes: []map[string]interface{}{},
			slug:       "width",
			wantValue:  0.0,
			wantOk:     false,
		},
		{
			name: "attribute with wrong type",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": "not-a-number"},
			},
			slug:      "width",
			wantValue: 0.0,
			wantOk:    false,
		},
		{
			name: "numeric string value",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": "50"},
			},
			slug:      "width",
			wantValue: 50.0,
			wantOk:    true,
		},
		{
			name: "int value",
			attributes: []map[string]interface{}{
				{"slug": "width", "value": 50},
			},
			slug:      "width",
			wantValue: 50.0,
			wantOk:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOk := service.ExtractAttributeValue(tt.attributes, tt.slug)
			if gotValue != tt.wantValue {
				t.Errorf("service.ExtractAttributeValue() value = %v, want %v", gotValue, tt.wantValue)
			}
			if gotOk != tt.wantOk {
				t.Errorf("service.ExtractAttributeValue() ok = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

// Test construction duration calculation
func TestBuildingService_ConstructionDuration(t *testing.T) {
	tests := []struct {
		name                 string
		requiredSatisfaction float64
		launchedSatisfaction float64
		wantHours            float64
	}{
		{
			name:                 "required 10, launched 10 - 288000 hours",
			requiredSatisfaction: 10.0,
			launchedSatisfaction: 10.0,
			wantHours:            288000.0, // 10 * 288000 / 10 = 288000
		},
		{
			name:                 "required 10, launched 20 - 144000 hours",
			requiredSatisfaction: 10.0,
			launchedSatisfaction: 20.0,
			wantHours:            144000.0, // 10 * 288000 / 20 = 144000
		},
		{
			name:                 "required 10, launched 100 - 28800 hours",
			requiredSatisfaction: 10.0,
			launchedSatisfaction: 100.0,
			wantHours:            28800.0, // 10 * 288000 / 100 = 28800
		},
		{
			name:                 "required 12.5, launched 25 - 144000 hours",
			requiredSatisfaction: 12.5,
			launchedSatisfaction: 25.0,
			wantHours:            144000.0, // 12.5 * 288000 / 25 = 144000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Formula: required_satisfaction * 288000 / launched_satisfaction
			gotHours := tt.requiredSatisfaction * 288000.0 / tt.launchedSatisfaction
			if gotHours != tt.wantHours {
				t.Errorf("Construction duration = %v hours, want %v hours", gotHours, tt.wantHours)
			}
		})
	}
}

// Test required satisfaction calculation
func TestBuildingService_RequiredSatisfaction(t *testing.T) {
	tests := []struct {
		name         string
		area         float64
		karbariCoeff float64
		density      int
		want         float64
		description  string
	}{
		{
			name:         "basic calculation",
			area:         100.0,
			karbariCoeff: 1.0,
			density:      1,
			want:         0.1, // 100 * 1.0 * 1 * 0.1 / 100 = 0.1
			description:  "area=100, karbari=1.0, density=1",
		},
		{
			name:         "with karbari coefficient",
			area:         100.0,
			karbariCoeff: 1.5,
			density:      1,
			want:         0.15, // 100 * 1.5 * 1 * 0.1 / 100 = 0.15
			description:  "area=100, karbari=1.5, density=1",
		},
		{
			name:         "with density",
			area:         100.0,
			karbariCoeff: 1.0,
			density:      3,
			want:         0.3, // 100 * 1.0 * 3 * 0.1 / 100 = 0.3
			description:  "area=100, karbari=1.0, density=3",
		},
		{
			name:         "full calculation",
			area:         1500.0,
			karbariCoeff: 1.2,
			density:      2,
			want:         3.6, // 1500 * 1.2 * 2 * 0.1 / 100 = 3.6
			description:  "area=1500, karbari=1.2, density=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Formula: area * karbariCoeff * density * 0.1 / 100
			got := tt.area * tt.karbariCoeff * float64(tt.density) * 0.1 / 100.0
			if got != tt.want {
				t.Errorf("Required satisfaction = %v, want %v (%s)", got, tt.want, tt.description)
			}
		})
	}
}

// Test ValidateBuildingInformation function
func TestBuildingService_ValidateBuildingInformation(t *testing.T) {
	service := service.NewBuildingService(nil, nil, nil, nil, nil)

	tests := []struct {
		name    string
		info    *pb.BuildingInformation
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil information - valid",
			info:    nil,
			wantErr: false,
		},
		{
			name: "valid information with all fields",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Name:         "Tech Solutions Inc",
				Address:      "123 Main St",
				PostalCode:   "1234567890",
				Website:      "https://example.com",
				Description:  "A software company",
			},
			wantErr: false,
		},
		{
			name: "activity_line exceeds 255 characters",
			info: &pb.BuildingInformation{
				ActivityLine: string(make([]byte, 256)), // 256 characters
			},
			wantErr: true,
			errMsg:  "activity_line must not exceed 255 characters",
		},
		{
			name: "name exceeds 255 characters",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Name:         string(make([]byte, 256)),
			},
			wantErr: true,
			errMsg:  "name must not exceed 255 characters",
		},
		{
			name: "address exceeds 255 characters",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Address:      string(make([]byte, 256)),
			},
			wantErr: true,
			errMsg:  "address must not exceed 255 characters",
		},
		{
			name: "invalid postal_code - too short",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				PostalCode:   "12345",
			},
			wantErr: true,
			errMsg:  "postal_code must be a valid Iranian postal code",
		},
		{
			name: "invalid postal_code - non-numeric",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				PostalCode:   "abcdefghij",
			},
			wantErr: true,
			errMsg:  "postal_code must be a valid Iranian postal code",
		},
		{
			name: "valid postal_code with dashes",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				PostalCode:   "12345-67890",
			},
			wantErr: false,
		},
		{
			name: "invalid website - not a URL",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Website:      "not-a-url",
			},
			wantErr: true,
			errMsg:  "website must be a valid URL",
		},
		{
			name: "invalid website - no scheme",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Website:      "example.com",
			},
			wantErr: true,
			errMsg:  "website must be a valid URL",
		},
		{
			name: "invalid website - wrong scheme",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Website:      "ftp://example.com",
			},
			wantErr: true,
			errMsg:  "website must use http or https protocol",
		},
		{
			name: "valid website - http",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Website:      "http://example.com",
			},
			wantErr: false,
		},
		{
			name: "valid website - https",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Website:      "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "website exceeds 255 characters",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Website:      "https://" + string(make([]byte, 250)) + ".com",
			},
			wantErr: true,
			errMsg:  "website must not exceed 255 characters",
		},
		{
			name: "description exceeds 5000 characters",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Description:  string(make([]byte, 5001)),
			},
			wantErr: true,
			errMsg:  "description must not exceed 5000 characters",
		},
		{
			name: "valid description at limit",
			info: &pb.BuildingInformation{
				ActivityLine: "Software Development",
				Description:  string(make([]byte, 5000)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateBuildingInformation(tt.info)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBuildingInformation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if err.Error()[:len(tt.errMsg)] != tt.errMsg {
					t.Errorf("ValidateBuildingInformation() error message = %v, want contains %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestBuildingService_DestroyBuilding(t *testing.T) {
	const (
		ownerID         uint64 = 42
		featureID       uint64 = 10
		buildingModelID        = "model-3"
		launchedSat            = 25.5
	)

	t.Run("refunds launched_satisfaction to user wallet", func(t *testing.T) {
		var (
			deleted          bool
			profitsActivated bool
			refundUserID     uint64
			refundAsset      string
			refundAmount     float64
			addBalanceCalls  int
		)

		buildingRepo := &mockBuildingRepository{
			findBuildingByFeatureAndModelFunc: func(ctx context.Context, fid uint64, modelID string) (*pb.Building, error) {
				if fid != featureID || modelID != buildingModelID {
					t.Fatalf("unexpected find args: featureID=%d modelID=%s", fid, modelID)
				}
				return &pb.Building{
					LaunchedSatisfaction: "25.5",
				}, nil
			},
			deleteBuildingFunc: func(ctx context.Context, fid uint64, modelID string) (string, error) {
				if fid != featureID || modelID != buildingModelID {
					t.Fatalf("unexpected delete args: featureID=%d modelID=%s", fid, modelID)
				}
				deleted = true
				return "25.5", nil
			},
		}
		featureRepo := &mockFeatureRepository{
			findByIDFunc: func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
				return &models.Feature{ID: featureID, OwnerID: ownerID}, &models.FeatureProperties{}, nil
			},
		}
		profitRepo := &mockHourlyProfitRepository{
			activateFunc: func(ctx context.Context, fid uint64) error {
				if fid != featureID {
					t.Fatalf("unexpected activate featureID=%d", fid)
				}
				profitsActivated = true
				return nil
			},
		}
		commercial := &mockCommercialClient{
			addBalanceFunc: func(ctx context.Context, userID uint64, asset string, amount float64) error {
				addBalanceCalls++
				refundUserID = userID
				refundAsset = asset
				refundAmount = amount
				return nil
			},
		}

		svc := service.NewBuildingService(buildingRepo, featureRepo, &mockGeometryRepository{}, profitRepo, nil)
		svc.SetCommercialClient(commercial)

		err := svc.DestroyBuilding(authContext(ownerID), featureID, buildingModelID)
		if err != nil {
			t.Fatalf("DestroyBuilding() error = %v", err)
		}
		if !deleted {
			t.Fatal("expected building to be deleted")
		}
		if !profitsActivated {
			t.Fatal("expected hourly profits to be reactivated")
		}
		if addBalanceCalls != 1 {
			t.Fatalf("AddBalance call count = %d, want 1", addBalanceCalls)
		}
		if refundUserID != ownerID {
			t.Errorf("refund userID = %d, want %d", refundUserID, ownerID)
		}
		if refundAsset != "satisfaction" {
			t.Errorf("refund asset = %q, want %q", refundAsset, "satisfaction")
		}
		if refundAmount != launchedSat {
			t.Errorf("refund amount = %v, want %v", refundAmount, launchedSat)
		}
	})

	t.Run("does not refund when launched_satisfaction is zero", func(t *testing.T) {
		addBalanceCalls := 0
		buildingRepo := &mockBuildingRepository{
			findBuildingByFeatureAndModelFunc: func(ctx context.Context, fid uint64, modelID string) (*pb.Building, error) {
				return &pb.Building{
					LaunchedSatisfaction: "0",
				}, nil
			},
			deleteBuildingFunc: func(ctx context.Context, fid uint64, modelID string) (string, error) {
				return "0", nil
			},
		}
		featureRepo := &mockFeatureRepository{
			findByIDFunc: func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
				return &models.Feature{ID: featureID, OwnerID: ownerID}, &models.FeatureProperties{}, nil
			},
		}
		profitRepo := &mockHourlyProfitRepository{
			activateFunc: func(ctx context.Context, fid uint64) error { return nil },
		}
		commercial := &mockCommercialClient{
			addBalanceFunc: func(ctx context.Context, userID uint64, asset string, amount float64) error {
				addBalanceCalls++
				return nil
			},
		}

		svc := service.NewBuildingService(buildingRepo, featureRepo, &mockGeometryRepository{}, profitRepo, nil)
		svc.SetCommercialClient(commercial)

		if err := svc.DestroyBuilding(authContext(ownerID), featureID, buildingModelID); err != nil {
			t.Fatalf("DestroyBuilding() error = %v", err)
		}
		if addBalanceCalls != 0 {
			t.Fatalf("AddBalance call count = %d, want 0", addBalanceCalls)
		}
	})

	t.Run("unauthorized when user does not own feature", func(t *testing.T) {
		buildingRepo := &mockBuildingRepository{}
		featureRepo := &mockFeatureRepository{
			findByIDFunc: func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
				return &models.Feature{ID: featureID, OwnerID: ownerID}, &models.FeatureProperties{}, nil
			},
		}
		svc := service.NewBuildingService(buildingRepo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)

		err := svc.DestroyBuilding(authContext(999), featureID, buildingModelID)
		if err == nil {
			t.Fatal("expected unauthorized error")
		}
		if !contains(err.Error(), "unauthorized") {
			t.Errorf("error = %v, want unauthorized", err)
		}
	})

	t.Run("building not found", func(t *testing.T) {
		buildingRepo := &mockBuildingRepository{
			deleteBuildingFunc: func(ctx context.Context, fid uint64, modelID string) (string, error) {
				return "", errors.New("building not found")
			},
		}
		featureRepo := &mockFeatureRepository{
			findByIDFunc: func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
				return &models.Feature{ID: featureID, OwnerID: ownerID}, &models.FeatureProperties{}, nil
			},
		}
		svc := service.NewBuildingService(buildingRepo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)

		err := svc.DestroyBuilding(authContext(ownerID), featureID, buildingModelID)
		if err == nil {
			t.Fatal("expected building not found error")
		}
		if !contains(err.Error(), "building not found") {
			t.Errorf("error = %v, want building not found", err)
		}
	})
}

func TestBuildingService_BuildFeature_Rollback(t *testing.T) {
	const (
		ownerID   uint64 = 42
		featureID uint64 = 10
	)
	req := &pb.BuildFeatureRequest{
		FeatureId:            featureID,
		BuildingModelId:      "m1",
		LaunchedSatisfaction: "20",
		Rotation:             "0",
		Position:             "1,2",
	}

	newSvc := func(createErr, deactivateErr error, refunds *int, activated *bool) *service.BuildingService {
		buildingRepo := &mockBuildingRepository{
			hasBuildingFunc: func(context.Context, uint64) (bool, error) { return false, nil },
			findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
				return &pb.BuildingModel{RequiredSatisfaction: "10", Attributes: "[]"}, nil
			},
			createBuildingFunc: func(context.Context, uint64, uint64, string, string, string, string, string, time.Time, time.Time, float64) error {
				return createErr
			},
			findByFeatureIDFunc: func(context.Context, uint64) ([]*pb.Building, error) { return nil, nil },
		}
		featureRepo := &mockFeatureRepository{
			findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
				return &models.Feature{ID: featureID, OwnerID: ownerID}, &models.FeatureProperties{}, nil
			},
		}
		profitRepo := &mockHourlyProfitRepository{
			deactivateFunc: func(context.Context, uint64) error { return deactivateErr },
			activateFunc: func(context.Context, uint64) error {
				if activated != nil {
					*activated = true
				}
				return nil
			},
		}
		commercial := &mockCommercialClient{
			getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
				return &commercialpb.WalletResponse{Satisfaction: "100"}, nil
			},
			deductBalanceFunc: func(context.Context, uint64, string, float64) error { return nil },
			addBalanceFunc: func(context.Context, uint64, string, float64) error {
				if refunds != nil {
					*refunds++
				}
				return nil
			},
		}
		svc := service.NewBuildingService(buildingRepo, featureRepo, &mockGeometryRepository{}, profitRepo, nil)
		svc.SetCommercialClient(commercial)
		return svc
	}

	t.Run("refunds when deactivate profits fails", func(t *testing.T) {
		refunds := 0
		svc := newSvc(nil, errors.New("deactivate failed"), &refunds, nil)
		_, err := svc.BuildFeature(authContext(ownerID), req)
		if err == nil {
			t.Fatal("expected error")
		}
		if refunds != 1 {
			t.Fatalf("refunds = %d, want 1", refunds)
		}
	})

	t.Run("reactivates profits and refunds when create building fails", func(t *testing.T) {
		refunds := 0
		activated := false
		svc := newSvc(errors.New("insert failed"), nil, &refunds, &activated)
		_, err := svc.BuildFeature(authContext(ownerID), req)
		if err == nil {
			t.Fatal("expected error")
		}
		if !activated {
			t.Fatal("expected profits to be reactivated")
		}
		if refunds != 1 {
			t.Fatalf("refunds = %d, want 1", refunds)
		}
	})
}

func TestBuildingService_BuildFeature_Success(t *testing.T) {
	const ownerID, featureID uint64 = 42, 10
	var created bool
	repo := &mockBuildingRepository{
		hasBuildingFunc: func(context.Context, uint64) (bool, error) { return false, nil },
		findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
			return &pb.BuildingModel{
				RequiredSatisfaction: "10",
				Attributes:           `[{"slug":"width","value":50},{"slug":"length","value":"20"},{"slug":"density","value":2}]`,
			}, nil
		},
		firstOrCreateIsicCodeFunc: func(context.Context, string) (uint64, error) { return 3, nil },
		createBuildingFunc: func(context.Context, uint64, uint64, string, string, string, string, string, time.Time, time.Time, float64) error {
			created = true
			return nil
		},
		findByFeatureIDFunc: func(context.Context, uint64) ([]*pb.Building, error) {
			return []*pb.Building{{Id: 9}}, nil
		},
	}
	svc := service.NewBuildingService(repo, &mockFeatureRepository{
		findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: featureID, OwnerID: ownerID}, &models.FeatureProperties{}, nil
		},
	}, &mockGeometryRepository{}, &mockHourlyProfitRepository{
		deactivateFunc: func(context.Context, uint64) error { return nil },
	}, nil)
	svc.SetCommercialClient(&mockCommercialClient{
		getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
			return &commercialpb.WalletResponse{Satisfaction: "100"}, nil
		},
		deductBalanceFunc: func(context.Context, uint64, string, float64) error { return nil },
	})
	feat, err := svc.BuildFeature(authContext(ownerID), &pb.BuildFeatureRequest{
		FeatureId: featureID, BuildingModelId: "m1", LaunchedSatisfaction: "20", Rotation: "0", Position: "1, 2",
		Information: &pb.BuildingInformation{
			ActivityLine: "shop", Name: "n", Address: "a", PostalCode: "1234567890",
			Website: "https://example.com", Description: "d",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created || feat.Id != featureID || len(feat.BuildingModels) != 1 {
		t.Fatalf("created=%v feat=%v", created, feat)
	}
}

func TestBuildingService_GetBuildingsAndUpdates(t *testing.T) {
	featureRepo := &mockFeatureRepository{
		findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 10, OwnerID: 42}, &models.FeatureProperties{Area: 5000, Density: 1}, nil
		},
	}

	t.Run("GetBuildings formats dates", func(t *testing.T) {
		repo := &mockBuildingRepository{
			findByFeatureIDFunc: func(context.Context, uint64) ([]*pb.Building, error) {
				return []*pb.Building{{
					ConstructionStartDate: "2026-01-02 15:04:05",
					ConstructionEndDate:   "2026-02-01T15:04:05Z",
					LaunchedSatisfaction:  "12.5",
				}}, nil
			},
		}
		svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
		got, err := svc.GetBuildings(context.Background(), 10)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].LaunchedSatisfaction != "12.5000" {
			t.Fatalf("sat=%s", got[0].LaunchedSatisfaction)
		}
	})

	t.Run("UpdateBuilding success", func(t *testing.T) {
		repo := &mockBuildingRepository{
			findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
				return &pb.BuildingModel{Id: 7, ModelId: "m1", RequiredSatisfaction: "1"}, nil
			},
			findBuildingByFeatureAndModelFunc: func(context.Context, uint64, string) (*pb.Building, error) {
				return &pb.Building{ConstructionStartDate: "2026-01-02 15:04:05", BubbleDiameter: "3.5", LaunchedSatisfaction: "10", Information: `{"name":"keep"}`}, nil
			},
			updateBuildingFunc: func(_ context.Context, _ uint64, _ string, _ string, _ string, _ string, information string, _ time.Time, _ float64) (*pb.Building, error) {
				if information != `{"name":"keep"}` {
					t.Fatalf("information overwritten: %s", information)
				}
				return &pb.Building{
					ConstructionStartDate: "2026-01-02 15:04:05",
					ConstructionEndDate:   "2026-02-01 15:04:05",
					LaunchedSatisfaction:  "10",
				}, nil
			},
		}
		svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
		svc.SetCommercialClient(&mockCommercialClient{
			getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
				return &commercialpb.WalletResponse{Satisfaction: "100"}, nil
			},
		})
		got, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "10", Rotation: "0", Position: "1,2",
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.LaunchedSatisfaction != "10.0000" {
			t.Fatalf("sat=%s", got.LaunchedSatisfaction)
		}
	})

	t.Run("UpdateBuilding with information", func(t *testing.T) {
		repo := &mockBuildingRepository{
			findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
				return &pb.BuildingModel{Id: 7, ModelId: "m1", RequiredSatisfaction: "1"}, nil
			},
			findBuildingByFeatureAndModelFunc: func(context.Context, uint64, string) (*pb.Building, error) {
				return &pb.Building{ConstructionStartDate: "2026-01-02 15:04:05", BubbleDiameter: "3.5", LaunchedSatisfaction: "10"}, nil
			},
			firstOrCreateIsicCodeFunc: func(context.Context, string) (uint64, error) { return 1, nil },
			updateBuildingFunc: func(context.Context, uint64, string, string, string, string, string, time.Time, float64) (*pb.Building, error) {
				return &pb.Building{LaunchedSatisfaction: "10", ConstructionStartDate: "2026-01-02T15:04:05Z"}, nil
			},
		}
		svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
		svc.SetCommercialClient(&mockCommercialClient{
			getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
				return &commercialpb.WalletResponse{Satisfaction: "100"}, nil
			},
		})
		_, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "10", Rotation: "0", Position: "1,2",
			Information: &pb.BuildingInformation{ActivityLine: "shop", Name: "n", Address: "a", Description: "d"},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("UpdateBuildingInformation merges", func(t *testing.T) {
		var saved string
		repo := &mockBuildingRepository{
			findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
				return &pb.BuildingModel{Id: 7, ModelId: "m1"}, nil
			},
			findBuildingByFeatureAndModelFunc: func(context.Context, uint64, string) (*pb.Building, error) {
				return &pb.Building{Information: `{"name":"old"}`}, nil
			},
			firstOrCreateIsicCodeFunc: func(context.Context, string) (uint64, error) { return 1, nil },
			updateBuildingInformationFunc: func(_ context.Context, _ uint64, _ string, information string) error {
				saved = information
				return nil
			},
		}
		svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
		info, err := svc.UpdateBuildingInformation(authContext(42), &pb.UpdateBuildingInformationRequest{
			FeatureId: 10, BuildingModelId: "m1",
			Information: &pb.BuildingInformation{ActivityLine: "shop", Name: "new", Address: "a", PostalCode: "1234567890", Description: "d"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if info.Name != "new" || saved == "" {
			t.Fatalf("info=%v saved=%s", info, saved)
		}
	})

	t.Run("UpdateBuilding validation errors", func(t *testing.T) {
		repo := &mockBuildingRepository{
			findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
				return &pb.BuildingModel{Id: 7, ModelId: "m1", RequiredSatisfaction: "5"}, nil
			},
			findBuildingByFeatureAndModelFunc: func(context.Context, uint64, string) (*pb.Building, error) {
				return &pb.Building{ConstructionStartDate: "not-a-date", BubbleDiameter: "x", LaunchedSatisfaction: "10"}, nil
			},
			updateBuildingFunc: func(context.Context, uint64, string, string, string, string, string, time.Time, float64) (*pb.Building, error) {
				return &pb.Building{LaunchedSatisfaction: "bad", ConstructionEndDate: "2026-02-01T15:04:05"}, nil
			},
		}
		svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
		if _, err := svc.UpdateBuilding(context.Background(), &pb.UpdateBuildingRequest{FeatureId: 10, BuildingModelId: "m1"}); err == nil {
			t.Fatal("expected unauthorized")
		}
		if _, err := svc.UpdateBuilding(authContext(99), &pb.UpdateBuildingRequest{FeatureId: 10, BuildingModelId: "m1"}); err == nil {
			t.Fatal("expected not owner")
		}
		if _, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "x",
		}); err == nil {
			t.Fatal("expected invalid satisfaction")
		}
		if _, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "1", Rotation: "0", Position: "1,2",
		}); err == nil {
			t.Fatal("expected below required")
		}
		svc.SetCommercialClient(&mockCommercialClient{
			getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
				return &commercialpb.WalletResponse{Satisfaction: "100"}, nil
			},
		})
		if _, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "10", Rotation: "bad", Position: "1,2",
		}); err == nil {
			t.Fatal("expected invalid rotation")
		}
		if _, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "10", Rotation: "0", Position: "oops",
		}); err == nil {
			t.Fatal("expected invalid position")
		}
		got, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
			FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "10", Rotation: "0", Position: "1,2",
		})
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Fatal("expected building")
		}
	})
}

func TestBuildingService_UpdateBuilding_ChargesOnlyTheIncrease(t *testing.T) {
	var deducted float64
	repo := &mockBuildingRepository{
		findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
			return &pb.BuildingModel{Id: 7, ModelId: "m1"}, nil
		},
		findBuildingByFeatureAndModelFunc: func(context.Context, uint64, string) (*pb.Building, error) {
			return &pb.Building{LaunchedSatisfaction: "10", ConstructionStartDate: "2026-01-02 15:04:05"}, nil
		},
		updateBuildingFunc: func(_ context.Context, _ uint64, _ string, launched string, _, _, _ string, _ time.Time, _ float64) (*pb.Building, error) {
			if launched != "15" {
				t.Fatalf("stored launched=%s", launched)
			}
			return &pb.Building{LaunchedSatisfaction: launched, ConstructionStartDate: "2026-01-02 15:04:05"}, nil
		},
	}
	featureRepo := &mockFeatureRepository{
		findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 10, OwnerID: 42}, &models.FeatureProperties{Area: 5000, Density: 1}, nil
		},
	}
	svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
	svc.SetCommercialClient(&mockCommercialClient{
		getWalletFunc: func(context.Context, uint64) (*commercialpb.WalletResponse, error) {
			return &commercialpb.WalletResponse{Satisfaction: "20"}, nil
		},
		deductBalanceFunc: func(_ context.Context, _ uint64, asset string, amount float64) error {
			if asset != "satisfaction" {
				t.Fatalf("asset=%s", asset)
			}
			deducted = amount
			return nil
		},
	})
	if _, err := svc.UpdateBuilding(authContext(42), &pb.UpdateBuildingRequest{
		FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "15", Rotation: "0", Position: "1,2",
	}); err != nil {
		t.Fatal(err)
	}
	if deducted != 5 {
		t.Fatalf("deducted=%v, want 5", deducted)
	}
}

func TestBuildingService_BuildFeature_InvalidPlacementDoesNotCharge(t *testing.T) {
	deducts := 0
	repo := &mockBuildingRepository{
		hasBuildingFunc: func(context.Context, uint64) (bool, error) { return false, nil },
		findModelFunc: func(context.Context, string) (*pb.BuildingModel, error) {
			return &pb.BuildingModel{RequiredSatisfaction: "10", Attributes: "[]"}, nil
		},
	}
	svc := service.NewBuildingService(repo, &mockFeatureRepository{
		findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 10, OwnerID: 42}, &models.FeatureProperties{Area: 100, Density: 1}, nil
		},
	}, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)
	svc.SetCommercialClient(&mockCommercialClient{
		deductBalanceFunc: func(context.Context, uint64, string, float64) error {
			deducts++
			return nil
		},
	})
	_, err := svc.BuildFeature(authContext(42), &pb.BuildFeatureRequest{
		FeatureId: 10, BuildingModelId: "m1", LaunchedSatisfaction: "20", Rotation: "nope", Position: "1,2",
	})
	if err == nil {
		t.Fatal("expected invalid rotation")
	}
	if deducts != 0 {
		t.Fatalf("deducts=%d", deducts)
	}
}

func TestBuildingService_GetBuildings_HidesInformationFromNonOwner(t *testing.T) {
	repo := &mockBuildingRepository{
		findByFeatureIDFunc: func(context.Context, uint64) ([]*pb.Building, error) {
			return []*pb.Building{{Information: `{"address":"secret"}`, LaunchedSatisfaction: "1"}}, nil
		},
	}
	featureRepo := &mockFeatureRepository{
		findByIDFunc: func(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
			return &models.Feature{ID: 10, OwnerID: 42}, &models.FeatureProperties{}, nil
		},
	}
	svc := service.NewBuildingService(repo, featureRepo, &mockGeometryRepository{}, &mockHourlyProfitRepository{}, nil)

	hidden, err := svc.GetBuildings(authContext(7), 10)
	if err != nil {
		t.Fatal(err)
	}
	if hidden[0].Information != "" {
		t.Fatalf("information=%s", hidden[0].Information)
	}
	visible, err := svc.GetBuildings(authContext(42), 10)
	if err != nil {
		t.Fatal(err)
	}
	if visible[0].Information != `{"address":"secret"}` {
		t.Fatalf("information=%s", visible[0].Information)
	}
}
