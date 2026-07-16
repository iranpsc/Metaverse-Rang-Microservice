package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"metarang/features-service/internal/handler"
	"metarang/features-service/internal/models"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/logger"
)

// mockProfitService implements profit service methods for testing
type mockProfitService struct {
	getHourlyProfitsFunc        func(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, string, string, string, bool, error)
	getSingleProfitFunc         func(ctx context.Context, profitID, userID uint64) (*models.FeatureHourlyProfit, error)
	getProfitsByApplicationFunc func(ctx context.Context, userID uint64, karbari string) (float64, error)
}

func (m *mockProfitService) GetHourlyProfits(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, string, string, string, bool, error) {
	if m.getHourlyProfitsFunc != nil {
		return m.getHourlyProfitsFunc(ctx, userID, page, pageSize)
	}
	return nil, "0.00", "0.00", "0.00", false, errors.New("not implemented")
}

func (m *mockProfitService) GetSingleProfit(ctx context.Context, profitID, userID uint64) (*models.FeatureHourlyProfit, error) {
	if m.getSingleProfitFunc != nil {
		return m.getSingleProfitFunc(ctx, profitID, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProfitService) GetProfitsByApplication(ctx context.Context, userID uint64, karbari string) (float64, error) {
	if m.getProfitsByApplicationFunc != nil {
		return m.getProfitsByApplicationFunc(ctx, userID, karbari)
	}
	return 0, errors.New("not implemented")
}

func (m *mockProfitService) StartHourlyProfitCalculator(ctx context.Context, log *logger.Logger) {
	// Mock implementation - no-op for testing
}

func (m *mockProfitService) TransferProfitOnSale(ctx context.Context, featureID, sellerID, buyerID uint64, withdrawProfitDays int) error {
	return errors.New("not implemented")
}

func (m *mockProfitService) RunHourlyProfitCalculation(ctx context.Context) (int, error) {
	return 0, nil
}

func (m *mockProfitService) GetHourlyProfitTimePercentage(ctx context.Context, userID uint64) (float64, error) {
	return 0, nil
}

func TestProfitHandler_GetHourlyProfits(t *testing.T) {
	ctx := context.Background()

	t.Run("successful request with default pagination", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getHourlyProfitsFunc = func(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, string, string, string, bool, error) {
			if userID != 1 {
				t.Errorf("Expected userID 1, got %d", userID)
			}
			if page != 1 {
				t.Errorf("Expected page 1 (default), got %d", page)
			}
			if pageSize != 10 {
				t.Errorf("Expected pageSize 10 (default), got %d", pageSize)
			}
			profits := []*models.FeatureHourlyProfit{
				{
					ID:           1,
					UserID:       1,
					FeatureID:    100,
					Asset:        "yellow",
					Amount:       123.456,
					Deadline:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
					IsActive:     true,
					Karbari:      "m",
					PropertiesID: "prop-123",
					FeatureDBID:  100,
				},
			}
			return profits, "100.50", "200.75", "300.25", false, nil
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetHourlyProfitsRequest{
			UserId:   1,
			Page:     0,
			PageSize: 0,
		}

		resp, err := handler.GetHourlyProfits(ctx, req)
		if err != nil {
			t.Fatalf("GetHourlyProfits failed: %v", err)
		}

		if resp == nil {
			t.Fatal("Expected response, got nil")
		}

		if resp.TotalMaskoniProfit != "100.50" {
			t.Errorf("Expected TotalMaskoniProfit 100.50, got %s", resp.TotalMaskoniProfit)
		}
		if resp.TotalTejariProfit != "200.75" {
			t.Errorf("Expected TotalTejariProfit 200.75, got %s", resp.TotalTejariProfit)
		}
		if resp.TotalAmozeshiProfit != "300.25" {
			t.Errorf("Expected TotalAmozeshiProfit 300.25, got %s", resp.TotalAmozeshiProfit)
		}

		if len(resp.Profits) != 1 {
			t.Fatalf("Expected 1 profit, got %d", len(resp.Profits))
		}

		// Check formatting: amount should have 3 decimals
		if resp.Profits[0].Amount != "123.456" {
			t.Errorf("Expected amount 123.456, got %s", resp.Profits[0].Amount)
		}

		// Check Jalali date format (Y/m/d)
		if resp.Profits[0].DeadLine == "" {
			t.Error("Expected deadline to be set")
		}
		if len(resp.Profits[0].DeadLine) < 8 { // At least YYYY/MM/DD
			t.Errorf("Expected Jalali date format, got %s", resp.Profits[0].DeadLine)
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		mockService := &mockProfitService{}
		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetHourlyProfitsRequest{
			UserId:   0,
			Page:     1,
			PageSize: 10,
		}

		resp, err := handler.GetHourlyProfits(ctx, req)
		if err == nil {
			t.Error("Expected error for missing user_id")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getHourlyProfitsFunc = func(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, string, string, string, bool, error) {
			return nil, "0.00", "0.00", "0.00", false, errors.New("service error")
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetHourlyProfitsRequest{
			UserId:   1,
			Page:     1,
			PageSize: 10,
		}

		resp, err := handler.GetHourlyProfits(ctx, req)
		if err == nil {
			t.Error("Expected error from service")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})
}

func TestProfitHandler_GetSingleProfit(t *testing.T) {
	ctx := context.Background()

	t.Run("successful withdrawal", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getSingleProfitFunc = func(ctx context.Context, profitID, userID uint64) (*models.FeatureHourlyProfit, error) {
			if profitID != 1 || userID != 1 {
				t.Errorf("Expected profitID 1, userID 1, got %d, %d", profitID, userID)
			}
			return &models.FeatureHourlyProfit{
				ID:           1,
				UserID:       1,
				FeatureID:    100,
				Asset:        "yellow",
				Amount:       50.123,
				Deadline:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				IsActive:     true,
				Karbari:      "m",
				PropertiesID: "prop-123",
				FeatureDBID:  100,
			}, nil
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetSingleProfitRequest{
			ProfitId: 1,
			UserId:   1,
		}

		resp, err := handler.GetSingleProfit(ctx, req)
		if err != nil {
			t.Fatalf("GetSingleProfit failed: %v", err)
		}

		if resp == nil {
			t.Fatal("Expected response, got nil")
		}

		if !resp.Success {
			t.Error("Expected success to be true")
		}

		if resp.Profit == nil {
			t.Fatal("Expected profit to be set")
		}

		if resp.Profit.Id != 1 {
			t.Errorf("Expected profit ID 1, got %d", resp.Profit.Id)
		}

		// Check formatting: amount should have 3 decimals
		if resp.Profit.Amount != "50.123" {
			t.Errorf("Expected amount 50.123, got %s", resp.Profit.Amount)
		}

		// Check Jalali date format
		if resp.Profit.DeadLine == "" {
			t.Error("Expected deadline to be set")
		}
	})

	t.Run("missing profit_id", func(t *testing.T) {
		mockService := &mockProfitService{}
		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetSingleProfitRequest{
			ProfitId: 0,
			UserId:   1,
		}

		resp, err := handler.GetSingleProfit(ctx, req)
		if err == nil {
			t.Error("Expected error for missing profit_id")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		mockService := &mockProfitService{}
		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetSingleProfitRequest{
			ProfitId: 1,
			UserId:   0,
		}

		resp, err := handler.GetSingleProfit(ctx, req)
		if err == nil {
			t.Error("Expected error for missing user_id")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("unauthorized access", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getSingleProfitFunc = func(ctx context.Context, profitID, userID uint64) (*models.FeatureHourlyProfit, error) {
			return nil, errors.New("unauthorized")
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetSingleProfitRequest{
			ProfitId: 1,
			UserId:   1,
		}

		resp, err := handler.GetSingleProfit(ctx, req)
		if err == nil {
			t.Error("Expected error for unauthorized access")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})
}

func TestProfitHandler_GetProfitsByApplication(t *testing.T) {
	ctx := context.Background()

	t.Run("successful withdrawal for maskoni", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getProfitsByApplicationFunc = func(ctx context.Context, userID uint64, karbari string) (float64, error) {
			if userID != 1 {
				t.Errorf("Expected userID 1, got %d", userID)
			}
			if karbari != "m" {
				t.Errorf("Expected karbari m, got %s", karbari)
			}
			return 150.75, nil
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  1,
			Karbari: "m",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err != nil {
			t.Fatalf("GetProfitsByApplication failed: %v", err)
		}

		if resp == nil {
			t.Fatal("Expected response, got nil")
		}

		if !resp.Success {
			t.Error("Expected success to be true")
		}
	})

	t.Run("successful withdrawal for tejari", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getProfitsByApplicationFunc = func(ctx context.Context, userID uint64, karbari string) (float64, error) {
			return 200.50, nil
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  1,
			Karbari: "t",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err != nil {
			t.Fatalf("GetProfitsByApplication failed: %v", err)
		}

		if !resp.Success {
			t.Error("Expected success to be true")
		}
	})

	t.Run("successful withdrawal for amozeshi", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getProfitsByApplicationFunc = func(ctx context.Context, userID uint64, karbari string) (float64, error) {
			return 300.25, nil
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  1,
			Karbari: "a",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err != nil {
			t.Fatalf("GetProfitsByApplication failed: %v", err)
		}

		if !resp.Success {
			t.Error("Expected success to be true")
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		mockService := &mockProfitService{}
		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  0,
			Karbari: "m",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err == nil {
			t.Error("Expected error for missing user_id")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("missing karbari", func(t *testing.T) {
		mockService := &mockProfitService{}
		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  1,
			Karbari: "",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err == nil {
			t.Error("Expected error for missing karbari")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("invalid karbari value", func(t *testing.T) {
		mockService := &mockProfitService{}
		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  1,
			Karbari: "x",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err == nil {
			t.Error("Expected error for invalid karbari")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &mockProfitService{}
		mockService.getProfitsByApplicationFunc = func(ctx context.Context, userID uint64, karbari string) (float64, error) {
			return 0, errors.New("service error")
		}

		handler := handler.NewProfitHandler(mockService)

		req := &pb.GetProfitsByApplicationRequest{
			UserId:  1,
			Karbari: "m",
		}

		resp, err := handler.GetProfitsByApplication(ctx, req)
		if err == nil {
			t.Error("Expected error from service")
		}
		if resp != nil {
			t.Error("Expected nil response on error")
		}
	})
}
