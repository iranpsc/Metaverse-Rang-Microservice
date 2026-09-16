package handler_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"metarang/financial-service/internal/handler"
	"metarang/financial-service/internal/service"
	pb "metarang/shared/pb/financial"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockOrderService implements service.OrderService for testing
type mockOrderService struct {
	createOrderFunc func(ctx context.Context, userID uint64, amount int32, asset string) (string, error)
	callbackFunc    func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error)
}

func (m *mockOrderService) CreateOrder(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
	if m.createOrderFunc != nil {
		return m.createOrderFunc(ctx, userID, amount, asset)
	}
	return "", nil
}

func (m *mockOrderService) HandleCallback(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
	if m.callbackFunc != nil {
		return m.callbackFunc(ctx, orderID, token, resCode, additionalParams)
	}
	return "", nil
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	ctx := context.Background()

	t.Run("successful order creation - psc asset", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				if userID != 1 || amount != 10 || asset != "psc" {
					t.Errorf("unexpected parameters: userID=%d, amount=%d, asset=%s", userID, amount, asset)
				}
				return "https://sadad.shaparak.ir/Purchase?Token=abc123", nil
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		resp, err := h.CreateOrder(ctx, req)
		if err != nil {
			t.Fatalf("CreateOrder failed: %v", err)
		}

		if resp.Link != "https://sadad.shaparak.ir/Purchase?Token=abc123" {
			t.Errorf("expected link %s, got %s", "https://sadad.shaparak.ir/Purchase?Token=abc123", resp.Link)
		}
	})

	t.Run("successful order creation - all asset types", func(t *testing.T) {
		assetTypes := []string{"psc", "irr", "red", "blue", "yellow"}

		for _, asset := range assetTypes {
			t.Run("asset_"+asset, func(t *testing.T) {
				mockService := &mockOrderService{
					createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
						return "https://sadad.shaparak.ir/Purchase?Token=test", nil
					},
				}
				h := handler.NewOrderHandler(mockService)

				req := &pb.CreateOrderRequest{
					UserId: 1,
					Amount: 5,
					Asset:  asset,
				}

				resp, err := h.CreateOrder(ctx, req)
				if err != nil {
					t.Fatalf("CreateOrder failed for asset %s: %v", asset, err)
				}

				if resp.Link == "" {
					t.Errorf("expected non-empty link for asset %s", asset)
				}
			})
		}
	})

	t.Run("validation error - amount less than minimum", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 0, // Invalid: must be at least 1
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected validation error for amount < 1")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("validation error - negative amount", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: -5, // Invalid: must be at least 1
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected validation error for negative amount")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("validation error - missing asset", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "", // Invalid: required field
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected validation error for empty asset")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("validation error - invalid asset enum value", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "invalid_asset", // Invalid: not one of allowed values
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected validation error for invalid asset")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error - invalid amount", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				return "", service.ErrInvalidAmount
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error - invalid asset", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				return "", service.ErrInvalidAsset
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error - user not eligible", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				return "", service.ErrUserNotEligible
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("service error - payment failed with detail", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				return "", fmt.Errorf("%w: gateway rejected", service.ErrPaymentFailed)
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error - payment failed", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				return "", service.ErrPaymentFailed
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error - internal error", func(t *testing.T) {
		mockService := &mockOrderService{
			createOrderFunc: func(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
				return "", errors.New("database connection failed")
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 10,
			Asset:  "psc",
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("validation error - multiple fields", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.CreateOrderRequest{
			UserId: 1,
			Amount: 0,  // Invalid
			Asset:  "", // Invalid
		}

		_, err := h.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("expected validation error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})
}

func TestOrderHandler_HandleCallback(t *testing.T) {
	ctx := context.Background()

	t.Run("successful callback - res_code 0 (payment success)", func(t *testing.T) {
		expectedURL := "https://rgb.irpsc.com/metaverse/payment/verify?OrderId=123&ResCode=0&Token=456789"
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				if orderID != 123 {
					t.Errorf("unexpected orderID: %d", orderID)
				}
				if resCode != "0" {
					t.Errorf("unexpected resCode: %s", resCode)
				}
				if token != "456789" {
					t.Errorf("unexpected token: %s", token)
				}
				if additionalParams["PrimaryAccNo"] != "1234****5678" {
					t.Errorf("unexpected PrimaryAccNo: %s", additionalParams["PrimaryAccNo"])
				}
				return expectedURL, nil
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 123,
			ResCode: "0",
			Token:   "456789",
			AdditionalParams: map[string]string{
				"PrimaryAccNo": "1234****5678",
			},
		}

		resp, err := h.HandleCallback(ctx, req)
		if err != nil {
			t.Fatalf("HandleCallback failed: %v", err)
		}

		if resp.RedirectUrl != expectedURL {
			t.Errorf("expected redirect URL %s, got %s", expectedURL, resp.RedirectUrl)
		}
	})

	t.Run("successful callback - res_code != 0 (payment failure)", func(t *testing.T) {
		expectedURL := "https://rgb.irpsc.com/metaverse/payment/verify?OrderId=123&ResCode=-1&Token=456789"
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				if resCode != "-1" {
					t.Errorf("unexpected resCode: %s", resCode)
				}
				return expectedURL, nil
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 123,
			ResCode: "-1",
			Token:   "456789",
		}

		resp, err := h.HandleCallback(ctx, req)
		if err != nil {
			t.Fatalf("HandleCallback failed: %v", err)
		}

		if resp.RedirectUrl != expectedURL {
			t.Errorf("expected redirect URL %s, got %s", expectedURL, resp.RedirectUrl)
		}
	})

	t.Run("validation error - missing order_id", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 0,
			ResCode: "0",
			Token:   "456789",
		}

		_, err := h.HandleCallback(ctx, req)
		if err == nil {
			t.Fatal("expected validation error for missing order_id")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("allows empty token for failed payment callbacks", func(t *testing.T) {
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				if token != "" {
					t.Errorf("expected empty token, got %s", token)
				}
				return "https://rgb.irpsc.com/metaverse/payment/verify?OrderId=123&ResCode=1", nil
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 123,
			ResCode: "1",
			Token:   "",
		}

		resp, err := h.HandleCallback(ctx, req)
		if err != nil {
			t.Fatalf("HandleCallback failed: %v", err)
		}
		if resp.RedirectUrl == "" {
			t.Error("expected redirect URL")
		}
	})

	t.Run("validation error - missing order_id with empty token", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 0,
			ResCode: "0",
			Token:   "",
		}

		_, err := h.HandleCallback(ctx, req)
		if err == nil {
			t.Fatal("expected validation error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error - order not found", func(t *testing.T) {
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				return "", service.ErrOrderNotFound
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 999,
			ResCode: "0",
			Token:   "456789",
		}

		_, err := h.HandleCallback(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("expected NotFound, got %v", err)
		}
	})

	t.Run("service error - internal error", func(t *testing.T) {
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				return "", errors.New("database connection failed")
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 123,
			ResCode: "0",
			Token:   "456789",
		}

		_, err := h.HandleCallback(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Internal {
			t.Errorf("expected Internal, got %v", err)
		}
	})

	t.Run("additional params handling - empty map", func(t *testing.T) {
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				if additionalParams == nil {
					t.Error("expected non-nil additionalParams map")
				}
				return "https://rgb.irpsc.com/metaverse/payment/verify?OrderId=123&ResCode=0&Token=456789", nil
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 123,
			ResCode: "0",
			Token:   "456789",
		}

		resp, err := h.HandleCallback(ctx, req)
		if err != nil {
			t.Fatalf("HandleCallback failed: %v", err)
		}

		if resp.RedirectUrl == "" {
			t.Error("expected non-empty redirect URL")
		}
	})

	t.Run("additional params handling - multiple params", func(t *testing.T) {
		mockService := &mockOrderService{
			callbackFunc: func(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
				expectedParams := map[string]string{
					"PrimaryAccNo": "1234****5678",
					"CustomField":  "custom_value",
				}
				for k, v := range expectedParams {
					if additionalParams[k] != v {
						t.Errorf("unexpected value for %s: expected %s, got %s", k, v, additionalParams[k])
					}
				}
				return "https://rgb.irpsc.com/metaverse/payment/verify?OrderId=123&ResCode=0&Token=456789", nil
			},
		}
		h := handler.NewOrderHandler(mockService)

		req := &pb.HandleCallbackRequest{
			OrderId: 123,
			ResCode: "0",
			Token:   "456789",
			AdditionalParams: map[string]string{
				"PrimaryAccNo": "1234****5678",
				"CustomField":  "custom_value",
			},
		}

		_, err := h.HandleCallback(ctx, req)
		if err != nil {
			t.Fatalf("HandleCallback failed: %v", err)
		}
	})

	t.Run("callback with various res codes", func(t *testing.T) {
		resCodes := []string{"0", "-1", "-2", "1", "2", "100"}

		for _, resCode := range resCodes {
			t.Run("res_code_"+resCode, func(t *testing.T) {
				mockService := &mockOrderService{
					callbackFunc: func(ctx context.Context, orderID uint64, token string, gotResCode string, additionalParams map[string]string) (string, error) {
						if gotResCode != resCode {
							t.Errorf("unexpected resCode: expected %s, got %s", resCode, gotResCode)
						}
						return "https://rgb.irpsc.com/metaverse/payment/verify", nil
					},
				}
				h := handler.NewOrderHandler(mockService)

				req := &pb.HandleCallbackRequest{
					OrderId: 123,
					ResCode: resCode,
					Token:   "456789",
				}

				resp, err := h.HandleCallback(ctx, req)
				if err != nil {
					t.Fatalf("HandleCallback failed for resCode %s: %v", resCode, err)
				}

				if resp.RedirectUrl == "" {
					t.Errorf("expected non-empty redirect URL for resCode %s", resCode)
				}
			})
		}
	})
}

func TestOrderHandler_RegisterOrderHandler(t *testing.T) {
	// This test verifies that the handler can be registered with a gRPC server
	// We can't easily test the actual registration without a real gRPC server,
	// but we can verify the function exists and doesn't panic
	t.Run("handler registration", func(t *testing.T) {
		mockService := &mockOrderService{}
		h := handler.NewOrderHandler(mockService)

		if h == nil {
			t.Fatal("expected non-nil handler")
		}
	})
}
