package service

import (
	"context"
	"fmt"
	"math"

	"github.com/shopspring/decimal"

	"metarang/commercial-service/internal/repository"
)

type WalletService interface {
	GetWallet(ctx context.Context, userID uint64) (map[string]string, error)
	CreateWallet(ctx context.Context, userID uint64) (map[string]string, error)
	DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) (map[string]string, error)
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) (map[string]string, error)
	LockBalance(ctx context.Context, userID uint64, asset string, amount float64, reason string) error
	UnlockBalance(ctx context.Context, userID uint64, asset string, amount float64) error
}

type walletService struct {
	walletRepo repository.WalletRepository
}

func NewWalletService(walletRepo repository.WalletRepository) WalletService {
	return &walletService{
		walletRepo: walletRepo,
	}
}

func (s *walletService) GetWallet(ctx context.Context, userID uint64) (map[string]string, error) {
	wallet, err := s.walletRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}
	if wallet == nil {
		return nil, fmt.Errorf("wallet not found")
	}

	// Return raw numeric values without formatting (no K, M suffixes)
	return map[string]string{
		"psc":          wallet.PSC.String(),
		"irr":          wallet.IRR.String(),
		"red":          wallet.Red.String(),
		"blue":         wallet.Blue.String(),
		"yellow":       wallet.Yellow.String(),
		"satisfaction": wallet.Satisfaction.String(),
		"effect":       wallet.Effect.String(),
	}, nil
}

func (s *walletService) CreateWallet(ctx context.Context, userID uint64) (map[string]string, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user_id is required")
	}

	wallet, err := s.walletRepo.Create(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	return map[string]string{
		"psc":          wallet.PSC.String(),
		"irr":          wallet.IRR.String(),
		"red":          wallet.Red.String(),
		"blue":         wallet.Blue.String(),
		"yellow":       wallet.Yellow.String(),
		"satisfaction": wallet.Satisfaction.String(),
		"effect":       wallet.Effect.String(),
	}, nil
}

func (s *walletService) DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) (map[string]string, error) {
	amountDec, err := decimalFromAmount(amount)
	if err != nil {
		return nil, err
	}

	if err = s.walletRepo.DeductBalance(ctx, userID, asset, amountDec); err != nil {
		return nil, fmt.Errorf("failed to deduct balance: %w", err)
	}

	return s.GetWallet(ctx, userID)
}

func (s *walletService) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) (map[string]string, error) {
	amountDec, err := decimalFromAmount(amount)
	if err != nil {
		return nil, err
	}

	err = s.walletRepo.AddBalance(ctx, userID, asset, amountDec)
	if err != nil {
		return nil, fmt.Errorf("failed to add balance: %w", err)
	}

	// The credit is already committed. A follow-up read failure must not look
	// like a failed credit, or callers will refund money that the owner kept.
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return map[string]string{asset: amountDec.StringFixed(2)}, nil
	}
	return wallet, nil
}

func decimalFromAmount(amount float64) (decimal.Decimal, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 {
		return decimal.Zero, fmt.Errorf("invalid amount")
	}
	return decimal.NewFromFloat(amount).Round(2), nil
}

func (s *walletService) LockBalance(ctx context.Context, userID uint64, asset string, amount float64, reason string) error {
	amountDec := decimal.NewFromFloat(amount)

	err := s.walletRepo.LockBalance(ctx, userID, asset, amountDec, reason)
	if err != nil {
		return fmt.Errorf("failed to lock balance: %w", err)
	}

	return nil
}

func (s *walletService) UnlockBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	amountDec := decimal.NewFromFloat(amount)

	err := s.walletRepo.UnlockBalance(ctx, userID, asset, amountDec)
	if err != nil {
		return fmt.Errorf("failed to unlock balance: %w", err)
	}

	return nil
}
