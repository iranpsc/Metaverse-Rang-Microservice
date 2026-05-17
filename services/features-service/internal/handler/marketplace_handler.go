package handler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"metargb/features-service/internal/models"
	"metargb/features-service/internal/repository"
	"metargb/features-service/internal/service"
	pb "metargb/shared/pb/features"
	"metargb/shared/pkg/helpers"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type MarketplaceHandler struct {
	pb.UnimplementedFeatureMarketplaceServiceServer
	service        *service.MarketplaceService
	geometryRepo   *repository.GeometryRepository
	propertiesRepo *repository.PropertiesRepository
	featureRepo    *repository.FeatureRepository
}

func NewMarketplaceHandler(service *service.MarketplaceService, geometryRepo *repository.GeometryRepository, propertiesRepo *repository.PropertiesRepository, featureRepo *repository.FeatureRepository) *MarketplaceHandler {
	return &MarketplaceHandler{
		service:        service,
		geometryRepo:   geometryRepo,
		propertiesRepo: propertiesRepo,
		featureRepo:    featureRepo,
	}
}

// BuyFeature handles direct feature purchase
// Implements POST /api/features/buy/{feature}
// Returns updated feature in response per documentation
func (h *MarketplaceHandler) BuyFeature(ctx context.Context, req *pb.BuyFeatureRequest) (*pb.BuyFeatureResponse, error) {
	locale := "en" // TODO: Get locale from config or context
	validationErrors := mergeValidationErrors(
		validateRequired("feature_id", req.FeatureId, locale),
		validateRequired("buyer_id", req.BuyerId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	// Execute purchase
	updatedFeature, err := h.service.BuyFeature(ctx, req.FeatureId, req.BuyerId)
	if err != nil {
		// Map service errors to appropriate gRPC status codes
		if strings.Contains(err.Error(), "موجودی") || strings.Contains(err.Error(), "balance") {
			return nil, status.Errorf(codes.PermissionDenied, "insufficient balance: %v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "feature not found: %v", err)
		}
		if strings.Contains(err.Error(), "خطایی") || strings.Contains(err.Error(), "campaign") {
			return nil, status.Errorf(codes.FailedPrecondition, "purchase failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to buy feature: %v", err)
	}

	return &pb.BuyFeatureResponse{
		Success: true,
		Message: "Feature purchased successfully",
		Feature: updatedFeature,
	}, nil
}

// SendBuyRequest creates a buy request for a feature
// Implements Laravel's BuyRequestsController@store
func (h *MarketplaceHandler) SendBuyRequest(ctx context.Context, req *pb.SendBuyRequestRequest) (*pb.BuyRequestResponse, error) {
	locale := "en" // TODO: Get locale from config or context
	validationErrors := mergeValidationErrors(
		validateRequired("feature_id", req.FeatureId, locale),
		validateRequired("buyer_id", req.BuyerId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	buyRequest, err := h.service.SendBuyRequest(ctx, req)
	if err != nil {
		// Map service errors to appropriate gRPC status codes
		if strings.Contains(err.Error(), "موجودی") {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		if strings.Contains(err.Error(), "مجاز") || strings.Contains(err.Error(), "price") {
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to send buy request: %v", err)
	}

	// Build full response
	return h.buildBuyRequestResponse(ctx, buyRequest)
}

// AcceptBuyRequest accepts a pending buy request
// Implements Laravel's BuyRequestsController@acceptBuyRequest
func (h *MarketplaceHandler) AcceptBuyRequest(ctx context.Context, req *pb.AcceptBuyRequestRequest) (*pb.BuyRequestResponse, error) {
	locale := "en" // TODO: Get locale from config or context
	validationErrors := mergeValidationErrors(
		validateRequired("request_id", req.RequestId, locale),
		validateRequired("seller_id", req.SellerId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	buyRequest, err := h.service.AcceptBuyRequest(ctx, req.RequestId, req.SellerId)
	if err != nil {
		// Map service errors
		if strings.Contains(err.Error(), "unauthorized") {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		if strings.Contains(err.Error(), "صبر") || strings.Contains(err.Error(), "زیر قیمت") {
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to accept buy request: %v", err)
	}

	// Build full response
	return h.buildBuyRequestResponse(ctx, buyRequest)
}

// CreateSellRequest creates a sell request for a feature
// Implements POST /api/sell-requests/store/{feature}
func (h *MarketplaceHandler) CreateSellRequest(ctx context.Context, req *pb.CreateSellRequestRequest) (*pb.SellRequestResponse, error) {
	locale := "en" // TODO: Get locale from config or context
	validationErrors := mergeValidationErrors(
		validateRequired("feature_id", req.FeatureId, locale),
		validateRequired("seller_id", req.SellerId, locale),
	)

	// Validate mutually exclusive fields
	hasExplicitPrices := (req.PricePsc != "" && req.PricePsc != "0") || (req.PriceIrr != "" && req.PriceIrr != "0")
	hasPercentage := req.MinimumPricePercentage > 0

	if hasExplicitPrices && hasPercentage {
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["price_psc"] = fmt.Sprintf(t.Invalid, "price_psc/price_irr and minimum_price_percentage are mutually exclusive")
		validationErrors["minimum_price_percentage"] = fmt.Sprintf(t.Invalid, "price_psc/price_irr and minimum_price_percentage are mutually exclusive")
	}

	if !hasExplicitPrices && !hasPercentage {
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["price_psc"] = fmt.Sprintf(t.Required, "either price_psc/price_irr or minimum_price_percentage")
		validationErrors["minimum_price_percentage"] = fmt.Sprintf(t.Required, "either price_psc/price_irr or minimum_price_percentage")
	}

	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	sellRequest, err := h.service.CreateSellRequest(ctx, req)
	if err != nil {
		// Map service errors to appropriate gRPC status codes
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "not the owner") {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		if strings.Contains(err.Error(), "مجاز") || strings.Contains(err.Error(), "mutually exclusive") || strings.Contains(err.Error(), "invalid") {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "at least one") {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to create sell request: %v", err)
	}

	// Build response with eager loaded feature data
	return h.buildSellRequestResponse(ctx, sellRequest)
}

// ListSellRequests lists all sell requests for a seller
// Implements GET /api/sell-requests
func (h *MarketplaceHandler) ListSellRequests(ctx context.Context, req *pb.ListSellRequestsRequest) (*pb.SellRequestsResponse, error) {
	if req.SellerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "seller_id is required")
	}

	requests, err := h.service.ListSellRequests(ctx, req.SellerId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list sell requests: %v", err)
	}

	responses := make([]*pb.SellRequestResponse, 0, len(requests))
	for _, req := range requests {
		resp, err := h.buildSellRequestResponse(ctx, req)
		if err != nil {
			continue // Skip on error
		}
		responses = append(responses, resp)
	}

	return &pb.SellRequestsResponse{
		SellRequests: responses,
	}, nil
}

// DeleteSellRequest deletes a sell request
// Implements DELETE /api/sell-requests/{sellRequest}
func (h *MarketplaceHandler) DeleteSellRequest(ctx context.Context, req *pb.DeleteSellRequestRequest) (*emptypb.Empty, error) {
	if req.SellRequestId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "sell_request_id is required")
	}
	if req.SellerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "seller_id is required")
	}

	err := h.service.DeleteSellRequest(ctx, req.SellRequestId, req.SellerId)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "not the seller") {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete sell request: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// buildSellRequestResponse builds a complete SellRequestResponse from a SellFeatureRequest model
// Eager loads feature properties and coordinates as per documentation
func (h *MarketplaceHandler) buildSellRequestResponse(ctx context.Context, sellRequest *models.SellFeatureRequest) (*pb.SellRequestResponse, error) {
	if sellRequest == nil {
		return nil, fmt.Errorf("sell request is nil")
	}

	response := &pb.SellRequestResponse{
		Id:        sellRequest.ID,
		SellerId:  sellRequest.SellerID,
		FeatureId: sellRequest.FeatureID,
		PricePsc:  fmt.Sprintf("%.10f", sellRequest.PricePSC),
		PriceIrr:  fmt.Sprintf("%.10f", sellRequest.PriceIRR),
		Status:    int32(sellRequest.Status),
		CreatedAt: helpers.FormatJalaliDate(sellRequest.CreatedAt),
	}

	// Get feature properties (eager loaded)
	_, properties, err := h.featureRepo.FindByID(ctx, sellRequest.FeatureID)
	if err == nil && properties != nil {
		response.FeatureProperties = models.PropertiesToPB(properties)
	}

	// Get feature coordinates (eager loaded)
	coordinates, err := h.geometryRepo.GetCoordinatesWithIDs(ctx, sellRequest.FeatureID)
	if err == nil {
		for _, coord := range coordinates {
			response.FeatureCoordinates = append(response.FeatureCoordinates, &pb.Coordinate{
				Id:         coord.ID,
				GeometryId: coord.GeometryID,
				X:          fmt.Sprintf("%.6f", coord.X),
				Y:          fmt.Sprintf("%.6f", coord.Y),
			})
		}
	}

	return response, nil
}

// ListBuyRequests lists all buy requests for a buyer
// Implements GET /api/buy-requests
func (h *MarketplaceHandler) ListBuyRequests(ctx context.Context, req *pb.ListBuyRequestsRequest) (*pb.BuyRequestsResponse, error) {
	if req.BuyerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "buyer_id is required")
	}

	requests, err := h.service.ListBuyRequests(ctx, req.BuyerId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list buy requests: %v", err)
	}

	responses := make([]*pb.BuyRequestResponse, 0, len(requests))
	for _, req := range requests {
		resp, err := h.buildBuyRequestResponse(ctx, req)
		if err != nil {
			continue // Skip on error
		}
		responses = append(responses, resp)
	}

	return &pb.BuyRequestsResponse{
		BuyRequests: responses,
	}, nil
}

// ListReceivedBuyRequests lists all buy requests received by a seller
// Implements GET /api/buy-requests/recieved
func (h *MarketplaceHandler) ListReceivedBuyRequests(ctx context.Context, req *pb.ListReceivedBuyRequestsRequest) (*pb.BuyRequestsResponse, error) {
	if req.SellerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "seller_id is required")
	}

	requests, err := h.service.ListReceivedBuyRequests(ctx, req.SellerId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list received buy requests: %v", err)
	}

	responses := make([]*pb.BuyRequestResponse, 0, len(requests))
	for _, req := range requests {
		resp, err := h.buildBuyRequestResponse(ctx, req)
		if err != nil {
			continue // Skip on error
		}
		responses = append(responses, resp)
	}

	return &pb.BuyRequestsResponse{
		BuyRequests: responses,
	}, nil
}

// RejectBuyRequest rejects a buy request
// Implements POST /api/buy-requests/reject/{buyFeatureRequest}
func (h *MarketplaceHandler) RejectBuyRequest(ctx context.Context, req *pb.RejectBuyRequestRequest) (*emptypb.Empty, error) {
	if req.RequestId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "request_id is required")
	}
	if req.SellerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "seller_id is required")
	}

	err := h.service.RejectBuyRequest(ctx, req.RequestId, req.SellerId)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to reject buy request: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// DeleteBuyRequest deletes a buy request
// Implements DELETE /api/buy-requests/delete/{buyFeatureRequest}
func (h *MarketplaceHandler) DeleteBuyRequest(ctx context.Context, req *pb.DeleteBuyRequestRequest) (*emptypb.Empty, error) {
	if req.RequestId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "request_id is required")
	}
	if req.BuyerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "buyer_id is required")
	}

	err := h.service.DeleteBuyRequest(ctx, req.RequestId, req.BuyerId)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete buy request: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// UpdateGracePeriod updates the grace period for a buy request
// Implements POST /api/buy-requests/add-grace-period/{buyFeatureRequest}
func (h *MarketplaceHandler) UpdateGracePeriod(ctx context.Context, req *pb.UpdateGracePeriodRequest) (*emptypb.Empty, error) {
	if req.RequestId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "request_id is required")
	}
	if req.SellerId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "seller_id is required")
	}
	if req.GracePeriodDays < 1 || req.GracePeriodDays > 30 {
		return nil, status.Errorf(codes.InvalidArgument, "grace_period_days must be between 1 and 30")
	}

	err := h.service.UpdateGracePeriod(ctx, req.RequestId, req.SellerId, req.GracePeriodDays)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		if strings.Contains(err.Error(), "not pending") || strings.Contains(err.Error(), "between 1 and 30") {
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to update grace period: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// RequestGracePeriod adds grace period to a buy request (deprecated)
// Implements Laravel's BuyRequestsController@addGracePeriod
func (h *MarketplaceHandler) RequestGracePeriod(ctx context.Context, req *pb.RequestGracePeriodRequest) (*pb.GracePeriodResponse, error) {
	if req.RequestId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "request_id is required")
	}

	// Parse grace period string to int
	gracePeriodDays, err := strconv.ParseInt(req.GracePeriod, 10, 32)
	if err != nil || gracePeriodDays < 1 || gracePeriodDays > 30 {
		return nil, status.Errorf(codes.InvalidArgument, "grace_period must be between 1 and 30")
	}

	// Deprecated RPC: grace period must be set by the seller (proto only carries buyer_id).
	sellerID, err := h.service.GetBuyRequestSellerID(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "buy request not found: %v", err)
	}
	err = h.service.UpdateGracePeriod(ctx, req.RequestId, sellerID, int32(gracePeriodDays))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to request grace period: %v", err)
	}

	return &pb.GracePeriodResponse{
		Approved: true,
		Message:  "Grace period added successfully",
	}, nil
}

// buildBuyRequestResponse builds a complete BuyRequestResponse from a BuyFeatureRequest model
func (h *MarketplaceHandler) buildBuyRequestResponse(ctx context.Context, buyRequest *models.BuyFeatureRequest) (*pb.BuyRequestResponse, error) {
	if buyRequest == nil {
		return nil, fmt.Errorf("buy request is nil")
	}

	response := &pb.BuyRequestResponse{
		Id:        buyRequest.ID,
		FeatureId: buyRequest.FeatureID,
		Status:    int32(buyRequest.Status),
		Note:      buyRequest.Note,
		PricePsc:  fmt.Sprintf("%.2f", buyRequest.PricePSC),
		PriceIrr:  fmt.Sprintf("%.0f", buyRequest.PriceIRR),
		CreatedAt: helpers.FormatJalaliDate(buyRequest.CreatedAt),
	}

	// Get buyer code and profile photo
	buyerCode, _ := h.service.GetUserCode(ctx, buyRequest.BuyerID)
	buyerPhoto, _ := h.service.GetLatestProfilePhoto(ctx, buyRequest.BuyerID)
	response.Buyer = &pb.BuyerInfo{
		Id:           buyRequest.BuyerID,
		Code:         buyerCode,
		ProfilePhoto: buyerPhoto,
	}

	// Get seller code
	sellerCode, _ := h.service.GetUserCode(ctx, buyRequest.SellerID)
	response.Seller = &pb.SellerInfo{
		Id:   buyRequest.SellerID,
		Code: sellerCode,
	}

	// Get feature properties
	_, properties, err := h.featureRepo.FindByID(ctx, buyRequest.FeatureID)
	if err == nil && properties != nil {
		response.FeatureProperties = models.PropertiesToPB(properties)
	}

	// Get feature coordinates
	coordinates, err := h.geometryRepo.GetCoordinatesWithIDs(ctx, buyRequest.FeatureID)
	if err == nil {
		for _, coord := range coordinates {
			response.FeatureCoordinates = append(response.FeatureCoordinates, &pb.Coordinate{
				Id:         coord.ID,
				GeometryId: coord.GeometryID,
				X:          fmt.Sprintf("%.6f", coord.X),
				Y:          fmt.Sprintf("%.6f", coord.Y),
			})
		}
	}

	// Format requested_grace_period if present
	if buyRequest.RequestedGracePeriod.Valid {
		response.RequestedGracePeriod = helpers.FormatJalaliDateTime(buyRequest.RequestedGracePeriod.Time)
	}

	return response, nil
}
