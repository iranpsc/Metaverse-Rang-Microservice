package handler_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"metargb/features-service/internal/handler"
	pb "metargb/shared/pb/features"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockFeaturePort struct {
	listFeatures    func(ctx context.Context, points []string, loadBuildings bool, userFeaturesLocation bool, authUserID uint64) ([]*pb.Feature, error)
	getFeature      func(ctx context.Context, featureID uint64) (*pb.Feature, error)
	updateFeature   func(ctx context.Context, featureID uint64, properties *pb.FeatureProperties) (*pb.Feature, error)
	addImages       func(ctx context.Context, featureID uint64, imageURLs []string) (*pb.Feature, error)
	getMyFeatures   func(ctx context.Context, userID uint64) ([]*pb.Feature, error)
	listMyFeatures  func(ctx context.Context, userID uint64, page int32) ([]*pb.Feature, error)
	getMyFeature    func(ctx context.Context, userID, featureID uint64) (*pb.Feature, error)
	addMyImages     func(ctx context.Context, userID, featureID uint64, imageURLs []string) (*pb.Feature, error)
	removeMyImage   func(ctx context.Context, userID, featureID, imageID uint64) error
	updateMyFeature func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error
}

func (m *mockFeaturePort) ListFeatures(ctx context.Context, points []string, loadBuildings bool, userFeaturesLocation bool, authUserID uint64) ([]*pb.Feature, error) {
	if m.listFeatures != nil {
		return m.listFeatures(ctx, points, loadBuildings, userFeaturesLocation, authUserID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) GetFeature(ctx context.Context, featureID uint64) (*pb.Feature, error) {
	if m.getFeature != nil {
		return m.getFeature(ctx, featureID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) UpdateFeature(ctx context.Context, featureID uint64, properties *pb.FeatureProperties) (*pb.Feature, error) {
	if m.updateFeature != nil {
		return m.updateFeature(ctx, featureID, properties)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) AddFeatureImages(ctx context.Context, featureID uint64, imageURLs []string) (*pb.Feature, error) {
	if m.addImages != nil {
		return m.addImages(ctx, featureID, imageURLs)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) GetMyFeatures(ctx context.Context, userID uint64) ([]*pb.Feature, error) {
	if m.getMyFeatures != nil {
		return m.getMyFeatures(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) ListMyFeatures(ctx context.Context, userID uint64, page int32) ([]*pb.Feature, error) {
	if m.listMyFeatures != nil {
		return m.listMyFeatures(ctx, userID, page)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) GetMyFeature(ctx context.Context, userID, featureID uint64) (*pb.Feature, error) {
	if m.getMyFeature != nil {
		return m.getMyFeature(ctx, userID, featureID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) AddMyFeatureImages(ctx context.Context, userID, featureID uint64, imageURLs []string) (*pb.Feature, error) {
	if m.addMyImages != nil {
		return m.addMyImages(ctx, userID, featureID, imageURLs)
	}
	return nil, errors.New("not implemented")
}

func (m *mockFeaturePort) RemoveMyFeatureImage(ctx context.Context, userID, featureID, imageID uint64) error {
	if m.removeMyImage != nil {
		return m.removeMyImage(ctx, userID, featureID, imageID)
	}
	return errors.New("not implemented")
}

func (m *mockFeaturePort) UpdateMyFeature(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error {
	if m.updateMyFeature != nil {
		return m.updateMyFeature(ctx, userID, featureID, minimumPricePercentage)
	}
	return errors.New("not implemented")
}

func bboxPoints() []string {
	return []string{"0,0", "1,0", "1,1", "0,1"}
}

func TestFeatureHandler_ListFeatures_Validation(t *testing.T) {
	ctx := context.Background()
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.ListFeatures(ctx, &pb.ListFeaturesRequest{Points: []string{"a", "b"}})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_ListFeatures_InvalidCoordinate(t *testing.T) {
	ctx := context.Background()
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	pts := []string{"0,0", "1,0", "1,1", "x,y"}
	_, err := h.ListFeatures(ctx, &pb.ListFeaturesRequest{Points: pts})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_ListFeatures_Success(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.listFeatures = func(ctx context.Context, points []string, loadBuildings bool, userFeaturesLocation bool, authUserID uint64) ([]*pb.Feature, error) {
		return []*pb.Feature{{Id: 1}}, nil
	}
	h := handler.NewFeatureHandler(m)
	resp, err := h.ListFeatures(ctx, &pb.ListFeaturesRequest{Points: bboxPoints()})
	require.NoError(t, err)
	require.Len(t, resp.Features, 1)
}

func TestFeatureHandler_GetFeature(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.getFeature = func(ctx context.Context, featureID uint64) (*pb.Feature, error) {
		return &pb.Feature{Id: featureID}, nil
	}
	h := handler.NewFeatureHandler(m)
	resp, err := h.GetFeature(ctx, &pb.GetFeatureRequest{FeatureId: 42})
	require.NoError(t, err)
	assert.Equal(t, uint64(42), resp.Feature.Id)
}

func TestFeatureHandler_GetFeature_MissingId(t *testing.T) {
	ctx := context.Background()
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.GetFeature(ctx, &pb.GetFeatureRequest{FeatureId: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_GetFeature_NotFound(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.getFeature = func(ctx context.Context, featureID uint64) (*pb.Feature, error) {
		return nil, fmt.Errorf("feature not found: %w", sql.ErrNoRows)
	}
	h := handler.NewFeatureHandler(m)
	_, err := h.GetFeature(ctx, &pb.GetFeatureRequest{FeatureId: 99})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_GetFeature_InternalError(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.getFeature = func(ctx context.Context, featureID uint64) (*pb.Feature, error) {
		return nil, errors.New("database timeout")
	}
	h := handler.NewFeatureHandler(m)
	_, err := h.GetFeature(ctx, &pb.GetFeatureRequest{FeatureId: 99})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_ListFeatures_ServiceError(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.listFeatures = func(ctx context.Context, points []string, loadBuildings bool, userFeaturesLocation bool, authUserID uint64) ([]*pb.Feature, error) {
		return nil, errors.New("db down")
	}
	h := handler.NewFeatureHandler(m)
	_, err := h.ListFeatures(ctx, &pb.ListFeaturesRequest{Points: bboxPoints()})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_UpdateFeature(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.updateFeature = func(ctx context.Context, featureID uint64, properties *pb.FeatureProperties) (*pb.Feature, error) {
		return &pb.Feature{Id: featureID}, nil
	}
	h := handler.NewFeatureHandler(m)
	resp, err := h.UpdateFeature(ctx, &pb.UpdateFeatureRequest{FeatureId: 3, Properties: &pb.FeatureProperties{}})
	require.NoError(t, err)
	assert.Equal(t, uint64(3), resp.Feature.Id)
}

func TestFeatureHandler_AddFeatureImages(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.addImages = func(ctx context.Context, featureID uint64, imageURLs []string) (*pb.Feature, error) {
		return &pb.Feature{Id: featureID}, nil
	}
	h := handler.NewFeatureHandler(m)
	resp, err := h.AddFeatureImages(ctx, &pb.AddFeatureImagesRequest{FeatureId: 5, ImageUrls: []string{"http://x/y.jpg"}})
	require.NoError(t, err)
	assert.Equal(t, uint64(5), resp.Feature.Id)
}

func TestFeatureHandler_AddFeatureImages_NoUrls(t *testing.T) {
	ctx := context.Background()
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.AddFeatureImages(ctx, &pb.AddFeatureImagesRequest{FeatureId: 5, ImageUrls: nil})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_GetMyFeatures(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.getMyFeatures = func(ctx context.Context, userID uint64) ([]*pb.Feature, error) {
		return []*pb.Feature{{Id: 7}}, nil
	}
	h := handler.NewFeatureHandler(m)
	resp, err := h.GetMyFeatures(ctx, &pb.GetMyFeaturesRequest{UserId: 99})
	require.NoError(t, err)
	require.Len(t, resp.Features, 1)
}

func TestFeatureHandler_GetMyFeatures_Error(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.getMyFeatures = func(ctx context.Context, userID uint64) ([]*pb.Feature, error) {
		return nil, errors.New("db")
	}
	h := handler.NewFeatureHandler(m)
	_, err := h.GetMyFeatures(ctx, &pb.GetMyFeaturesRequest{UserId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_UpdateFeature_Error(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.updateFeature = func(ctx context.Context, featureID uint64, properties *pb.FeatureProperties) (*pb.Feature, error) {
		return nil, errors.New("failed")
	}
	h := handler.NewFeatureHandler(m)
	_, err := h.UpdateFeature(ctx, &pb.UpdateFeatureRequest{FeatureId: 1, Properties: &pb.FeatureProperties{}})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_AddFeatureImages_Error(t *testing.T) {
	ctx := context.Background()
	m := &mockFeaturePort{}
	m.addImages = func(ctx context.Context, featureID uint64, imageURLs []string) (*pb.Feature, error) {
		return nil, errors.New("failed")
	}
	h := handler.NewFeatureHandler(m)
	_, err := h.AddFeatureImages(ctx, &pb.AddFeatureImagesRequest{FeatureId: 1, ImageUrls: []string{"http://x"}})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_GetMyFeatures_MissingUser(t *testing.T) {
	ctx := context.Background()
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.GetMyFeatures(ctx, &pb.GetMyFeaturesRequest{UserId: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}
