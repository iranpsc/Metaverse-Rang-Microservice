package handler_test

import (
	"context"
	"errors"
	"testing"

	"metargb/features-service/internal/handler"
	pb "metargb/shared/pb/features"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockMapService implements service.MapService for testing
type mockMapService struct {
	listMapsFunc     func(ctx context.Context) ([]*pb.Map, error)
	getMapFunc       func(ctx context.Context, mapID uint64) (*pb.Map, error)
	getMapBorderFunc func(ctx context.Context, mapID uint64) (string, error)
}

func (m *mockMapService) ListMaps(ctx context.Context) ([]*pb.Map, error) {
	if m.listMapsFunc != nil {
		return m.listMapsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMapService) GetMap(ctx context.Context, mapID uint64) (*pb.Map, error) {
	if m.getMapFunc != nil {
		return m.getMapFunc(ctx, mapID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMapService) GetMapBorder(ctx context.Context, mapID uint64) (string, error) {
	if m.getMapBorderFunc != nil {
		return m.getMapBorderFunc(ctx, mapID)
	}
	return "", errors.New("not implemented")
}

func TestMapHandler_ListMaps(t *testing.T) {
	ctx := context.Background()

	t.Run("successful list maps", func(t *testing.T) {
		mockService := &mockMapService{}
		mockService.listMapsFunc = func(ctx context.Context) ([]*pb.Map, error) {
			return []*pb.Map{
				{
					Id:                      1,
					Name:                    "Map 1",
					Color:                   "red",
					CentralPointCoordinates: "[10,20]",
					SoldFeaturesPercentage:  "50.00",
				},
			}, nil
		}

		handler := handler.NewMapHandler(mockService)

		req := &pb.ListMapsRequest{}
		resp, err := handler.ListMaps(ctx, req)
		if err != nil {
			t.Fatalf("ListMaps failed: %v", err)
		}

		if len(resp.Maps) != 1 {
			t.Errorf("Expected 1 map, got %d", len(resp.Maps))
		}

		if resp.Maps[0].Id != 1 {
			t.Errorf("Expected map ID 1, got %d", resp.Maps[0].Id)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &mockMapService{}
		mockService.listMapsFunc = func(ctx context.Context) ([]*pb.Map, error) {
			return nil, errors.New("database error")
		}

		handler := handler.NewMapHandler(mockService)

		req := &pb.ListMapsRequest{}
		_, err := handler.ListMaps(ctx, req)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a gRPC status")
		}

		if st.Code() != codes.Internal {
			t.Errorf("Expected Internal error code, got %v", st.Code())
		}
	})
}

func TestMapHandler_GetMap(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get map", func(t *testing.T) {
		mockService := &mockMapService{}
		mockService.getMapFunc = func(ctx context.Context, mapID uint64) (*pb.Map, error) {
			return &pb.Map{
				Id:                     1,
				Name:                   "Map 1",
				Color:                  "blue",
				Area:                   5000,
				Address:                "Test Address",
				SoldFeaturesPercentage: "75.00",
				Features: &pb.MapFeatures{
					Maskoni:   &pb.MapFeatureCount{Sold: 2},
					Tejari:    &pb.MapFeatureCount{Sold: 1},
					Amoozeshi: &pb.MapFeatureCount{Sold: 1},
				},
			}, nil
		}

		handler := handler.NewMapHandler(mockService)

		req := &pb.GetMapRequest{MapId: 1}
		resp, err := handler.GetMap(ctx, req)
		if err != nil {
			t.Fatalf("GetMap failed: %v", err)
		}

		if resp.Map.Id != 1 {
			t.Errorf("Expected map ID 1, got %d", resp.Map.Id)
		}

		if resp.Map.Area != 5000 {
			t.Errorf("Expected area 5000, got %d", resp.Map.Area)
		}
	})

	t.Run("map not found", func(t *testing.T) {
		mockService := &mockMapService{}
		mockService.getMapFunc = func(ctx context.Context, mapID uint64) (*pb.Map, error) {
			return nil, errors.New("map not found")
		}

		handler := handler.NewMapHandler(mockService)

		req := &pb.GetMapRequest{MapId: 999}
		_, err := handler.GetMap(ctx, req)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a gRPC status")
		}

		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("invalid request - missing map_id", func(t *testing.T) {
		handler := handler.NewMapHandler(&mockMapService{})

		req := &pb.GetMapRequest{MapId: 0}
		_, err := handler.GetMap(ctx, req)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a gRPC status")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})
}

func TestMapHandler_GetMapBorder(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get border", func(t *testing.T) {
		mockService := &mockMapService{}
		mockService.getMapBorderFunc = func(ctx context.Context, mapID uint64) (string, error) {
			return "[[10,20],[30,40]]", nil
		}

		handler := handler.NewMapHandler(mockService)

		req := &pb.GetMapRequest{MapId: 1}
		resp, err := handler.GetMapBorder(ctx, req)
		if err != nil {
			t.Fatalf("GetMapBorder failed: %v", err)
		}

		if resp.Data.BorderCoordinates != "[[10,20],[30,40]]" {
			t.Errorf("Expected border '%s', got '%s'", "[[10,20],[30,40]]", resp.Data.BorderCoordinates)
		}
	})

	t.Run("map not found", func(t *testing.T) {
		mockService := &mockMapService{}
		mockService.getMapBorderFunc = func(ctx context.Context, mapID uint64) (string, error) {
			return "", errors.New("map not found")
		}

		handler := handler.NewMapHandler(mockService)

		req := &pb.GetMapRequest{MapId: 999}
		_, err := handler.GetMapBorder(ctx, req)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a gRPC status")
		}

		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("invalid request - missing map_id", func(t *testing.T) {
		handler := handler.NewMapHandler(&mockMapService{})

		req := &pb.GetMapRequest{MapId: 0}
		_, err := handler.GetMapBorder(ctx, req)
		if err == nil {
			t.Error("Expected error, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a gRPC status")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error code, got %v", st.Code())
		}
	})
}
