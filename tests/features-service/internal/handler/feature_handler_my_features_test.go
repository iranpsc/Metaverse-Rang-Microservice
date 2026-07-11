package handler_test

import (
	"context"
	"errors"
	"testing"

	"metargb/features-service/internal/handler"
	pb "metargb/shared/pb/features"
	"metargb/shared/pkg/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func withUserID(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, auth.UserContextKey{}, &auth.UserContext{UserID: userID})
}

func TestFeatureHandler_ListMyFeatures_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.ListMyFeatures(context.Background(), &pb.ListMyFeaturesRequest{Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_ListMyFeatures_Success_WithNext(t *testing.T) {
	features := make([]*pb.Feature, 5)
	for i := range features {
		features[i] = &pb.Feature{Id: uint64(i + 1)}
	}
	m := &mockFeaturePort{}
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32) ([]*pb.Feature, error) {
		assert.Equal(t, uint64(42), userID)
		assert.Equal(t, int32(2), page)
		return features, nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 42)
	resp, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 2})
	require.NoError(t, err)
	require.Len(t, resp.Data, 5)
	require.NotEmpty(t, resp.Links.Next)
	assert.Contains(t, resp.Links.Next, "page=3")
	assert.Equal(t, "/api/my-features?page=1", resp.Links.First)
	assert.Equal(t, int32(2), resp.Meta.CurrentPage)
}

func TestFeatureHandler_ListMyFeatures_PageResetsToOne(t *testing.T) {
	m := &mockFeaturePort{}
	var gotPage int32
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32) ([]*pb.Feature, error) {
		gotPage = page
		return []*pb.Feature{{Id: 1}}, nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 1)
	_, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 0})
	require.NoError(t, err)
	assert.Equal(t, int32(1), gotPage)
}

func TestFeatureHandler_ListMyFeatures_InternalError(t *testing.T) {
	m := &mockFeaturePort{}
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32) ([]*pb.Feature, error) {
		return nil, errors.New("db down")
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 1)
	_, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_GetMyFeature_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.GetMyFeature(context.Background(), &pb.GetMyFeatureRequest{UserId: 1, FeatureId: 9})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_GetMyFeature_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	ctx := withUserID(context.Background(), 7)
	_, err := h.GetMyFeature(ctx, &pb.GetMyFeatureRequest{UserId: 99, FeatureId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestFeatureHandler_GetMyFeature_Success(t *testing.T) {
	m := &mockFeaturePort{}
	m.getMyFeature = func(ctx context.Context, userID, featureID uint64) (*pb.Feature, error) {
		return &pb.Feature{Id: featureID, OwnerId: userID}, nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 10)
	resp, err := h.GetMyFeature(ctx, &pb.GetMyFeatureRequest{UserId: 10, FeatureId: 55})
	require.NoError(t, err)
	assert.Equal(t, uint64(55), resp.Feature.Id)
}

func TestFeatureHandler_GetMyFeature_NotFound(t *testing.T) {
	m := &mockFeaturePort{}
	m.getMyFeature = func(ctx context.Context, userID, featureID uint64) (*pb.Feature, error) {
		return nil, errors.New("feature not found")
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 10)
	_, err := h.GetMyFeature(ctx, &pb.GetMyFeatureRequest{UserId: 10, FeatureId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.AddMyFeatureImages(context.Background(), &pb.AddMyFeatureImagesRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	ctx := withUserID(context.Background(), 1)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{UserId: 2, FeatureId: 9})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_NoImages(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	ctx := withUserID(context.Background(), 5)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{UserId: 5, FeatureId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_Success(t *testing.T) {
	var gotURLs []string
	m := &mockFeaturePort{}
	m.addMyImages = func(ctx context.Context, userID, featureID uint64, imageURLs []string) (*pb.Feature, error) {
		gotURLs = imageURLs
		return &pb.Feature{Id: featureID}, nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 3)
	resp, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{
		UserId:       3,
		FeatureId:    100,
		ImageData:    [][]byte{{1}, {2}},
		Filenames:    []string{"a.jpg", "b.jpg"},
		ContentTypes: []string{"image/jpeg", "image/jpeg"},
	})
	require.NoError(t, err)
	require.Len(t, gotURLs, 2)
	assert.Contains(t, gotURLs[0], "uploads/features/100/")
	assert.Equal(t, uint64(100), resp.Feature.Id)
}

func TestFeatureHandler_RemoveMyFeatureImage_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.RemoveMyFeatureImage(context.Background(), &pb.RemoveMyFeatureImageRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_RemoveMyFeatureImage_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	ctx := withUserID(context.Background(), 1)
	_, err := h.RemoveMyFeatureImage(ctx, &pb.RemoveMyFeatureImageRequest{UserId: 2})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestFeatureHandler_RemoveMyFeatureImage_Success(t *testing.T) {
	m := &mockFeaturePort{}
	m.removeMyImage = func(ctx context.Context, userID, featureID, imageID uint64) error {
		return nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 9)
	_, err := h.RemoveMyFeatureImage(ctx, &pb.RemoveMyFeatureImageRequest{UserId: 9, FeatureId: 1, ImageId: 88})
	require.NoError(t, err)
}

func TestFeatureHandler_RemoveMyFeatureImage_NotFound(t *testing.T) {
	m := &mockFeaturePort{}
	m.removeMyImage = func(ctx context.Context, userID, featureID, imageID uint64) error {
		return errors.New("image not found")
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 9)
	_, err := h.RemoveMyFeatureImage(ctx, &pb.RemoveMyFeatureImageRequest{UserId: 9, FeatureId: 1, ImageId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	_, err := h.UpdateMyFeature(context.Background(), &pb.UpdateMyFeatureRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	ctx := withUserID(context.Background(), 1)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 2, FeatureId: 1, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_MinimumBelow80(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{})
	ctx := withUserID(context.Background(), 1)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 1, FeatureId: 1, MinimumPricePercentage: 79})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_Success(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error {
		assert.Equal(t, int32(95), minimumPricePercentage)
		return nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 12)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 95})
	require.NoError(t, err)
}

func TestFeatureHandler_UpdateMyFeature_PersianValidationError(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error {
		return errors.New("حداقل درصد نامعتبر است")
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 12)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_NotFound(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error {
		return errors.New("feature not found in DB")
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 12)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_ReturnsEmpty(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) error {
		return nil
	}
	h := handler.NewFeatureHandler(m)
	ctx := withUserID(context.Background(), 12)
	out, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	require.NoError(t, err)
	require.NotNil(t, out)
}
