package handler_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"metargb/features-service/internal/handler"
	"metargb/features-service/internal/models"
	pb "metargb/shared/pb/features"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockMarketplace struct {
	buyFeature              func(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error)
	sendBuyRequest          func(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error)
	acceptBuyRequest        func(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error)
	createSellRequest       func(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error)
	listSellRequests        func(ctx context.Context, sellerID uint64) ([]*models.SellFeatureRequest, error)
	deleteSellRequest       func(ctx context.Context, sellRequestID, sellerID uint64) error
	requestGracePeriod      func(ctx context.Context, requestID, sellerID uint64, grace string) error
	listBuyRequests         func(ctx context.Context, buyerID uint64) ([]*models.BuyFeatureRequest, error)
	listReceivedBuyRequests func(ctx context.Context, sellerID uint64) ([]*models.BuyFeatureRequest, error)
	rejectBuyRequest        func(ctx context.Context, requestID, sellerID uint64) error
	deleteBuyRequest        func(ctx context.Context, requestID, buyerID uint64) error
	updateGracePeriod       func(ctx context.Context, requestID, sellerID uint64, days int32) error
	getUserCode             func(ctx context.Context, userID uint64) (string, error)
	getLatestProfilePhoto   func(ctx context.Context, userID uint64) (string, error)
}

func (m *mockMarketplace) BuyFeature(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error) {
	if m.buyFeature != nil {
		return m.buyFeature(ctx, featureID, buyerID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) SendBuyRequest(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error) {
	if m.sendBuyRequest != nil {
		return m.sendBuyRequest(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) AcceptBuyRequest(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error) {
	if m.acceptBuyRequest != nil {
		return m.acceptBuyRequest(ctx, requestID, sellerID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) CreateSellRequest(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error) {
	if m.createSellRequest != nil {
		return m.createSellRequest(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) ListSellRequests(ctx context.Context, sellerID uint64) ([]*models.SellFeatureRequest, error) {
	if m.listSellRequests != nil {
		return m.listSellRequests(ctx, sellerID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) DeleteSellRequest(ctx context.Context, sellRequestID, sellerID uint64) error {
	if m.deleteSellRequest != nil {
		return m.deleteSellRequest(ctx, sellRequestID, sellerID)
	}
	return errors.New("not implemented")
}

func (m *mockMarketplace) RequestGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriod string) error {
	if m.requestGracePeriod != nil {
		return m.requestGracePeriod(ctx, requestID, sellerID, gracePeriod)
	}
	return errors.New("not implemented")
}

func (m *mockMarketplace) ListBuyRequests(ctx context.Context, buyerID uint64) ([]*models.BuyFeatureRequest, error) {
	if m.listBuyRequests != nil {
		return m.listBuyRequests(ctx, buyerID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) ListReceivedBuyRequests(ctx context.Context, sellerID uint64) ([]*models.BuyFeatureRequest, error) {
	if m.listReceivedBuyRequests != nil {
		return m.listReceivedBuyRequests(ctx, sellerID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockMarketplace) RejectBuyRequest(ctx context.Context, requestID, sellerID uint64) error {
	if m.rejectBuyRequest != nil {
		return m.rejectBuyRequest(ctx, requestID, sellerID)
	}
	return errors.New("not implemented")
}

func (m *mockMarketplace) DeleteBuyRequest(ctx context.Context, requestID, buyerID uint64) error {
	if m.deleteBuyRequest != nil {
		return m.deleteBuyRequest(ctx, requestID, buyerID)
	}
	return errors.New("not implemented")
}

func (m *mockMarketplace) UpdateGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriodDays int32) error {
	if m.updateGracePeriod != nil {
		return m.updateGracePeriod(ctx, requestID, sellerID, gracePeriodDays)
	}
	return errors.New("not implemented")
}

func (m *mockMarketplace) GetUserCode(ctx context.Context, userID uint64) (string, error) {
	if m.getUserCode != nil {
		return m.getUserCode(ctx, userID)
	}
	return "U1", nil
}

func (m *mockMarketplace) GetLatestProfilePhoto(ctx context.Context, userID uint64) (string, error) {
	if m.getLatestProfilePhoto != nil {
		return m.getLatestProfilePhoto(ctx, userID)
	}
	return "", nil
}

type stubFeatureReader struct {
	find func(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error)
}

func (s *stubFeatureReader) FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
	if s.find != nil {
		return s.find(ctx, id)
	}
	return &models.Feature{ID: id, OwnerID: 1}, &models.FeatureProperties{
		ID: "fp1", FeatureID: id, Karbari: "m", MinimumPricePercentage: 100,
	}, nil
}

type stubGeometryReader struct {
	get func(ctx context.Context, featureID uint64) ([]*models.Coordinate, error)
}

func (s *stubGeometryReader) GetCoordinatesWithIDs(ctx context.Context, featureID uint64) ([]*models.Coordinate, error) {
	if s.get != nil {
		return s.get(ctx, featureID)
	}
	return []*models.Coordinate{{ID: 1, GeometryID: 1, X: 1.5, Y: 2.5}}, nil
}

func newTestMarketplaceHandler(m *mockMarketplace) *handler.MarketplaceHandler {
	return handler.NewMarketplaceHandler(m, &stubGeometryReader{}, &stubFeatureReader{})
}

func TestMarketplaceHandler_SendBuyRequest_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.SendBuyRequest(ctx, &pb.SendBuyRequestRequest{BuyerId: 0, FeatureId: 1})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_SendBuyRequest_Success(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	now := time.Now()
	m.sendBuyRequest = func(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error) {
		return &models.BuyFeatureRequest{
			ID: 9, BuyerID: 1, SellerID: 2, FeatureID: 100, Note: "n",
			PricePSC: 1, PriceIRR: 2, Status: 0,
			CreatedAt: now, UpdatedAt: now,
		}, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.SendBuyRequest(ctx, &pb.SendBuyRequestRequest{
		BuyerId: 1, FeatureId: 100, PricePsc: "1", PriceIrr: "2", Note: "n",
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, uint64(9), resp.Id)
}

func TestMarketplaceHandler_SendBuyRequest_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.sendBuyRequest = func(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error) {
		return nil, errors.New("موجودی کافی نیست")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.SendBuyRequest(ctx, &pb.SendBuyRequestRequest{
		BuyerId: 1, FeatureId: 100, PricePsc: "1", PriceIrr: "1",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_BuyFeature_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.BuyFeature(ctx, &pb.BuyFeatureRequest{FeatureId: 0, BuyerId: 1})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	_, err = h.BuyFeature(ctx, &pb.BuyFeatureRequest{FeatureId: 1, BuyerId: 0})
	require.Error(t, err)
	st, _ = status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_BuyFeature(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.buyFeature = func(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error) {
		return &pb.Feature{Id: featureID}, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.BuyFeature(ctx, &pb.BuyFeatureRequest{FeatureId: 5, BuyerId: 1})
	require.NoError(t, err)
	assert.True(t, resp.Success)
}

func TestMarketplaceHandler_BuyFeature_BalanceError(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.buyFeature = func(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error) {
		return nil, errors.New("insufficient balance")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.BuyFeature(ctx, &pb.BuyFeatureRequest{FeatureId: 5, BuyerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestMarketplaceHandler_AcceptBuyRequest_Unauthorized(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.acceptBuyRequest = func(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error) {
		return nil, errors.New("unauthorized: not the seller")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.AcceptBuyRequest(ctx, &pb.AcceptBuyRequestRequest{RequestId: 1, SellerId: 999})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestMarketplaceHandler_AcceptBuyRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.acceptBuyRequest = func(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error) {
		return nil, errors.New("buy request not found: sql: no rows")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.AcceptBuyRequest(ctx, &pb.AcceptBuyRequestRequest{RequestId: 99999, SellerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMarketplaceHandler_CreateSellRequest_MutuallyExclusive(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.CreateSellRequest(ctx, &pb.CreateSellRequestRequest{
		SellerId:               1,
		FeatureId:              10,
		PricePsc:               "10",
		MinimumPricePercentage: 50,
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_CreateSellRequest_Success(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	now := time.Now()
	m.createSellRequest = func(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error) {
		return &models.SellFeatureRequest{
			ID: 1, SellerID: req.SellerId, FeatureID: req.FeatureId,
			PricePSC: 0, PriceIRR: 0, Limit: 100, Status: 0,
			CreatedAt: now, UpdatedAt: now,
		}, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.CreateSellRequest(ctx, &pb.CreateSellRequestRequest{
		SellerId: 1, FeatureId: 20, MinimumPricePercentage: 90,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(20), resp.FeatureId)
}

func TestMarketplaceHandler_ListSellRequests_Empty(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.listSellRequests = func(ctx context.Context, sellerID uint64) ([]*models.SellFeatureRequest, error) {
		return nil, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.ListSellRequests(ctx, &pb.ListSellRequestsRequest{SellerId: 1})
	require.NoError(t, err)
	assert.Len(t, resp.SellRequests, 0)
}

func TestMarketplaceHandler_DeleteSellRequest(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.deleteSellRequest = func(ctx context.Context, sellRequestID, sellerID uint64) error {
		return nil
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.DeleteSellRequest(ctx, &pb.DeleteSellRequestRequest{SellRequestId: 1, SellerId: 2})
	require.NoError(t, err)
}

func TestMarketplaceHandler_ListBuyRequests_Empty(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.listBuyRequests = func(ctx context.Context, buyerID uint64) ([]*models.BuyFeatureRequest, error) {
		return []*models.BuyFeatureRequest{}, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.ListBuyRequests(ctx, &pb.ListBuyRequestsRequest{BuyerId: 1})
	require.NoError(t, err)
	assert.Len(t, resp.BuyRequests, 0)
}

func TestMarketplaceHandler_ListReceivedBuyRequests(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	now := time.Now()
	m.listReceivedBuyRequests = func(ctx context.Context, sellerID uint64) ([]*models.BuyFeatureRequest, error) {
		return []*models.BuyFeatureRequest{{
			ID: 3, BuyerID: 9, SellerID: sellerID, FeatureID: 100,
			CreatedAt: now, UpdatedAt: now,
		}}, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.ListReceivedBuyRequests(ctx, &pb.ListReceivedBuyRequestsRequest{SellerId: 7})
	require.NoError(t, err)
	require.Len(t, resp.BuyRequests, 1)
}

func TestMarketplaceHandler_RejectBuyRequest(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.rejectBuyRequest = func(ctx context.Context, requestID, sellerID uint64) error {
		return nil
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.RejectBuyRequest(ctx, &pb.RejectBuyRequestRequest{RequestId: 1, SellerId: 2})
	require.NoError(t, err)
}

func TestMarketplaceHandler_RejectBuyRequest_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.RejectBuyRequest(ctx, &pb.RejectBuyRequestRequest{RequestId: 0, SellerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_DeleteBuyRequest_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.DeleteBuyRequest(ctx, &pb.DeleteBuyRequestRequest{RequestId: 1, BuyerId: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_ListBuyRequests_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.ListBuyRequests(ctx, &pb.ListBuyRequestsRequest{BuyerId: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_ListReceivedBuyRequests_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.ListReceivedBuyRequests(ctx, &pb.ListReceivedBuyRequestsRequest{SellerId: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_ListSellRequests_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.ListSellRequests(ctx, &pb.ListSellRequestsRequest{SellerId: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_DeleteSellRequest_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.DeleteSellRequest(ctx, &pb.DeleteSellRequestRequest{SellRequestId: 0, SellerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_AcceptBuyRequest_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.AcceptBuyRequest(ctx, &pb.AcceptBuyRequestRequest{RequestId: 0, SellerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_RequestGracePeriod_Validation(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.RequestGracePeriod(ctx, &pb.RequestGracePeriodRequest{RequestId: 1, BuyerId: 2, GracePeriod: ""})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_CreateSellRequest_NeitherPriceNorPercent(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.CreateSellRequest(ctx, &pb.CreateSellRequestRequest{SellerId: 1, FeatureId: 10})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_BuyFeature_NotFound(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.buyFeature = func(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error) {
		return nil, errors.New("feature not found: x")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.BuyFeature(ctx, &pb.BuyFeatureRequest{FeatureId: 5, BuyerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMarketplaceHandler_SendBuyRequest_FailedPrecondition(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.sendBuyRequest = func(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error) {
		return nil, errors.New("قیمت مجاز نیست")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.SendBuyRequest(ctx, &pb.SendBuyRequestRequest{BuyerId: 1, FeatureId: 2, PricePsc: "1", PriceIrr: "1"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMarketplaceHandler_AcceptBuyRequest_FailedPrecondition(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.acceptBuyRequest = func(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error) {
		return nil, errors.New("صبر کنید — زیر قیمت")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.AcceptBuyRequest(ctx, &pb.AcceptBuyRequestRequest{RequestId: 1, SellerId: 2})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMarketplaceHandler_CreateSellRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.createSellRequest = func(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error) {
		return nil, errors.New("feature not found")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.CreateSellRequest(ctx, &pb.CreateSellRequestRequest{SellerId: 1, FeatureId: 9, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMarketplaceHandler_CreateSellRequest_Forbidden(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.createSellRequest = func(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error) {
		return nil, errors.New("unauthorized: not the owner")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.CreateSellRequest(ctx, &pb.CreateSellRequestRequest{SellerId: 1, FeatureId: 9, MinimumPricePercentage: 90})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestMarketplaceHandler_RejectBuyRequest_Internal(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.rejectBuyRequest = func(ctx context.Context, requestID, sellerID uint64) error {
		return errors.New("locked assets missing")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.RejectBuyRequest(ctx, &pb.RejectBuyRequestRequest{RequestId: 1, SellerId: 2})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestMarketplaceHandler_UpdateGracePeriod_NotFound(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.updateGracePeriod = func(ctx context.Context, requestID, sellerID uint64, days int32) error {
		return errors.New("buy request not found")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.UpdateGracePeriod(ctx, &pb.UpdateGracePeriodRequest{RequestId: 1, SellerId: 2, GracePeriodDays: 5})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMarketplaceHandler_UpdateGracePeriod_FailedPrecondition(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.updateGracePeriod = func(ctx context.Context, requestID, sellerID uint64, days int32) error {
		return errors.New("buy request is not pending")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.UpdateGracePeriod(ctx, &pb.UpdateGracePeriodRequest{RequestId: 1, SellerId: 2, GracePeriodDays: 5})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMarketplaceHandler_RequestGracePeriod_Unauthorized(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.requestGracePeriod = func(ctx context.Context, requestID, sellerID uint64, grace string) error {
		return errors.New("unauthorized: not the seller")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.RequestGracePeriod(ctx, &pb.RequestGracePeriodRequest{RequestId: 1, BuyerId: 2, GracePeriod: "5"})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestMarketplaceHandler_BuyFeature_FailedPrecondition(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.buyFeature = func(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error) {
		return nil, errors.New("خطایی در کمپین")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.BuyFeature(ctx, &pb.BuyFeatureRequest{FeatureId: 5, BuyerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMarketplaceHandler_DeleteSellRequest_NotFound(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.deleteSellRequest = func(ctx context.Context, sellRequestID, sellerID uint64) error {
		return errors.New("sell request not found")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.DeleteSellRequest(ctx, &pb.DeleteSellRequestRequest{SellRequestId: 3, SellerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestMarketplaceHandler_DeleteSellRequest_Forbidden(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.deleteSellRequest = func(ctx context.Context, sellRequestID, sellerID uint64) error {
		return errors.New("unauthorized: not the seller")
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.DeleteSellRequest(ctx, &pb.DeleteSellRequestRequest{SellRequestId: 3, SellerId: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestMarketplaceHandler_DeleteBuyRequest(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.deleteBuyRequest = func(ctx context.Context, requestID, buyerID uint64) error {
		return nil
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.DeleteBuyRequest(ctx, &pb.DeleteBuyRequestRequest{RequestId: 1, BuyerId: 9})
	require.NoError(t, err)
}

func TestMarketplaceHandler_UpdateGracePeriod(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.updateGracePeriod = func(ctx context.Context, requestID, sellerID uint64, days int32) error {
		return nil
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.UpdateGracePeriod(ctx, &pb.UpdateGracePeriodRequest{RequestId: 1, SellerId: 2, GracePeriodDays: 5})
	require.NoError(t, err)
}

func TestMarketplaceHandler_UpdateGracePeriod_InvalidRange(t *testing.T) {
	ctx := context.Background()
	h := newTestMarketplaceHandler(&mockMarketplace{})
	_, err := h.UpdateGracePeriod(ctx, &pb.UpdateGracePeriodRequest{RequestId: 1, SellerId: 2, GracePeriodDays: 0})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestMarketplaceHandler_RequestGracePeriod(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.requestGracePeriod = func(ctx context.Context, requestID, sellerID uint64, grace string) error {
		return nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.RequestGracePeriod(ctx, &pb.RequestGracePeriodRequest{
		RequestId: 1, BuyerId: 2, GracePeriod: "7",
	})
	require.NoError(t, err)
	assert.True(t, resp.Approved)
}

func TestMarketplaceHandler_RequestGracePeriod_InvalidGrace(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	m.requestGracePeriod = func(ctx context.Context, requestID, sellerID uint64, grace string) error {
		days, err := strconv.ParseInt(grace, 10, 32)
		if err != nil || days < 1 || days > 30 {
			return fmt.Errorf("grace period must be between 1 and 30 days")
		}
		return nil
	}
	h := newTestMarketplaceHandler(m)
	_, err := h.RequestGracePeriod(ctx, &pb.RequestGracePeriodRequest{
		RequestId: 1, BuyerId: 2, GracePeriod: "99",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestMarketplaceHandler_ListSellRequests_WithRows(t *testing.T) {
	ctx := context.Background()
	m := &mockMarketplace{}
	now := time.Now()
	m.listSellRequests = func(ctx context.Context, sellerID uint64) ([]*models.SellFeatureRequest, error) {
		return []*models.SellFeatureRequest{{
			ID: 1, SellerID: 1, FeatureID: 50, CreatedAt: now, UpdatedAt: now,
		}}, nil
	}
	h := newTestMarketplaceHandler(m)
	resp, err := h.ListSellRequests(ctx, &pb.ListSellRequestsRequest{SellerId: 1})
	require.NoError(t, err)
	require.Len(t, resp.SellRequests, 1)
	assert.Equal(t, uint64(50), resp.SellRequests[0].FeatureId)
}
