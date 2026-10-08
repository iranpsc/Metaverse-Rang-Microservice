package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"metarang/commercial-service/internal/models"
	"metarang/commercial-service/internal/repository"
)

type TransactionService interface {
	ListTransactions(ctx context.Context, userID uint64, filters map[string]interface{}) ([]*models.TransactionDTO, error)
	GetLatestTransaction(ctx context.Context, userID uint64) (*models.Transaction, error)
	CreateTransaction(ctx context.Context, transaction *models.Transaction) error
}

type transactionService struct {
	transactionRepo repository.TransactionRepository
	jalaliConverter JalaliConverter
}

func NewTransactionService(
	transactionRepo repository.TransactionRepository,
	jalaliConverter JalaliConverter,
) TransactionService {
	return &transactionService{
		transactionRepo: transactionRepo,
		jalaliConverter: jalaliConverter,
	}
}

func (s *transactionService) ListTransactions(ctx context.Context, userID uint64, filters map[string]interface{}) ([]*models.TransactionDTO, error) {
	transactions, err := s.transactionRepo.FindByUserID(ctx, userID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list transactions: %w", err)
	}

	// Convert to DTOs with Jalali date formatting
	dtos := make([]*models.TransactionDTO, len(transactions))
	for i, t := range transactions {
		dtos[i] = s.transactionToDTO(t)
	}

	return dtos, nil
}

func (s *transactionService) GetLatestTransaction(ctx context.Context, userID uint64) (*models.Transaction, error) {
	transaction, err := s.transactionRepo.FindLatestByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest transaction: %w", err)
	}
	return transaction, nil
}

// transactionToDTO converts Transaction model to TransactionDTO with proper formatting
func (s *transactionService) transactionToDTO(t *models.Transaction) *models.TransactionDTO {
	payableType := ""
	if t.PayableType != nil {
		payableType = payableTypeToTransactionType(*t.PayableType)
	}

	return &models.TransactionDTO{
		ID:     t.ID,
		Type:   payableType,
		Asset:  t.Asset,
		Amount: strconv.FormatFloat(t.Amount, 'f', -1, 64),
		Action: t.Action,
		Status: t.Status,
		Date:   s.jalaliConverter.FormatJalaliDate(t.CreatedAt),
		Time:   s.jalaliConverter.FormatJalaliTime(t.CreatedAt),
	}
}

func payableTypeToTransactionType(payableType string) string {
	switch payableType {
	case `App\Models\Trade`:
		return "trade"
	case `App\Models\Order`:
		return "order"
	case `App\Models\BuildingEntrySession`:
		return "building_entry"
	default:
		return "unknown"
	}
}

func (s *transactionService) CreateTransaction(ctx context.Context, transaction *models.Transaction) error {
	// Generate transaction ID if not provided
	if transaction.ID == "" {
		transaction.ID = fmt.Sprintf("TR-%d", time.Now().UnixNano())
	}

	err := s.transactionRepo.Create(ctx, transaction)
	if err != nil {
		return fmt.Errorf("failed to create transaction: %w", err)
	}

	return nil
}
