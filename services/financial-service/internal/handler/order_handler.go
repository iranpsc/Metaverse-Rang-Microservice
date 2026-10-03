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

func RegisterOrderHandler(grpcServer *grpc.Server, orderService service.OrderService) *OrderHandler {
	handler := NewOrderHandler(orderService)
	pb.RegisterOrderServiceServer(grpcServer, handler)
	return handler
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	locale := GetLocaleFromContext(ctx)
	validationErrors := mergeValidationErrors(
		validateMin("amount", int64(req.Amount), int64(constants.MinOrderAmount), locale),
		validateRequired("asset", req.Asset, locale),
		validateOneOf("asset", req.Asset, constants.ValidOrderAssets, locale),
	)
	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	link, err := h.orderService.CreateOrder(ctx, req.UserId, req.Amount, req.Asset)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAmount) || errors.Is(err, service.ErrInvalidAsset) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		if errors.Is(err, service.ErrUserNotEligible) {
			return nil, status.Errorf(codes.PermissionDenied, "%v", err)
		}
		if errors.Is(err, service.ErrPaymentFailed) {
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

	redirectURL, err := h.orderService.HandleCallback(ctx, req.OrderId, req.Token, req.ResCode, callbackAdditionalParams(req))
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

func callbackAdditionalParams(req *pb.HandleCallbackRequest) map[string]string {
	if params := req.GetAdditionalParams(); params != nil {
		return params
	}
	return map[string]string{}
}

func paymentFailedMessage(err error) string {
	prefix := service.ErrPaymentFailed.Error() + ": "
	if msg := strings.TrimPrefix(err.Error(), prefix); msg != err.Error() {
		return msg
	}
	return err.Error()
}
