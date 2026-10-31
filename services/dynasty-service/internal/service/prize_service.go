package service

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
)

// WalletPort credits wallet balances via commercial-service (AddBalance gRPC).
type WalletPort interface {
	IncrementWalletPSC(ctx context.Context, userID uint64, amount float64) error
	IncrementSatisfaction(ctx context.Context, userID uint64, amount float64) error
}

type PrizeService struct {
	prizeRepo   *repository.PrizeRepository
	rates       AssetRatePort
	userVarRepo *repository.UserVariableRepository
	wallet      WalletPort
	db          *sql.DB
}

func NewPrizeService(
	db *sql.DB,
	prizeRepo *repository.PrizeRepository,
	rates AssetRatePort,
	userVarRepo *repository.UserVariableRepository,
	wallet WalletPort,
) *PrizeService {
	return &PrizeService{
		prizeRepo:   prizeRepo,
		rates:       rates,
		userVarRepo: userVarRepo,
		wallet:      wallet,
		db:          db,
	}
}

// GetAllPrizes retrieves all dynasty prizes
func (s *PrizeService) GetAllPrizes(ctx context.Context, page, perPage int32) ([]*models.DynastyPrize, int32, error) {
	return s.prizeRepo.GetAllPrizes(ctx, page, perPage)
}

// GetPrize retrieves a specific prize
func (s *PrizeService) GetPrize(ctx context.Context, prizeID uint64) (*models.DynastyPrize, error) {
	prize, err := s.prizeRepo.GetPrizeByID(ctx, prizeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get prize: %w", err)
	}
	if prize == nil {
		return nil, fmt.Errorf("prize not found")
	}
	return prize, nil
}

// ClaimPrize redeems a received_dynasty_prize row: wallet credit (PSC rate), satisfaction,
func (s *PrizeService) ClaimPrize(ctx context.Context, receivedPrizeID, userID uint64) error {
	if s.wallet == nil {
		return fmt.Errorf("wallet client not configured")
	}
	if s.rates == nil || s.userVarRepo == nil || s.db == nil {
		return fmt.Errorf("prize claim dependencies not configured")
	}

	receivedPrize, err := s.prizeRepo.GetReceivedPrize(ctx, receivedPrizeID)
	if err != nil {
		return fmt.Errorf("failed to get received prize: %w", err)
	}
	if receivedPrize == nil {
		return fmt.Errorf("prize not found")
	}

	if receivedPrize.UserID != userID {
		return fmt.Errorf("unauthorized: prize does not belong to user")
	}

	prize := receivedPrize.Prize
	if prize == nil {
		return fmt.Errorf("prize definition not found")
	}

	rate, err := PSCRate(ctx, s.rates)
	if err != nil {
		return err
	}

	pscAmount, err := RialsToPSC(int64(prize.PSC), rate)
	if err != nil {
		return err
	}
	if err := s.wallet.IncrementWalletPSC(ctx, userID, pscAmount); err != nil {
		return fmt.Errorf("wallet psc credit: %w", err)
	}
	if err := s.wallet.IncrementSatisfaction(ctx, userID, prize.Satisfaction); err != nil {
		return fmt.Errorf("wallet satisfaction credit: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.userVarRepo.ApplyDynastyPrizeMultipliers(ctx, tx, userID,
		prize.IntroductionProfitIncrease,
		prize.DataStorage,
		prize.AccumulatedCapitalReserve,
	); err != nil {
		return err
	}

	if err := s.prizeRepo.DeleteReceivedPrizeTx(ctx, tx, receivedPrizeID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit prize claim: %w", err)
	}

	return nil
}

// GetUserReceivedPrizes retrieves all received prizes for a user
func (s *PrizeService) GetUserReceivedPrizes(ctx context.Context, userID uint64, page, perPage int32) ([]*models.ReceivedPrize, int32, error) {
	// Get all prizes for user
	prizes, err := s.prizeRepo.GetUserReceivedPrizes(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user prizes: %w", err)
	}

	total := int32(len(prizes))

	// Simple pagination
	offset := (page - 1) * perPage
	if offset >= total {
		return []*models.ReceivedPrize{}, total, nil
	}

	end := offset + perPage
	if end > total {
		end = total
	}

	return prizes[offset:end], total, nil
}

// GetReceivedPrize retrieves a received prize by ID
func (s *PrizeService) GetReceivedPrize(ctx context.Context, receivedPrizeID uint64) (*models.ReceivedPrize, error) {
	return s.prizeRepo.GetReceivedPrize(ctx, receivedPrizeID)
}
