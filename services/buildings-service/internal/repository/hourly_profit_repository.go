package repository

import (
	"context"
	"database/sql"
	"log/slog"
)

type HourlyProfitRepository struct {
	db *sql.DB
}

func NewHourlyProfitRepository(db *sql.DB) *HourlyProfitRepository {
	return &HourlyProfitRepository{db: db}
}

// ActivateProfitsForFeature activates all profits for a feature (used when destroying buildings).
func (r *HourlyProfitRepository) ActivateProfitsForFeature(ctx context.Context, featureID uint64) error {
	query := "UPDATE feature_hourly_profits SET is_active = 1, updated_at = NOW() WHERE feature_id = ?"
	_, err := r.db.ExecContext(ctx, query, featureID)
	if err != nil {
		slog.Error("activate feature hourly profits failed", "feature_id", featureID, "error", err)
	}
	return err
}

// DeactivateProfitsForFeature deactivates all profits for a feature (used when starting construction).
func (r *HourlyProfitRepository) DeactivateProfitsForFeature(ctx context.Context, featureID uint64) error {
	query := "UPDATE feature_hourly_profits SET is_active = 0, updated_at = NOW() WHERE feature_id = ?"
	_, err := r.db.ExecContext(ctx, query, featureID)
	if err != nil {
		slog.Error("deactivate feature hourly profits failed", "feature_id", featureID, "error", err)
	}
	return err
}
