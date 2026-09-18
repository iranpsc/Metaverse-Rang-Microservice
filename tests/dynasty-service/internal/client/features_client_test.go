package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/dynasty-service/internal/client"
	pb "metarang/shared/pb/features"
)

type stubFeatureServiceClient struct {
	pb.FeatureServiceClient
	getFeatureFunc     func(context.Context, *pb.GetFeatureRequest, ...grpc.CallOption) (*pb.FeatureResponse, error)
	getMyFeaturesFunc  func(context.Context, *pb.GetMyFeaturesRequest, ...grpc.CallOption) (*pb.FeaturesResponse, error)
	listMyFeaturesFunc func(context.Context, *pb.ListMyFeaturesRequest, ...grpc.CallOption) (*pb.ListMyFeaturesResponse, error)
}

func (s *stubFeatureServiceClient) GetFeature(ctx context.Context, in *pb.GetFeatureRequest, opts ...grpc.CallOption) (*pb.FeatureResponse, error) {
	if s.getFeatureFunc != nil {
		return s.getFeatureFunc(ctx, in, opts...)
	}
	return &pb.FeatureResponse{Feature: &pb.Feature{Id: in.FeatureId}}, nil
}
func (s *stubFeatureServiceClient) GetMyFeatures(ctx context.Context, in *pb.GetMyFeaturesRequest, opts ...grpc.CallOption) (*pb.FeaturesResponse, error) {
	if s.getMyFeaturesFunc != nil {
		return s.getMyFeaturesFunc(ctx, in, opts...)
	}
	return &pb.FeaturesResponse{Features: []*pb.Feature{{Id: 1}}}, nil
}
func (s *stubFeatureServiceClient) ListMyFeatures(ctx context.Context, in *pb.ListMyFeaturesRequest, opts ...grpc.CallOption) (*pb.ListMyFeaturesResponse, error) {
	if s.listMyFeaturesFunc != nil {
		return s.listMyFeaturesFunc(ctx, in, opts...)
	}
	return &pb.ListMyFeaturesResponse{Data: []*pb.Feature{{Id: 2}}}, nil
}

// Satisfy remaining interface methods via embedding zero value panic avoidance:
func (s *stubFeatureServiceClient) ListFeatures(context.Context, *pb.ListFeaturesRequest, ...grpc.CallOption) (*pb.FeaturesResponse, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) UpdateFeature(context.Context, *pb.UpdateFeatureRequest, ...grpc.CallOption) (*pb.FeatureResponse, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) AddFeatureImages(context.Context, *pb.AddFeatureImagesRequest, ...grpc.CallOption) (*pb.FeatureResponse, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) GetMyFeature(context.Context, *pb.GetMyFeatureRequest, ...grpc.CallOption) (*pb.FeatureResponse, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) AddMyFeatureImages(context.Context, *pb.AddMyFeatureImagesRequest, ...grpc.CallOption) (*pb.FeatureResponse, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) RemoveMyFeatureImage(context.Context, *pb.RemoveMyFeatureImageRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) UpdateMyFeature(context.Context, *pb.UpdateMyFeatureRequest, ...grpc.CallOption) (*pb.UpdateMyFeatureResponse, error) {
	return nil, nil
}
func (s *stubFeatureServiceClient) GetFeatureTradeHistory(context.Context, *pb.GetFeatureTradeHistoryRequest, ...grpc.CallOption) (*pb.GetFeatureTradeHistoryResponse, error) {
	return nil, nil
}

func TestFeaturesClient_GetAndList(t *testing.T) {
	stub := &stubFeatureServiceClient{}
	c := client.NewFeaturesClientFromGRPC(stub, nil)

	feat, err := c.GetFeature(context.Background(), 99)
	require.NoError(t, err)
	assert.Equal(t, uint64(99), feat.Id)

	list, err := c.GetMyFeatures(context.Background(), 1)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	paged, err := c.ListMyFeatures(context.Background(), 1, 2)
	require.NoError(t, err)
	assert.Len(t, paged, 1)
	require.NoError(t, c.Close())

	stub.getFeatureFunc = func(context.Context, *pb.GetFeatureRequest, ...grpc.CallOption) (*pb.FeatureResponse, error) {
		return nil, errors.New("fail")
	}
	_, err = c.GetFeature(context.Background(), 1)
	require.Error(t, err)

	stub.getMyFeaturesFunc = func(context.Context, *pb.GetMyFeaturesRequest, ...grpc.CallOption) (*pb.FeaturesResponse, error) {
		return nil, errors.New("fail")
	}
	_, err = c.GetMyFeatures(context.Background(), 1)
	require.Error(t, err)

	stub.listMyFeaturesFunc = func(context.Context, *pb.ListMyFeaturesRequest, ...grpc.CallOption) (*pb.ListMyFeaturesResponse, error) {
		return nil, errors.New("fail")
	}
	_, err = c.ListMyFeatures(context.Background(), 1, 1)
	require.Error(t, err)
}
