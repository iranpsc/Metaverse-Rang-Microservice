package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/models"
)

type HourlyProfitRepository struct {
	db *sql.DB
}

func NewHourlyProfitRepository(db *sql.DB) *HourlyProfitRepository {
	return &HourlyProfitRepository{db: db}
}

// Create ensures a single hourly profit record for the feature owned by userID.
// If any rows already exist for feature_id, the latest is reassigned (amount reset)
// and extras are deleted — never inserts a second row for the same feature.
func (r *HourlyProfitRepository) Create(ctx context.Context, userID, featureID uint64, asset string, withdrawProfitDays int) (uint64, error) {
	deadlineSeconds := withdrawProfitDays * 86400
	deadline := time.Now().Add(time.Duration(deadlineSeconds) * time.Second)

	var existingID uint64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM feature_hourly_profits WHERE feature_id = ? ORDER BY id DESC LIMIT 1`,
		featureID,
	).Scan(&existingID)
	if err == nil {
		if _, err := r.db.ExecContext(ctx,
			`DELETE FROM feature_hourly_profits WHERE feature_id = ? AND id != ?`,
			featureID, existingID,
		); err != nil {
			return 0, err
		}
		_, err = r.db.ExecContext(ctx, `
			UPDATE feature_hourly_profits
			SET user_id = ?, asset = ?, amount = 0, dead_line = ?, is_active = 1, updated_at = NOW()
			WHERE id = ?
		`, userID, asset, deadline, existingID)
		return existingID, err
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	query := `
		INSERT INTO feature_hourly_profits (user_id, feature_id, asset, amount, dead_line, is_active, created_at, updated_at)
		VALUES (?, ?, ?, 0, ?, 1, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, userID, featureID, asset, deadline)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return uint64(id), err
}

// oneFeaturePropertyJoin picks a single feature_properties row per feature.
// feature_properties.feature_id is not unique; a plain JOIN duplicates hourly profits.
const oneFeaturePropertyJoin = `
		LEFT JOIN feature_properties fp ON fp.id = (
			SELECT fp2.id
			FROM feature_properties fp2
			WHERE fp2.feature_id = fhp.feature_id
			ORDER BY fp2.id ASC
			LIMIT 1
		)`

// FindByID retrieves a single profit record
// Joins with feature_properties to get karbari and properties.id
func (r *HourlyProfitRepository) FindByID(ctx context.Context, id uint64) (*models.FeatureHourlyProfit, error) {
	profit := &models.FeatureHourlyProfit{}

	query := `
		SELECT 
			fhp.id, 
			fhp.user_id, 
			fhp.feature_id, 
			fhp.asset, 
			fhp.amount, 
			fhp.dead_line, 
			fhp.is_active, 
			fhp.created_at, 
			fhp.updated_at,
			f.id as feature_db_id,
			fp.id as properties_id,
			fp.karbari
		FROM feature_hourly_profits fhp
		INNER JOIN features f ON fhp.feature_id = f.id` + oneFeaturePropertyJoin + `
		WHERE fhp.id = ?
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&profit.ID, &profit.UserID, &profit.FeatureID, &profit.Asset,
		&profit.Amount, &profit.Deadline, &profit.IsActive,
		&profit.CreatedAt, &profit.UpdatedAt,
		&profit.FeatureDBID, &profit.PropertiesID, &profit.Karbari,
	)

	return profit, err
}

// FindByUserID retrieves all profits for a user with pagination
// Joins with feature_properties to get karbari and properties.id.
// Guarantees one row per feature: the properties join is limited to a single
// property row, and only the latest profit record per feature is returned
// (orphaned duplicates can exist after ownership cycles).
func (r *HourlyProfitRepository) FindByUserID(ctx context.Context, userID uint64, page, pageSize int32) ([]*models.FeatureHourlyProfit, bool, error) {
	offset := (page - 1) * pageSize
	limit := pageSize + 1

	query := `
		SELECT 
			fhp.id, 
			fhp.user_id, 
			fhp.feature_id, 
			fhp.asset, 
			fhp.amount, 
			fhp.dead_line, 
			fhp.is_active, 
			fhp.created_at, 
			fhp.updated_at,
			f.id as feature_db_id,
			fp.id as properties_id,
			fp.karbari
		FROM feature_hourly_profits fhp
		INNER JOIN features f ON fhp.feature_id = f.id` + oneFeaturePropertyJoin + `
		WHERE fhp.user_id = ?
		  AND fhp.id = (
			SELECT MAX(fhp2.id)
			FROM feature_hourly_profits fhp2
			WHERE fhp2.user_id = fhp.user_id
			  AND fhp2.feature_id = fhp.feature_id
		  )
		ORDER BY fhp.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = rows.Close() }()

	profits := []*models.FeatureHourlyProfit{}
	for rows.Next() {
		profit := &models.FeatureHourlyProfit{}
		if err := rows.Scan(
			&profit.ID, &profit.UserID, &profit.FeatureID, &profit.Asset,
			&profit.Amount, &profit.Deadline, &profit.IsActive,
			&profit.CreatedAt, &profit.UpdatedAt,
			&profit.FeatureDBID, &profit.PropertiesID, &profit.Karbari,
		); err != nil {
			continue
		}
		profits = append(profits, profit)
	}

	hasMore := int32(len(profits)) > pageSize
	if hasMore {
		profits = profits[:pageSize]
	}

	return profits, hasMore, nil
}

// GetTotalsByKarbari calculates total amounts for each karbari
func (r *HourlyProfitRepository) GetTotalsByKarbari(ctx context.Context, userID uint64) (maskoni, tejari, amozeshi string, err error) {
	query := `
		SELECT fp.karbari, SUM(fhp.amount) as total
		FROM feature_hourly_profits fhp
		INNER JOIN feature_properties fp ON fp.id = (
			SELECT fp2.id
			FROM feature_properties fp2
			WHERE fp2.feature_id = fhp.feature_id
			ORDER BY fp2.id ASC
			LIMIT 1
		)
		WHERE fhp.user_id = ?
		  AND fhp.id = (
			SELECT MAX(fhp2.id)
			FROM feature_hourly_profits fhp2
			WHERE fhp2.user_id = fhp.user_id
			  AND fhp2.feature_id = fhp.feature_id
		  )
		GROUP BY fp.karbari
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return "0", "0", "0", err
	}
	defer func() { _ = rows.Close() }()

	totals := map[string]float64{
		"m": 0,
		"t": 0,
		"a": 0,
	}

	for rows.Next() {
		var karbari string
		var total float64
		if err := rows.Scan(&karbari, &total); err != nil {
			continue
		}
		totals[karbari] = total
	}

	return fmt.Sprintf("%.6f", totals["m"]),
		fmt.Sprintf("%.6f", totals["t"]),
		fmt.Sprintf("%.6f", totals["a"]),
		nil
}

// ResetProfitAndUpdateDeadline resets amount to 0 and updates deadline
// Implements Laravel's FeatureHourlyProfitController@getSingleProfit logic
func (r *HourlyProfitRepository) ResetProfitAndUpdateDeadline(ctx context.Context, profitID uint64, withdrawProfitDays int) error {
	deadlineSeconds := withdrawProfitDays * 86400
	newDeadline := time.Now().Add(time.Duration(deadlineSeconds) * time.Second)

	query := `
		UPDATE feature_hourly_profits
		SET amount = 0, dead_line = ?, updated_at = NOW()
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query, newDeadline, profitID)
	return err
}

// CalculateAndUpdateProfits implements the hourly profit calculation job
// From Laravel's CalculateFeatureProfit command.
// Returns the number of profit records updated in this batch (max 100).
func (r *HourlyProfitRepository) CalculateAndUpdateProfits(ctx context.Context) (int, error) {
	// Find all profits that need updating:
	// - dead_line > now (not expired)
	// - updated_at < 3 hours ago
	// - is_active = true
	threeHoursAgo := time.Now().Add(-3 * time.Hour)

	query := `
		SELECT fhp.id, fhp.feature_id
		FROM feature_hourly_profits fhp
		WHERE fhp.dead_line > NOW()
		  AND fhp.updated_at < ?
		  AND fhp.is_active = 1
		LIMIT 100
	`

	rows, err := r.db.QueryContext(ctx, query, threeHoursAgo)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	profits := []struct {
		ID        uint64
		FeatureID uint64
	}{}

	for rows.Next() {
		var p struct {
			ID        uint64
			FeatureID uint64
		}
		if err := rows.Scan(&p.ID, &p.FeatureID); err != nil {
			continue
		}
		profits = append(profits, p)
	}

	// For each profit, get feature stability and increment amount
	updated := 0
	for _, p := range profits {
		var stability float64
		stabilityQuery := "SELECT stability FROM feature_properties WHERE feature_id = ?"
		if err := r.db.QueryRowContext(ctx, stabilityQuery, p.FeatureID).Scan(&stability); err != nil {
			continue
		}

		// Increment amount by stability * 0.000041666
		increment := stability * constants.HourlyProfitCalculationRate

		updateQuery := "UPDATE feature_hourly_profits SET amount = amount + ?, updated_at = NOW() WHERE id = ?"
		if _, err := r.db.ExecContext(ctx, updateQuery, increment, p.ID); err != nil {
			continue
		}
		updated++
	}

	return updated, nil
}

type profitExecer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TransferProfitToNewOwner reassigns the feature's hourly profit to the buyer.
// asset is used when creating a new row (or refreshing an orphaned one).
// Guarantees a single profit row per feature_id.
func (r *HourlyProfitRepository) TransferProfitToNewOwner(ctx context.Context, featureID, oldOwnerID, newOwnerID uint64, asset string, withdrawProfitDays int) error {
	return r.transferProfitToNewOwner(ctx, r.db, featureID, oldOwnerID, newOwnerID, asset, withdrawProfitDays)
}

// TransferProfitToNewOwnerWithTx transfers profit within a transaction
func (r *HourlyProfitRepository) TransferProfitToNewOwnerWithTx(ctx context.Context, tx *sql.Tx, featureID, oldOwnerID, newOwnerID uint64, asset string, withdrawProfitDays int) error {
	return r.transferProfitToNewOwner(ctx, tx, featureID, oldOwnerID, newOwnerID, asset, withdrawProfitDays)
}

func (r *HourlyProfitRepository) transferProfitToNewOwner(
	ctx context.Context,
	ex profitExecer,
	featureID, oldOwnerID, newOwnerID uint64,
	asset string,
	withdrawProfitDays int,
) error {
	deadlineSeconds := withdrawProfitDays * 86400
	newDeadline := time.Now().Add(time.Duration(deadlineSeconds) * time.Second)

	var profitID uint64
	var existingAsset string

	// Prefer seller's row; otherwise take any existing row for this feature
	err := ex.QueryRowContext(ctx,
		`SELECT id, asset FROM feature_hourly_profits WHERE feature_id = ? AND user_id = ? ORDER BY id DESC LIMIT 1`,
		featureID, oldOwnerID,
	).Scan(&profitID, &existingAsset)
	if err == sql.ErrNoRows {
		err = ex.QueryRowContext(ctx,
			`SELECT id, asset FROM feature_hourly_profits WHERE feature_id = ? ORDER BY id DESC LIMIT 1`,
			featureID,
		).Scan(&profitID, &existingAsset)
	} else if err != nil {
		return err
	}

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if asset == "" {
		asset = existingAsset
	}

	if err == sql.ErrNoRows {
		result, err := ex.ExecContext(ctx, `
			INSERT INTO feature_hourly_profits (user_id, feature_id, asset, amount, dead_line, is_active, created_at, updated_at)
			VALUES (?, ?, ?, 0, ?, 1, NOW(), NOW())
		`, newOwnerID, featureID, asset, newDeadline)
		if err != nil {
			return err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		profitID = uint64(id)
	} else {
		if _, err := ex.ExecContext(ctx, `
			UPDATE feature_hourly_profits
			SET user_id = ?, asset = ?, amount = 0, dead_line = ?, is_active = 1, updated_at = NOW()
			WHERE id = ?
		`, newOwnerID, asset, newDeadline, profitID); err != nil {
			return err
		}
	}

	// Drop orphaned duplicates so each feature has at most one profit row
	_, err = ex.ExecContext(ctx,
		`DELETE FROM feature_hourly_profits WHERE feature_id = ? AND id != ?`,
		featureID, profitID,
	)
	return err
}

// GetByFeatureAndUser retrieves profit for a specific feature and user
func (r *HourlyProfitRepository) GetByFeatureAndUser(ctx context.Context, featureID, userID uint64) (*models.FeatureHourlyProfit, error) {
	profit := &models.FeatureHourlyProfit{}

	query := `
		SELECT id, user_id, feature_id, asset, amount, dead_line, is_active, created_at, updated_at
		FROM feature_hourly_profits
		WHERE feature_id = ? AND user_id = ?
	`

	err := r.db.QueryRowContext(ctx, query, featureID, userID).Scan(
		&profit.ID, &profit.UserID, &profit.FeatureID, &profit.Asset,
		&profit.Amount, &profit.Deadline, &profit.IsActive,
		&profit.CreatedAt, &profit.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	return profit, err
}

// GetAllByUserAndKarbari gets all profits for user filtered by feature_properties.karbari
func (r *HourlyProfitRepository) GetAllByUserAndKarbari(ctx context.Context, userID uint64, karbari string) ([]*models.FeatureHourlyProfit, error) {
	query := `
		SELECT fhp.id, fhp.user_id, fhp.feature_id, fhp.asset, fhp.amount, fhp.dead_line, fhp.is_active, fhp.created_at, fhp.updated_at
		FROM feature_hourly_profits fhp
		INNER JOIN features f ON fhp.feature_id = f.id` + oneFeaturePropertyJoin + `
		WHERE fhp.user_id = ? AND fp.karbari = ?
		  AND fhp.id = (
			SELECT MAX(fhp2.id)
			FROM feature_hourly_profits fhp2
			WHERE fhp2.user_id = fhp.user_id
			  AND fhp2.feature_id = fhp.feature_id
		  )
		ORDER BY fhp.id
	`

	rows, err := r.db.QueryContext(ctx, query, userID, karbari)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	profits := []*models.FeatureHourlyProfit{}
	for rows.Next() {
		profit := &models.FeatureHourlyProfit{}
		if err := rows.Scan(
			&profit.ID, &profit.UserID, &profit.FeatureID, &profit.Asset,
			&profit.Amount, &profit.Deadline, &profit.IsActive,
			&profit.CreatedAt, &profit.UpdatedAt,
		); err != nil {
			continue
		}
		profits = append(profits, profit)
	}

	return profits, nil
}

// ActivateProfitsForFeature activates all profits for a feature
// Used when destroying buildings
func (r *HourlyProfitRepository) ActivateProfitsForFeature(ctx context.Context, featureID uint64) error {
	query := "UPDATE feature_hourly_profits SET is_active = 1, updated_at = NOW() WHERE feature_id = ?"
	_, err := r.db.ExecContext(ctx, query, featureID)
	return err
}

// DeactivateProfitsForFeature deactivates all profits for a feature
// Used when starting building construction
func (r *HourlyProfitRepository) DeactivateProfitsForFeature(ctx context.Context, featureID uint64) error {
	query := "UPDATE feature_hourly_profits SET is_active = 0, updated_at = NOW() WHERE feature_id = ?"
	_, err := r.db.ExecContext(ctx, query, featureID)
	return err
}

// FindOldestByUserID returns the user's hourly profit with the earliest deadline.
// Matches Laravel: FeatureHourlyProfit::whereUserId($user->id)->oldest('dead_line')->first()
func (r *HourlyProfitRepository) FindOldestByUserID(ctx context.Context, userID uint64) (*models.FeatureHourlyProfit, error) {
	query := `
		SELECT id, user_id, feature_id, asset, amount, dead_line, is_active, created_at, updated_at
		FROM feature_hourly_profits
		WHERE user_id = ?
		ORDER BY dead_line ASC
		LIMIT 1
	`

	profit := &models.FeatureHourlyProfit{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&profit.ID, &profit.UserID, &profit.FeatureID, &profit.Asset,
		&profit.Amount, &profit.Deadline, &profit.IsActive,
		&profit.CreatedAt, &profit.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find oldest hourly profit: %w", err)
	}

	return profit, nil
}
