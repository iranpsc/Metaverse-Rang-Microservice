package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/models"
	"metarang/features-service/internal/repository"
	"metarang/shared/pkg/logger"
)

// ProfitServiceInterface defines the interface for profit service operations
type ProfitServiceInterface interface {
	GetSingleProfit(ctx context.Context, profitID, userID uint64) (*models.FeatureHourlyProfit, error)
	GetProfitsByApplication(ctx context.Context, userID uint64, karbari string) (float64, error)
	TransferProfitOnSale(ctx context.Context, featureID, sellerID, buyerID uint64, withdrawProfitDays int) error
	GetHourlyProfits(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, string, string, string, bool, error)
	GetHourlyProfitTimePercentage(ctx context.Context, userID uint64) (float64, error)
	RunHourlyProfitCalculation(ctx context.Context) (int, error)
	StartHourlyProfitCalculator(ctx context.Context, log *logger.Logger)
}

// ProfitService implements profit service with gRPC cross-service calls
type ProfitService struct {
	profitRepo         *repository.HourlyProfitRepository
	featureRepo        *repository.FeatureRepository
	propertiesRepo     *repository.PropertiesRepository
	commercialClient   *client.CommercialClient
	notificationClient *client.NotificationClient
	db                 *sql.DB
	log                *logger.Logger
}

func NewProfitService(
	profitRepo *repository.HourlyProfitRepository,
	featureRepo *repository.FeatureRepository,
	propertiesRepo *repository.PropertiesRepository,
	commercialClient *client.CommercialClient,
	notificationClient *client.NotificationClient,
	db *sql.DB,
	log *logger.Logger,
) ProfitServiceInterface {
	return &ProfitService{
		profitRepo:         profitRepo,
		featureRepo:        featureRepo,
		propertiesRepo:     propertiesRepo,
		commercialClient:   commercialClient,
		notificationClient: notificationClient,
		db:                 db,
		log:                log,
	}
}

// GetSingleProfit withdraws a single profit using gRPC
// Returns the updated profit record with feature information
func (s *ProfitService) GetSingleProfit(ctx context.Context, profitID, userID uint64) (*models.FeatureHourlyProfit, error) {
	// Get profit record with feature properties
	profit, err := s.profitRepo.FindByID(ctx, profitID)
	if err != nil {
		return nil, fmt.Errorf("profit not found: %w", err)
	}

	// Verify ownership
	if profit.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}

	// Add amount to user wallet via gRPC
	if profit.Amount > 0 && s.commercialClient != nil {
		if err := s.commercialClient.AddBalance(ctx, userID, profit.Asset, profit.Amount); err != nil {
			return nil, fmt.Errorf("failed to update wallet: %w", err)
		}

		s.log.Info("Profit withdrawn",
			"profit_id", profitID,
			"user_id", userID,
			"asset", profit.Asset,
			"amount", profit.Amount,
		)

		// Send notification if notification client is available
		if s.notificationClient != nil {
			if err := s.notificationClient.SendFeatureHourlyProfitDeposit(ctx, userID, profit.Asset, profit.Amount, profit.Karbari, profit.PropertiesID); err != nil {
				s.log.Warn("Failed to send notification", "error", err)
			}
		}
	}

	// Get user's withdraw_profit days
	withdrawProfitDays, err := s.getUserVariableWithdrawProfit(ctx, userID)
	if err != nil || withdrawProfitDays == 0 {
		withdrawProfitDays = 10
	}

	// Reset profit and update deadline
	if err := s.profitRepo.ResetProfitAndUpdateDeadline(ctx, profitID, withdrawProfitDays); err != nil {
		return nil, fmt.Errorf("failed to reset profit: %w", err)
	}

	// Re-fetch the updated profit record
	updatedProfit, err := s.profitRepo.FindByID(ctx, profitID)
	if err != nil {
		return profit, nil // Return original if re-fetch fails
	}

	return updatedProfit, nil
}

// GetProfitsByApplication withdraws all profits by karbari using gRPC
// Processes profits in chunks to avoid memory spikes
func (s *ProfitService) GetProfitsByApplication(ctx context.Context, userID uint64, karbari string) (float64, error) {
	// Validate karbari
	if karbari != constants.Maskoni && karbari != constants.Tejari && karbari != constants.Amozeshi {
		return 0, fmt.Errorf("invalid karbari: must be m, t, or a")
	}

	asset := constants.GetColor(karbari)

	// Get user's withdraw_profit days
	withdrawProfitDays, err := s.getUserVariableWithdrawProfit(ctx, userID)
	if err != nil || withdrawProfitDays == 0 {
		withdrawProfitDays = 10
	}

	// Get all profits for this user and karbari (matches Laravel properties.karbari filter)
	profits, err := s.profitRepo.GetAllByUserAndKarbari(ctx, userID, karbari)
	if err != nil {
		return 0, fmt.Errorf("failed to get profits: %w", err)
	}

	// Process profits in chunks of 100 (as per Laravel's chunkById(100))
	chunkSize := 100
	totalAmount := 0.0

	for i := 0; i < len(profits); i += chunkSize {
		end := i + chunkSize
		if end > len(profits) {
			end = len(profits)
		}

		chunk := profits[i:end]
		for _, profit := range chunk {
			if profit.Amount > 0 {
				if s.commercialClient == nil {
					continue
				}
				if err := s.commercialClient.AddBalance(ctx, userID, profit.Asset, profit.Amount); err != nil {
					s.log.Error("Failed to add profit to wallet", "profit_id", profit.ID, "error", err)
					continue
				}
				totalAmount += profit.Amount
			}

			if err := s.profitRepo.ResetProfitAndUpdateDeadline(ctx, profit.ID, withdrawProfitDays); err != nil {
				s.log.Error("Failed to reset profit", "profit_id", profit.ID, "error", err)
			}
		}
	}

	// Log withdrawal
	if totalAmount > 0 {
		s.log.Info("Batch profits withdrawn by karbari",
			"user_id", userID,
			"karbari", karbari,
			"asset", asset,
			"amount", totalAmount,
			"count", len(profits),
		)

		// Send notification if notification client is available
		if s.notificationClient != nil {
			if err := s.notificationClient.SendFeatureHourlyProfitDeposit(ctx, userID, asset, totalAmount, karbari, ""); err != nil {
				s.log.Warn("Failed to send notification", "error", err)
			}
		}
	}

	return totalAmount, nil
}

// TransferProfitOnSale handles profit transfer when feature is sold
// Uses gRPC to add accumulated profit to seller's wallet
func (s *ProfitService) TransferProfitOnSale(ctx context.Context, featureID, sellerID, buyerID uint64, withdrawProfitDays int) error {
	// Get existing profit for seller
	oldProfit, err := s.profitRepo.GetByFeatureAndUser(ctx, featureID, sellerID)
	if err == nil && oldProfit != nil && oldProfit.Amount > 0 && s.commercialClient != nil {
		// Add accumulated profit to seller's wallet via gRPC
		if err := s.commercialClient.AddBalance(ctx, sellerID, oldProfit.Asset, oldProfit.Amount); err != nil {
			s.log.Error("Failed to transfer profit to seller", "error", err)
			return err
		}

		s.log.Info("Profit transferred on sale",
			"feature_id", featureID,
			"seller_id", sellerID,
			"amount", oldProfit.Amount,
			"asset", oldProfit.Asset,
		)
	}

	// Transfer profit record to new owner
	asset := ""
	if oldProfit != nil {
		asset = oldProfit.Asset
	}
	if err := s.profitRepo.TransferProfitToNewOwner(ctx, featureID, sellerID, buyerID, asset, withdrawProfitDays); err != nil {
		return fmt.Errorf("failed to transfer profit record: %w", err)
	}

	return nil
}

// GetHourlyProfits retrieves paginated hourly profits for a user
// Returns profits with feature information and formatted totals
func (s *ProfitService) GetHourlyProfits(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, string, string, string, bool, error) {
	// Default page size to 10 if not specified
	if pageSize <= 0 {
		pageSize = 10
	}
	if page <= 0 {
		page = 1
	}

	// Get profits with pagination
	profits, hasMore, err := s.profitRepo.FindByUserID(ctx, userID, page, pageSize)
	if err != nil {
		return nil, "0.00", "0.00", "0.00", false, fmt.Errorf("failed to get profits: %w", err)
	}

	// Get totals by karbari and format to 2 decimal places
	totalMaskoni, totalTejari, totalAmozeshi, err := s.profitRepo.GetTotalsByKarbari(ctx, userID)
	if err != nil {
		return profits, "0.00", "0.00", "0.00", hasMore, nil
	}

	// Format totals to 2 decimal places (matching Laravel's number_format(..., 2))
	totalMaskoniFormatted := formatTotal(totalMaskoni)
	totalTejariFormatted := formatTotal(totalTejari)
	totalAmozeshiFormatted := formatTotal(totalAmozeshi)

	return profits, totalMaskoniFormatted, totalTejariFormatted, totalAmozeshiFormatted, hasMore, nil
}

// GetHourlyProfitTimePercentage implements Laravel's hourlyProfitInfo helper.
func (s *ProfitService) GetHourlyProfitTimePercentage(ctx context.Context, userID uint64) (float64, error) {
	profit, err := s.profitRepo.FindOldestByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get oldest hourly profit: %w", err)
	}
	if profit == nil {
		return 0, nil
	}

	now := time.Now()
	totalSeconds := math.Abs(profit.Deadline.Sub(profit.UpdatedAt).Seconds())
	secondsPassed := math.Abs(now.Sub(profit.UpdatedAt).Seconds())

	if totalSeconds == 0 || secondsPassed >= totalSeconds {
		return 0, nil
	}

	percentage := (secondsPassed / totalSeconds) * 100
	return math.Round(percentage*100) / 100, nil
}

// formatTotal formats a total amount string to 2 decimal places
func formatTotal(totalStr string) string {
	total, err := strconv.ParseFloat(totalStr, 64)
	if err != nil {
		return "0.00"
	}
	return fmt.Sprintf("%.2f", total)
}

// RunHourlyProfitCalculation processes all eligible profit records.
// Eligibility (deadline not passed, is_active, last update >= 3 hours ago) is enforced in the repository.
func (s *ProfitService) RunHourlyProfitCalculation(ctx context.Context) (int, error) {
	const maxBatches = 1000

	totalUpdated := 0
	for batch := 0; batch < maxBatches; batch++ {
		if err := ctx.Err(); err != nil {
			return totalUpdated, err
		}

		updated, err := s.profitRepo.CalculateAndUpdateProfits(ctx)
		if err != nil {
			return totalUpdated, fmt.Errorf("failed to calculate hourly profits: %w", err)
		}

		totalUpdated += updated
		if updated == 0 {
			break
		}
	}

	return totalUpdated, nil
}

// StartHourlyProfitCalculator runs the profit calculation on a fixed interval.
// The scheduler ticks every minute; each record is only updated once every 3 hours.
func (s *ProfitService) StartHourlyProfitCalculator(ctx context.Context, log *logger.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	log.Info("Hourly profit calculator started",
		"scheduler_interval", "1m",
		"per_record_interval", fmt.Sprintf("%dh", constants.HourlyProfitCalculationIntervalHours),
	)

	run := func() {
		updated, err := s.RunHourlyProfitCalculation(ctx)
		if err != nil {
			log.Error("Hourly profit calculation failed", "error", err)
			return
		}
		if updated > 0 {
			log.Info("Hourly profit calculation completed", "updated_records", updated)
		}
	}

	run()

	for {
		select {
		case <-ctx.Done():
			log.Info("Hourly profit calculator stopped")
			return
		case <-ticker.C:
			run()
		}
	}
}

// Utility methods
func (s *ProfitService) getUserVariableWithdrawProfit(ctx context.Context, userID uint64) (int, error) {
	var days int
	err := s.db.QueryRowContext(ctx, "SELECT withdraw_profit FROM user_variables WHERE user_id = ?", userID).Scan(&days)
	if err != nil {
		return 10, nil
	}
	return days, nil
}
