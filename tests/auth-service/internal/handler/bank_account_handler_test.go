package handler_test

import (
	"context"
	"database/sql"
	"errors"
	"metarang/auth-service/internal/handler"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

// Test data constants
const (
	testShebaNum = "6201600000000000080068121" // 24 digits (sample format)
	testCardNum  = "6037997551234567"
	testBankName = "Tejarat"
)

// mockKYCService is a mock implementation for testing bank account handlers
type mockKYCService struct {
	listBankAccountsFunc  func(ctx context.Context, userID uint64) ([]*models.BankAccount, error)
	createBankAccountFunc func(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error)
	getBankAccountFunc    func(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error)
	updateBankAccountFunc func(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error)
	deleteBankAccountFunc func(ctx context.Context, userID uint64, bankAccountID uint64) error
}

func (m *mockKYCService) ListBankAccounts(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
	if m.listBankAccountsFunc != nil {
		return m.listBankAccountsFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockKYCService) CreateBankAccount(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
	if m.createBankAccountFunc != nil {
		return m.createBankAccountFunc(ctx, userID, bankName, shabaNum, cardNum)
	}
	return nil, errors.New("not implemented")
}

func (m *mockKYCService) GetBankAccount(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error) {
	if m.getBankAccountFunc != nil {
		return m.getBankAccountFunc(ctx, userID, bankAccountID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockKYCService) UpdateBankAccount(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
	if m.updateBankAccountFunc != nil {
		return m.updateBankAccountFunc(ctx, userID, bankAccountID, bankName, shabaNum, cardNum)
	}
	return nil, errors.New("not implemented")
}

func (m *mockKYCService) DeleteBankAccount(ctx context.Context, userID uint64, bankAccountID uint64) error {
	if m.deleteBankAccountFunc != nil {
		return m.deleteBankAccountFunc(ctx, userID, bankAccountID)
	}
	return errors.New("not implemented")
}

// Implement other KYCService methods as no-ops for compilation
func (m *mockKYCService) GetKYC(ctx context.Context, userID uint64) (*models.KYC, error) {
	return nil, errors.New("not implemented")
}

func (m *mockKYCService) UpdateKYC(ctx context.Context, userID uint64, fname, lname, melliCode, birthdate, province, melliCard, videoPath, videoName string, verifyTextID uint64, gender string) (*models.KYC, error) {
	return nil, errors.New("not implemented")
}

func TestKYCHandler_ListBankAccounts(t *testing.T) {
	ctx := context.Background()

	t.Run("successful list", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.listBankAccountsFunc = func(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
			return []*models.BankAccount{
				{
					ID:           1,
					BankableType: "App\\Models\\User",
					BankableID:   userID,
					BankName:     testBankName,
					ShabaNum:     testShebaNum,
					CardNum:      testCardNum,
					Status:       0,
					Errors:       sql.NullString{},
				},
			}, nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.ListBankAccountsRequest{
			UserId: 1,
		}

		resp, err := h.ListBankAccounts(ctx, req)
		if err != nil {
			t.Fatalf("ListBankAccounts failed: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("expected 1 account, got %d", len(resp.Data))
		}
		if resp.Data[0].Id != 1 {
			t.Errorf("expected ID 1, got %d", resp.Data[0].Id)
		}
		if resp.Data[0].BankName != testBankName {
			t.Errorf("expected BankName %q, got %q", testBankName, resp.Data[0].BankName)
		}
	})

	t.Run("returns empty list when no accounts", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.listBankAccountsFunc = func(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
			return []*models.BankAccount{}, nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.ListBankAccountsRequest{
			UserId: 1,
		}

		resp, err := h.ListBankAccounts(ctx, req)
		if err != nil {
			t.Fatalf("ListBankAccounts failed: %v", err)
		}

		if len(resp.Data) != 0 {
			t.Errorf("expected empty list, got %d accounts", len(resp.Data))
		}
	})

	t.Run("handles service error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.listBankAccountsFunc = func(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
			return nil, errors.New("database error")
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.ListBankAccountsRequest{
			UserId: 1,
		}

		_, err := h.ListBankAccounts(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.Internal {
			t.Errorf("expected Internal error code, got %v", st.Code())
		}
	})
}

func TestKYCHandler_CreateBankAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("successful creation", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.createBankAccountFunc = func(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return &models.BankAccount{
				ID:           1,
				BankableType: "App\\Models\\User",
				BankableID:   userID,
				BankName:     bankName,
				ShabaNum:     shabaNum,
				CardNum:      cardNum,
				Status:       0,
				Errors:       sql.NullString{},
			}, nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.CreateBankAccountRequest{
			UserId:   1,
			BankName: testBankName,
			ShabaNum: testShebaNum,
			CardNum:  testCardNum,
		}

		resp, err := h.CreateBankAccount(ctx, req)
		if err != nil {
			t.Fatalf("CreateBankAccount failed: %v", err)
		}

		if resp.Id != 1 {
			t.Errorf("expected ID 1, got %d", resp.Id)
		}
		if resp.BankName != testBankName {
			t.Errorf("expected BankName %q, got %q", testBankName, resp.BankName)
		}
		if resp.ShabaNum != testShebaNum {
			t.Errorf("expected ShabaNum %q, got %q", testShebaNum, resp.ShabaNum)
		}
		if resp.CardNum != testCardNum {
			t.Errorf("expected CardNum %q, got %q", testCardNum, resp.CardNum)
		}
		if resp.Status != 0 {
			t.Errorf("expected Status 0, got %d", resp.Status)
		}
	})

	t.Run("handles user not verified error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.createBankAccountFunc = func(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return nil, service.ErrUserNotVerified
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.CreateBankAccountRequest{
			UserId:   1,
			BankName: testBankName,
			ShabaNum: testShebaNum,
			CardNum:  testCardNum,
		}

		_, err := h.CreateBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied error code, got %v", st.Code())
		}
	})

	t.Run("handles validation error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.createBankAccountFunc = func(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return nil, service.ErrInvalidBankName
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.CreateBankAccountRequest{
			UserId:   1,
			BankName: "A", // Too short
			ShabaNum: testShebaNum,
			CardNum:  testCardNum,
		}

		_, err := h.CreateBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument error code, got %v", st.Code())
		}
	})

	t.Run("handles duplicate sheba error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.createBankAccountFunc = func(ctx context.Context, userID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return nil, service.ErrShabaNumNotUnique
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.CreateBankAccountRequest{
			UserId:   1,
			BankName: testBankName,
			ShabaNum: testShebaNum,
			CardNum:  testCardNum,
		}

		_, err := h.CreateBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument error code, got %v", st.Code())
		}
	})
}

func TestKYCHandler_GetBankAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.getBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error) {
			return &models.BankAccount{
				ID:           bankAccountID,
				BankableType: "App\\Models\\User",
				BankableID:   userID,
				BankName:     testBankName,
				ShabaNum:     testShebaNum,
				CardNum:      testCardNum,
				Status:       1,
				Errors:       sql.NullString{},
			}, nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.GetBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
		}

		resp, err := h.GetBankAccount(ctx, req)
		if err != nil {
			t.Fatalf("GetBankAccount failed: %v", err)
		}

		if resp.Id != 1 {
			t.Errorf("expected ID 1, got %d", resp.Id)
		}
		if resp.BankName != testBankName {
			t.Errorf("expected BankName %q, got %q", testBankName, resp.BankName)
		}
		if resp.Status != 1 {
			t.Errorf("expected Status 1, got %d", resp.Status)
		}
	})

	t.Run("handles not found error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.getBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error) {
			return nil, service.ErrBankAccountNotFound
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.GetBankAccountRequest{
			UserId:        1,
			BankAccountId: 999,
		}

		_, err := h.GetBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("handles not owned error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.getBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error) {
			return nil, service.ErrBankAccountNotOwned
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.GetBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
		}

		_, err := h.GetBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied error code, got %v", st.Code())
		}
	})

	t.Run("includes errors field when present", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.getBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) (*models.BankAccount, error) {
			return &models.BankAccount{
				ID:           bankAccountID,
				BankableType: "App\\Models\\User",
				BankableID:   userID,
				BankName:     testBankName,
				ShabaNum:     testShebaNum,
				CardNum:      testCardNum,
				Status:       -1,
				Errors:       sql.NullString{String: "Validation failed", Valid: true},
			}, nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.GetBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
		}

		resp, err := h.GetBankAccount(ctx, req)
		if err != nil {
			t.Fatalf("GetBankAccount failed: %v", err)
		}

		if resp.Errors != "Validation failed" {
			t.Errorf("expected Errors 'Validation failed', got %q", resp.Errors)
		}
	})
}

func TestKYCHandler_UpdateBankAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("successful update", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.updateBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return &models.BankAccount{
				ID:           bankAccountID,
				BankableType: "App\\Models\\User",
				BankableID:   userID,
				BankName:     bankName,
				ShabaNum:     shabaNum,
				CardNum:      cardNum,
				Status:       0,                // Reset to pending
				Errors:       sql.NullString{}, // Cleared
			}, nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.UpdateBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
			BankName:      "Melli",
			ShabaNum:      "820540102680020817909003",
			CardNum:       "6037997551234568",
		}

		resp, err := h.UpdateBankAccount(ctx, req)
		if err != nil {
			t.Fatalf("UpdateBankAccount failed: %v", err)
		}

		if resp.Id != 1 {
			t.Errorf("expected ID 1, got %d", resp.Id)
		}
		if resp.BankName != "Melli" {
			t.Errorf("expected BankName 'Melli', got %q", resp.BankName)
		}
		if resp.Status != 0 {
			t.Errorf("expected Status 0 (pending), got %d", resp.Status)
		}
	})

	t.Run("handles not found error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.updateBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return nil, service.ErrBankAccountNotFound
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.UpdateBankAccountRequest{
			UserId:        1,
			BankAccountId: 999,
			BankName:      testBankName,
			ShabaNum:      testShebaNum,
			CardNum:       testCardNum,
		}

		_, err := h.UpdateBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("handles not rejected error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.updateBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64, bankName, shabaNum, cardNum string) (*models.BankAccount, error) {
			return nil, service.ErrBankAccountNotRejected
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.UpdateBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
			BankName:      testBankName,
			ShabaNum:      testShebaNum,
			CardNum:       testCardNum,
		}

		_, err := h.UpdateBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied error code, got %v", st.Code())
		}
	})
}

func TestKYCHandler_DeleteBankAccount(t *testing.T) {
	ctx := context.Background()

	t.Run("successful deletion", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.deleteBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) error {
			return nil
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.DeleteBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
		}

		_, err := h.DeleteBankAccount(ctx, req)
		if err != nil {
			t.Fatalf("DeleteBankAccount failed: %v", err)
		}
	})

	t.Run("handles not found error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.deleteBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) error {
			return service.ErrBankAccountNotFound
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.DeleteBankAccountRequest{
			UserId:        1,
			BankAccountId: 999,
		}

		_, err := h.DeleteBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("expected NotFound error code, got %v", st.Code())
		}
	})

	t.Run("handles not owned error", func(t *testing.T) {
		mockService := &mockKYCService{}
		mockService.deleteBankAccountFunc = func(ctx context.Context, userID uint64, bankAccountID uint64) error {
			return service.ErrBankAccountNotOwned
		}

		h := handler.NewKYCHandler(mockService, nil, "")

		req := &pb.DeleteBankAccountRequest{
			UserId:        1,
			BankAccountId: 1,
		}

		_, err := h.DeleteBankAccount(ctx, req)
		if err == nil {
			t.Fatal("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("expected gRPC status error")
		}
		if st.Code() != codes.PermissionDenied {
			t.Errorf("expected PermissionDenied error code, got %v", st.Code())
		}
	})
}
