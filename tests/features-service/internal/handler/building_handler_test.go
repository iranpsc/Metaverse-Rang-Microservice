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

type mockBuildingPort struct {
	getBuildPackage func(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error)
	buildFeature    func(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error)
	getBuildings    func(ctx context.Context, featureID uint64) ([]*pb.Building, error)
	updateBuilding  func(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error)
	destroyBuilding func(ctx context.Context, featureID uint64, buildingModelID string) error
}

func (m *mockBuildingPort) GetBuildPackage(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error) {
	if m.getBuildPackage != nil {
		return m.getBuildPackage(ctx, featureID, page)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockBuildingPort) BuildFeature(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error) {
	if m.buildFeature != nil {
		return m.buildFeature(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingPort) GetBuildings(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
	if m.getBuildings != nil {
		return m.getBuildings(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingPort) UpdateBuilding(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error) {
	if m.updateBuilding != nil {
		return m.updateBuilding(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockBuildingPort) DestroyBuilding(ctx context.Context, featureID uint64, buildingModelID string) error {
	if m.destroyBuilding != nil {
		return m.destroyBuilding(ctx, featureID, buildingModelID)
	}
	return errors.New("not implemented")
}

func TestBuildingHandler_GetBuildPackage(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		m := &mockBuildingPort{}
		m.getBuildPackage = func(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error) {
			return []*pb.BuildingModel{{Id: 1, ModelId: "m1", Name: "Test"}}, []string{"1,2"}, nil
		}
		h := handler.NewBuildingHandler(m)
		resp, err := h.GetBuildPackage(ctx, &pb.GetBuildPackageRequest{FeatureId: 123, Page: 1})
		if err != nil {
			t.Fatal(err)
		}
		if len(resp.Models) != 1 || len(resp.Coordinates) != 1 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("error_feature_id_zero", func(t *testing.T) {
		h := handler.NewBuildingHandler(&mockBuildingPort{})
		_, err := h.GetBuildPackage(ctx, &pb.GetBuildPackageRequest{FeatureId: 0, Page: 1})
		if err == nil {
			t.Fatal("expected error")
		}
		if st, _ := status.FromError(err); st.Code() != codes.InvalidArgument {
			t.Fatalf("got %v", st.Code())
		}
	})

	t.Run("error_unauthorized", func(t *testing.T) {
		m := &mockBuildingPort{}
		m.getBuildPackage = func(ctx context.Context, featureID uint64, page int32) ([]*pb.BuildingModel, []string, error) {
			return nil, nil, errors.New("unauthorized: user does not own this feature")
		}
		h := handler.NewBuildingHandler(m)
		_, err := h.GetBuildPackage(ctx, &pb.GetBuildPackageRequest{FeatureId: 123, Page: 1})
		if st, _ := status.FromError(err); st.Code() != codes.PermissionDenied {
			t.Fatalf("got %v", st.Code())
		}
	})
}

func TestBuildingHandler_BuildFeature(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		m := &mockBuildingPort{}
		m.buildFeature = func(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error) {
			return &pb.Feature{Id: req.FeatureId}, nil
		}
		h := handler.NewBuildingHandler(m)
		resp, err := h.BuildFeature(ctx, &pb.BuildFeatureRequest{
			FeatureId:            123,
			BuildingModelId:      "model-1",
			LaunchedSatisfaction: "25.0",
			Rotation:             "0",
			Position:             "1,2",
		})
		if err != nil || resp.Feature == nil {
			t.Fatalf("err=%v resp=%+v", err, resp)
		}
	})

	t.Run("error_feature_id_zero", func(t *testing.T) {
		h := handler.NewBuildingHandler(&mockBuildingPort{})
		_, err := h.BuildFeature(ctx, &pb.BuildFeatureRequest{FeatureId: 0, BuildingModelId: "model-1"})
		if st, _ := status.FromError(err); st.Code() != codes.InvalidArgument {
			t.Fatalf("got %v", st.Code())
		}
	})

	t.Run("error_building_model_empty", func(t *testing.T) {
		h := handler.NewBuildingHandler(&mockBuildingPort{})
		_, err := h.BuildFeature(ctx, &pb.BuildFeatureRequest{FeatureId: 1, BuildingModelId: ""})
		if st, _ := status.FromError(err); st.Code() != codes.InvalidArgument {
			t.Fatalf("got %v", st.Code())
		}
	})

	t.Run("error_failed_precondition", func(t *testing.T) {
		m := &mockBuildingPort{}
		m.buildFeature = func(ctx context.Context, req *pb.BuildFeatureRequest) (*pb.Feature, error) {
			return nil, errors.New("feature already has a building")
		}
		h := handler.NewBuildingHandler(m)
		_, err := h.BuildFeature(ctx, &pb.BuildFeatureRequest{
			FeatureId:            1,
			BuildingModelId:      "model-1",
			LaunchedSatisfaction: "1",
			Rotation:             "0",
			Position:             "0,0",
		})
		if st, _ := status.FromError(err); st.Code() != codes.FailedPrecondition {
			t.Fatalf("got %v", st.Code())
		}
	})
}

func TestBuildingHandler_GetBuildings(t *testing.T) {
	ctx := context.Background()
	m := &mockBuildingPort{}
	m.getBuildings = func(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
		return []*pb.Building{{Id: 1}}, nil
	}
	h := handler.NewBuildingHandler(m)
	resp, err := h.GetBuildings(ctx, &pb.GetBuildingsRequest{FeatureId: 10})
	if err != nil || len(resp.Buildings) != 1 {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
}

func TestBuildingHandler_UpdateBuilding(t *testing.T) {
	ctx := context.Background()
	m := &mockBuildingPort{}
	m.updateBuilding = func(ctx context.Context, req *pb.UpdateBuildingRequest) (*pb.Building, error) {
		return &pb.Building{Id: 1}, nil
	}
	h := handler.NewBuildingHandler(m)
	resp, err := h.UpdateBuilding(ctx, &pb.UpdateBuildingRequest{
		FeatureId:            10,
		BuildingModelId:      "model-2",
		LaunchedSatisfaction: "10",
		Rotation:             "0",
		Position:             "0,0",
	})
	if err != nil || !resp.Success || resp.Building == nil {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
}

func TestBuildingHandler_DestroyBuilding(t *testing.T) {
	ctx := context.Background()
	m := &mockBuildingPort{}
	m.destroyBuilding = func(ctx context.Context, featureID uint64, buildingModelID string) error {
		return nil
	}
	h := handler.NewBuildingHandler(m)
	resp, err := h.DestroyBuilding(ctx, &pb.DestroyBuildingRequest{FeatureId: 10, BuildingModelId: "model-3"})
	if err != nil || !resp.Success {
		t.Fatalf("err=%v resp=%+v", err, resp)
	}
}

func TestBuildingHandler_DestroyBuilding_Unauthorized(t *testing.T) {
	ctx := context.Background()
	m := &mockBuildingPort{}
	m.destroyBuilding = func(ctx context.Context, featureID uint64, buildingModelID string) error {
		return errors.New("unauthorized: user does not own this feature")
	}
	h := handler.NewBuildingHandler(m)
	_, err := h.DestroyBuilding(ctx, &pb.DestroyBuildingRequest{FeatureId: 10, BuildingModelId: "model-3"})
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("got %v", st.Code())
	}
}

func TestBuildingHandler_GetBuildings_Error(t *testing.T) {
	ctx := context.Background()
	m := &mockBuildingPort{}
	m.getBuildings = func(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
		return nil, errors.New("db")
	}
	h := handler.NewBuildingHandler(m)
	_, err := h.GetBuildings(ctx, &pb.GetBuildingsRequest{FeatureId: 10})
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("got %v", st.Code())
	}
}

func TestBuildingHandler_UpdateBuilding_Invalid(t *testing.T) {
	ctx := context.Background()
	h := handler.NewBuildingHandler(&mockBuildingPort{})
	_, err := h.UpdateBuilding(ctx, &pb.UpdateBuildingRequest{FeatureId: 0, BuildingModelId: "model-1"})
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("got %v", st.Code())
	}
}
