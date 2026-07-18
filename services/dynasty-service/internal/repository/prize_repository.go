package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/dynasty-service/internal/models"
)

type PrizeRepository struct {
	db *sql.DB
}

func NewPrizeRepository(db *sql.DB) *PrizeRepository {
	return &PrizeRepository{db: db}
}

// GetPrizeByRelationship retrieves dynasty prize by relationship type
func (r *PrizeRepository) GetPrizeByRelationship(ctx context.Context, relationship string) (*models.DynastyPrize, error) {
	query := `
		SELECT id, member, satisfaction, introduction_profit_increase, 
		       accumulated_capital_reserve, data_storage, psc, created_at, updated_at
		FROM dynasty_prizes 
		WHERE member = ?
		LIMIT 1
	`

	var prize models.DynastyPrize
	err := r.db.QueryRowContext(ctx, query, relationship).Scan(
		&prize.ID,
		&prize.Member,
		&prize.Satisfaction,
		&prize.IntroductionProfitIncrease,
		&prize.AccumulatedCapitalReserve,
		&prize.DataStorage,
		&prize.PSC,
		&prize.CreatedAt,
		&prize.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get prize: %w", err)
	}

	return &prize, nil
}

// AwardPrize creates a received prize record for a user
func (r *PrizeRepository) AwardPrize(ctx context.Context, userID, prizeID uint64, message string) error {
	query := `
		INSERT INTO received_prizes (user_id, prize_id, message, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query, userID, prizeID, message)
	if err != nil {
		return fmt.Errorf("failed to award prize: %w", err)
	}

	return nil
}

// GetReceivedPrize retrieves a received prize by ID
func (r *PrizeRepository) GetReceivedPrize(ctx context.Context, receivedPrizeID uint64) (*models.ReceivedPrize, error) {
	query := `
		SELECT rp.id, rp.user_id, rp.prize_id, rp.message, rp.created_at, rp.updated_at,
		       dp.member, dp.satisfaction, dp.introduction_profit_increase,
		       dp.accumulated_capital_reserve, dp.data_storage, dp.psc
		FROM received_prizes rp
		INNER JOIN dynasty_prizes dp ON dp.id = rp.prize_id
		WHERE rp.id = ?
	`

	var received models.ReceivedPrize
	var prize models.DynastyPrize

	err := r.db.QueryRowContext(ctx, query, receivedPrizeID).Scan(
		&received.ID,
		&received.UserID,
		&received.PrizeID,
		&received.Message,
		&received.CreatedAt,
		&received.UpdatedAt,
		&prize.Member,
		&prize.Satisfaction,
		&prize.IntroductionProfitIncrease,
		&prize.AccumulatedCapitalReserve,
		&prize.DataStorage,
		&prize.PSC,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get received prize: %w", err)
	}

	received.Prize = &prize
	return &received, nil
}

// GetUserReceivedPrizes retrieves all received prizes for a user
func (r *PrizeRepository) GetUserReceivedPrizes(ctx context.Context, userID uint64) ([]*models.ReceivedPrize, error) {
	query := `
		SELECT rp.id, rp.user_id, rp.prize_id, rp.message, rp.created_at, rp.updated_at,
		       dp.id, dp.member, dp.satisfaction, dp.introduction_profit_increase,
		       dp.accumulated_capital_reserve, dp.data_storage, dp.psc
		FROM received_prizes rp
		INNER JOIN dynasty_prizes dp ON dp.id = rp.prize_id
		WHERE rp.user_id = ?
		ORDER BY rp.created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user prizes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var prizes []*models.ReceivedPrize
	for rows.Next() {
		var received models.ReceivedPrize
		var prize models.DynastyPrize

		if err := rows.Scan(
			&received.ID,
			&received.UserID,
			&received.PrizeID,
			&received.Message,
			&received.CreatedAt,
			&received.UpdatedAt,
			&prize.ID,
			&prize.Member,
			&prize.Satisfaction,
			&prize.IntroductionProfitIncrease,
			&prize.AccumulatedCapitalReserve,
			&prize.DataStorage,
			&prize.PSC,
		); err != nil {
			return nil, fmt.Errorf("failed to scan prize: %w", err)
		}

		received.Prize = &prize
		prizes = append(prizes, &received)
	}

	return prizes, nil
}

// DeleteReceivedPrize deletes a claimed prize
func (r *PrizeRepository) DeleteReceivedPrize(ctx context.Context, receivedPrizeID uint64) error {
	return r.DeleteReceivedPrizeTx(ctx, nil, receivedPrizeID)
}

// DeleteReceivedPrizeTx deletes a received_prizes row inside an optional transaction.
func (r *PrizeRepository) DeleteReceivedPrizeTx(ctx context.Context, tx *sql.Tx, receivedPrizeID uint64) error {
	query := `DELETE FROM received_prizes WHERE id = ?`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, receivedPrizeID)
	} else {
		_, err = r.db.ExecContext(ctx, query, receivedPrizeID)
	}
	if err != nil {
		return fmt.Errorf("failed to delete received prize: %w", err)
	}
	return nil
}

// GetAllDynastyPrizes retrieves all dynasty prizes (for introduction/display)
func (r *PrizeRepository) GetAllDynastyPrizes(ctx context.Context) ([]*models.DynastyPrize, error) {
	query := `
		SELECT id, member, satisfaction, introduction_profit_increase,
		       accumulated_capital_reserve, data_storage, psc, created_at, updated_at
		FROM dynasty_prizes
		ORDER BY id
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty prizes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var prizes []*models.DynastyPrize
	for rows.Next() {
		var prize models.DynastyPrize
		if err := rows.Scan(
			&prize.ID,
			&prize.Member,
			&prize.Satisfaction,
			&prize.IntroductionProfitIncrease,
			&prize.AccumulatedCapitalReserve,
			&prize.DataStorage,
			&prize.PSC,
			&prize.CreatedAt,
			&prize.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan prize: %w", err)
		}
		prizes = append(prizes, &prize)
	}

	return prizes, nil
}

// GetAllPrizes retrieves all dynasty prizes with pagination
func (r *PrizeRepository) GetAllPrizes(ctx context.Context, page, perPage int32) ([]*models.DynastyPrize, int32, error) {
	offset := (page - 1) * perPage

	// Get total count
	countQuery := `SELECT COUNT(*) FROM dynasty_prizes`
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count prizes: %w", err)
	}

	// Get prizes
	query := `
		SELECT id, member, satisfaction, introduction_profit_increase,
		       accumulated_capital_reserve, data_storage, psc, created_at, updated_at
		FROM dynasty_prizes
		ORDER BY id
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get dynasty prizes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var prizes []*models.DynastyPrize
	for rows.Next() {
		var prize models.DynastyPrize
		if err := rows.Scan(
			&prize.ID,
			&prize.Member,
			&prize.Satisfaction,
			&prize.IntroductionProfitIncrease,
			&prize.AccumulatedCapitalReserve,
			&prize.DataStorage,
			&prize.PSC,
			&prize.CreatedAt,
			&prize.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan prize: %w", err)
		}
		prizes = append(prizes, &prize)
	}

	return prizes, total, nil
}

// GetPrizeByID retrieves a dynasty prize by ID
func (r *PrizeRepository) GetPrizeByID(ctx context.Context, prizeID uint64) (*models.DynastyPrize, error) {
	query := `
		SELECT id, member, satisfaction, introduction_profit_increase,
		       accumulated_capital_reserve, data_storage, psc, created_at, updated_at
		FROM dynasty_prizes 
		WHERE id = ?
		LIMIT 1
	`

	var prize models.DynastyPrize
	err := r.db.QueryRowContext(ctx, query, prizeID).Scan(
		&prize.ID,
		&prize.Member,
		&prize.Satisfaction,
		&prize.IntroductionProfitIncrease,
		&prize.AccumulatedCapitalReserve,
		&prize.DataStorage,
		&prize.PSC,
		&prize.CreatedAt,
		&prize.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get prize: %w", err)
	}

	return &prize, nil
}

// CheckPrizeClaimed checks if a user has already claimed a specific prize
func (r *PrizeRepository) CheckPrizeClaimed(ctx context.Context, userID, prizeID uint64) (bool, error) {
	query := `SELECT COUNT(*) FROM received_prizes WHERE user_id = ? AND prize_id = ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, userID, prizeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check prize claimed: %w", err)
	}

	return count > 0, nil
}

// ClaimPrize allows a user to claim a prize
func (r *PrizeRepository) ClaimPrize(ctx context.Context, userID, prizeID uint64) error {
	query := `
		INSERT INTO received_prizes (user_id, prize_id, message, created_at, updated_at)
		VALUES (?, ?, '', NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query, userID, prizeID)
	if err != nil {
		return fmt.Errorf("failed to claim prize: %w", err)
	}

	return nil
}
