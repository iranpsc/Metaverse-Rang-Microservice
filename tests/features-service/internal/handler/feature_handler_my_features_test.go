package handler_test

import (
	"context"
	"errors"
	"testing"

	"metarang/features-service/internal/handler"
	"metarang/features-service/internal/service"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func withUserID(ctx context.Context, userID uint64) context.Context {
	return context.WithValue(ctx, auth.UserContextKey{}, &auth.UserContext{UserID: userID})
}

func TestFeatureHandler_ListMyFeatures_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
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
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32, search, filter string) ([]*pb.Feature, error) {
		assert.Equal(t, uint64(42), userID)
		assert.Equal(t, int32(2), page)
		assert.Empty(t, search)
		assert.Empty(t, filter)
		return features, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 42)
	resp, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 2})
	require.NoError(t, err)
	require.Len(t, resp.Data, 5)
	require.NotEmpty(t, resp.Links.Next)
	assert.Contains(t, resp.Links.Next, "page=3")
	assert.Equal(t, "/api/my-features?page=1", resp.Links.First)
	assert.Equal(t, int32(2), resp.Meta.CurrentPage)
	assert.Equal(t, int32(5), resp.Meta.PerPage)
}

func TestFeatureHandler_ListMyFeatures_PageResetsToOne(t *testing.T) {
	m := &mockFeaturePort{}
	var gotPage int32
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32, search, filter string) ([]*pb.Feature, error) {
		gotPage = page
		return []*pb.Feature{{Id: 1}}, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 1)
	_, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 0})
	require.NoError(t, err)
	assert.Equal(t, int32(1), gotPage)
}

func TestFeatureHandler_ListMyFeatures_InternalError(t *testing.T) {
	m := &mockFeaturePort{}
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32, search, filter string) ([]*pb.Feature, error) {
		return nil, errors.New("db down")
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 1)
	_, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_ListMyFeatures_SearchAndFilter(t *testing.T) {
	m := &mockFeaturePort{}
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32, search, filter string) ([]*pb.Feature, error) {
		assert.Equal(t, uint64(42), userID)
		assert.Equal(t, int32(1), page)
		assert.Equal(t, "TO111", search)
		assert.Equal(t, "m", filter)
		return []*pb.Feature{{
			Id: 1,
			Properties: &pb.FeatureProperties{
				Id:      "TO111",
				Address: "Main Street 12",
				Karbari: "m",
			},
		}}, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 42)
	resp, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{
		Page:   1,
		Search: "  TO111  ",
		Filter: " m ",
	})
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "TO111", resp.Data[0].Properties.Id)
	assert.Equal(t, "Main Street 12", resp.Data[0].Properties.Address)
	assert.Equal(t, "m", resp.Data[0].Properties.Karbari)
	assert.Equal(t, "/api/my-features?filter=m&page=1&search=TO111", resp.Links.First)
	assert.Empty(t, resp.Links.Next)
	assert.Equal(t, int32(5), resp.Meta.PerPage)
}

func TestFeatureHandler_ListMyFeatures_SearchFilterPaginationLinks(t *testing.T) {
	features := make([]*pb.Feature, 5)
	for i := range features {
		features[i] = &pb.Feature{Id: uint64(i + 1)}
	}
	m := &mockFeaturePort{}
	m.listMyFeatures = func(ctx context.Context, userID uint64, page int32, search, filter string) ([]*pb.Feature, error) {
		assert.Equal(t, int32(2), page)
		assert.Equal(t, "block", search)
		assert.Equal(t, "t", filter)
		return features, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 7)
	resp, err := h.ListMyFeatures(ctx, &pb.ListMyFeaturesRequest{Page: 2, Search: "block", Filter: "t"})
	require.NoError(t, err)
	assert.Equal(t, "/api/my-features?filter=t&page=1&search=block", resp.Links.First)
	assert.Equal(t, "/api/my-features?filter=t&page=1&search=block", resp.Links.Prev)
	assert.Equal(t, "/api/my-features?filter=t&page=3&search=block", resp.Links.Next)
}

func TestFeatureHandler_GetMyFeature_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	_, err := h.GetMyFeature(context.Background(), &pb.GetMyFeatureRequest{UserId: 1, FeatureId: 9})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_GetMyFeature_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
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
	h := handler.NewFeatureHandler(m, nil)
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
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 10)
	_, err := h.GetMyFeature(ctx, &pb.GetMyFeatureRequest{UserId: 10, FeatureId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	_, err := h.AddMyFeatureImages(context.Background(), &pb.AddMyFeatureImagesRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	ctx := withUserID(context.Background(), 1)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{UserId: 2, FeatureId: 9})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_NoImages(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	ctx := withUserID(context.Background(), 5)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{UserId: 5, FeatureId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_Success(t *testing.T) {
	var gotData [][]byte
	var gotNames, gotTypes []string
	m := &mockFeaturePort{}
	m.addMyImages = func(ctx context.Context, userID, featureID uint64, imageData [][]byte, filenames, contentTypes []string) (*pb.Feature, error) {
		gotData = imageData
		gotNames = filenames
		gotTypes = contentTypes
		return &pb.Feature{Id: featureID, Images: []*pb.Image{{Id: 1, Url: "http://localhost:8000/uploads/features/100/a.jpg"}}}, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 3)
	resp, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{
		UserId:       3,
		FeatureId:    100,
		ImageData:    [][]byte{{1}, {2}},
		Filenames:    []string{"a.jpg", "b.jpg"},
		ContentTypes: []string{"image/jpeg", "image/jpeg"},
	})
	require.NoError(t, err)
	require.Len(t, gotData, 2)
	assert.Equal(t, [][]byte{{1}, {2}}, gotData)
	assert.Equal(t, []string{"a.jpg", "b.jpg"}, gotNames)
	assert.Equal(t, []string{"image/jpeg", "image/jpeg"}, gotTypes)
	assert.Equal(t, uint64(100), resp.Feature.Id)
	assert.Equal(t, "http://localhost:8000/uploads/features/100/a.jpg", resp.Feature.Images[0].Url)
}

func TestFeatureHandler_AddMyFeatureImages_InvalidType(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	ctx := withUserID(context.Background(), 5)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{
		UserId:       5,
		FeatureId:    1,
		ImageData:    [][]byte{{1}},
		Filenames:    []string{"a.gif"},
		ContentTypes: []string{"image/gif"},
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_TooLarge(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	ctx := withUserID(context.Background(), 5)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{
		UserId:       5,
		FeatureId:    1,
		ImageData:    [][]byte{make([]byte, 1024*1024+1)},
		Filenames:    []string{"a.jpg"},
		ContentTypes: []string{"image/jpeg"},
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_AddMyFeatureImages_StorageUnavailable(t *testing.T) {
	m := &mockFeaturePort{}
	m.addMyImages = func(ctx context.Context, userID, featureID uint64, imageData [][]byte, filenames, contentTypes []string) (*pb.Feature, error) {
		return nil, service.ErrStorageUnavailable
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 3)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{
		UserId:       3,
		FeatureId:    100,
		ImageData:    [][]byte{{1}},
		Filenames:    []string{"a.jpg"},
		ContentTypes: []string{"image/jpeg"},
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "storage service not available")
}

func TestFeatureHandler_AddMyFeatureImages_NotFound(t *testing.T) {
	m := &mockFeaturePort{}
	m.addMyImages = func(ctx context.Context, userID, featureID uint64, imageData [][]byte, filenames, contentTypes []string) (*pb.Feature, error) {
		return nil, errors.New("feature not found or does not belong to user")
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 3)
	_, err := h.AddMyFeatureImages(ctx, &pb.AddMyFeatureImagesRequest{
		UserId:       3,
		FeatureId:    100,
		ImageData:    [][]byte{{1}},
		Filenames:    []string{"a.jpg"},
		ContentTypes: []string{"image/jpeg"},
	})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_RemoveMyFeatureImage_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	_, err := h.RemoveMyFeatureImage(context.Background(), &pb.RemoveMyFeatureImageRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_RemoveMyFeatureImage_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
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
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 9)
	_, err := h.RemoveMyFeatureImage(ctx, &pb.RemoveMyFeatureImageRequest{UserId: 9, FeatureId: 1, ImageId: 88})
	require.NoError(t, err)
}

func TestFeatureHandler_RemoveMyFeatureImage_NotFound(t *testing.T) {
	m := &mockFeaturePort{}
	m.removeMyImage = func(ctx context.Context, userID, featureID, imageID uint64) error {
		return errors.New("image not found")
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 9)
	_, err := h.RemoveMyFeatureImage(ctx, &pb.RemoveMyFeatureImageRequest{UserId: 9, FeatureId: 1, ImageId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_Unauthenticated(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	_, err := h.UpdateMyFeature(context.Background(), &pb.UpdateMyFeatureRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_ScopeMismatch(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	ctx := withUserID(context.Background(), 1)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 2, FeatureId: 1, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_MinimumBelow80(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, nil)
	ctx := withUserID(context.Background(), 1)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 1, FeatureId: 1, MinimumPricePercentage: 79})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_Success(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
		assert.Equal(t, int32(95), minimumPricePercentage)
		return &pb.UpdateMyFeatureResponse{PricePsc: "12.5", PriceIrr: "450"}, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 12)
	resp, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 95})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "12.5", resp.PricePsc)
	assert.Equal(t, "450", resp.PriceIrr)
}

func TestFeatureHandler_UpdateMyFeature_PersianValidationError(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
		return nil, errors.New("حداقل درصد نامعتبر است")
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 12)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_NotFound(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
		return nil, errors.New("feature not found in DB")
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 12)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_ReturnsPrices(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
		return &pb.UpdateMyFeatureResponse{PricePsc: "1.25", PriceIrr: "250.5"}, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 12)
	out, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "1.25", out.PricePsc)
	assert.Equal(t, "250.5", out.PriceIrr)
}

func TestFeatureHandler_UpdateMyFeature_InternalError(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
		return nil, errors.New("db down")
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 12)
	_, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestFeatureHandler_UpdateMyFeature_NilResult(t *testing.T) {
	m := &mockFeaturePort{}
	m.updateMyFeature = func(ctx context.Context, userID, featureID uint64, minimumPricePercentage int32) (*pb.UpdateMyFeatureResponse, error) {
		return nil, nil
	}
	h := handler.NewFeatureHandler(m, nil)
	ctx := withUserID(context.Background(), 12)
	out, err := h.UpdateMyFeature(ctx, &pb.UpdateMyFeatureRequest{UserId: 12, FeatureId: 5, MinimumPricePercentage: 90})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Empty(t, out.PricePsc)
	assert.Empty(t, out.PriceIrr)
}
