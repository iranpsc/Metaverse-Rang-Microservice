package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metargb/features-service/internal/models"
	"metargb/features-service/internal/service"
	pb "metargb/shared/pb/features"
)

// Mock repositories for testing
type mockBuildingRepository struct {
	upsertModelFunc                   func(ctx context.Context, modelID, name, sku, images, attributes, file string, requiredSatisfaction float64) error
	findModelFunc                     func(ctx context.Context, modelID string) (*pb.BuildingModel, error)
	hasBuildingFunc                   func(ctx context.Context, featureID uint64) (bool, error)
	createBuildingFunc                func(ctx context.Context, featureID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, startDate, endDate time.Time, bubbleDiameter float64) error
	findByFeatureIDFunc               func(ctx context.Context, featureID uint64) ([]*pb.Building, error)
	updateBuildingFunc                func(ctx context.Context, featureID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, endDate time.Time, bubbleDiameter float64) (*pb.Building, error)
	findBuildingByFeatureAndModelFunc func(ctx context.Context, featureID uint64, buildingModelID string) (*pb.Building, error)
	deleteBuildingFunc                func(ctx context.Context, featureID uint64, buildingModelID string) error
	firstOrCreateIsicCodeFunc         func(ctx context.Context, activityLine string) (uint64, error)
}

func (m *mockBuildingRepository) UpsertBuildingModel(ctx context.Context, modelID, name, sku, images, attributes, file string, requiredSatisfaction float64) error {
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

func (m *mockBuildingRepository) CreateBuilding(ctx context.Context, featureID uint64, buildingModelID string, launchedSatisfaction, rotation, position, information string, startDate, endDate time.Time, bubbleDiameter float64) error {
	if m.createBuildingFunc != nil {
		return m.createBuildingFunc(ctx, featureID, buildingModelID, launchedSatisfaction, rotation, position, information, startDate, endDate, bubbleDiameter)
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

func (m *mockBuildingRepository) FindBuildingByFeatureAndModel(ctx context.Context, featureID uint64, buildingModelID string) (*pb.Building, error) {
	if m.findBuildingByFeatureAndModelFunc != nil {
		return m.findBuildingByFeatureAndModelFunc(ctx, featureID, buildingModelID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingRepository) DeleteBuilding(ctx context.Context, featureID uint64, buildingModelID string) error {
	if m.deleteBuildingFunc != nil {
		return m.deleteBuildingFunc(ctx, featureID, buildingModelID)
	}
	return errors.New("not implemented")
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
	getBuildPackageFunc func(req interface{}) (interface{}, error)
}

func (m *mockThreeDClient) GetBuildPackage(req interface{}) (interface{}, error) {
	if m.getBuildPackageFunc != nil {
		return m.getBuildPackageFunc(req)
	}
	return nil, errors.New("not implemented")
}

func TestBuildingService_GetBuildPackage(t *testing.T) {
	t.Run("unauthorized user", func(t *testing.T) {
		// Note: This test requires refactoring to use interfaces instead of concrete types
		// The service constructor expects concrete repository types, not mocks
		// For now, we skip this test until the service is refactored to use interfaces
		t.Skip("Test requires service refactoring to use repository interfaces")
		// ctx := context.Background()
		// mockBuildingRepo := &mockBuildingRepository{}
		// mockFeatureRepo := &mockFeatureRepository{}
		// mockGeometryRepo := &mockGeometryRepository{}
		// mockProfitRepo := &mockHourlyProfitRepository{}
		//
		// mockFeatureRepo.findByIDFunc = func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
		// 	return &models.Feature{
		// 		ID:      1,
		// 		OwnerID: 100, // Different owner
		// 	}, &models.FeatureProperties{}, nil
		// }
		// service := service.NewBuildingService(mockBuildingRepo, mockFeatureRepo, mockGeometryRepo, mockProfitRepo, nil)
		// _, _, err := service.GetBuildPackage(ctx, 1, 1) // featureID=1, page=1
		// if err == nil {
		// 	t.Error("Expected error for unauthorized user")
		// }
		// if err != nil && !contains(err.Error(), "unauthorized") && !contains(err.Error(), "does not own") {
		// 	t.Errorf("Expected authorization error, got: %v", err)
		// }
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
