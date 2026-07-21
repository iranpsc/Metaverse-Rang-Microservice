// Package handler provides gRPC handlers for the financial service.
package handler

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/service"
	pb "metarang/shared/pb/financial"
)

type OrderHandler struct {
	pb.UnimplementedOrderServiceServer
	orderService service.OrderService
}

func NewOrderHandler(orderService service.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

func RegisterOrderHandler(grpcServer *grpc.Server, orderService service.OrderService) {
	handler := NewOrderHandler(orderService)
	pb.RegisterOrderServiceServer(grpcServer, handler)
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	locale := GetLocaleFromContext(ctx)
	validationErrors := mergeValidationErrors(
		validateMin("amount", int64(req.Amount), 1, locale),
		validateRequired("asset", req.Asset, locale),
		validateOneOf("asset", req.Asset, constants.ValidOrderAssets, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	// Call service
	link, err := h.orderService.CreateOrder(ctx, req.UserId, req.Amount, req.Asset)
	if err != nil {
		// Map service errors to gRPC status codes
		if errors.Is(err, service.ErrInvalidAmount) || errors.Is(err, service.ErrInvalidAsset) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		if errors.Is(err, service.ErrUserNotEligible) {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if errors.Is(err, service.ErrPaymentFailed) {
			// Laravel OrderController throws ValidationException with an "error" field (422)
			return nil, returnValidationError(map[string]string{
				"error": paymentFailedMessage(err),
			})
		}
		return nil, status.Errorf(codes.Internal, "failed to create order: %v", err)
	}

	return &pb.CreateOrderResponse{
		Link: link,
	}, nil
}

func (h *OrderHandler) HandleCallback(ctx context.Context, req *pb.HandleCallbackRequest) (*pb.HandleCallbackResponse, error) {
	locale := GetLocaleFromContext(ctx)
	validationErrors := mergeValidationErrors(
		validateRequired("order_id", req.OrderId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	// Convert additional params map
	additionalParams := make(map[string]string)
	if req.AdditionalParams != nil {
		for k, v := range req.AdditionalParams {
			additionalParams[k] = v
		}
	}

	// Call service
	redirectURL, err := h.orderService.HandleCallback(ctx, req.OrderId, req.Token, req.ResCode, additionalParams)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to handle callback: %v", err)
	}

	return &pb.HandleCallbackResponse{
		RedirectUrl: redirectURL,
	}, nil
}

func paymentFailedMessage(err error) string {
	prefix := service.ErrPaymentFailed.Error() + ": "
	if msg := strings.TrimPrefix(err.Error(), prefix); msg != err.Error() {
		return msg
	}
	return err.Error()
}
