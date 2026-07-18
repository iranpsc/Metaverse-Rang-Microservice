package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	pb "metarang/shared/pb/levels"
)

// UserLogRepository handles user_logs table operations
// Implements Laravel's UserLog model with score calculation
type UserLogRepository struct {
	db *sql.DB
}

type UserLogRepositoryInterface interface {
	GetUserScore(ctx context.Context, userID uint64) (int32, error)
	GetUserLog(ctx context.Context, userID uint64) (*pb.UserLog, error)
	UpdateScore(ctx context.Context, userID uint64, score int32) error
	UpdateTransactionsCount(ctx context.Context, userID uint64, count string) error
	IncrementDeposit(ctx context.Context, userID uint64, amount string) error
	UpdateFollowersCount(ctx context.Context, userID uint64, totalFollowers int32) error
	UpdateActivityHours(ctx context.Context, userID uint64, totalMinutes int32) error
	GetTotalFollowers(ctx context.Context, userID uint64) (int32, error)
	CalculateScore(ctx context.Context, userID uint64) (int32, error)
}

func NewUserLogRepository(db *sql.DB) *UserLogRepository {
	return &UserLogRepository{db: db}
}

// GetUserScore retrieves user's current score
// Implements Laravel: $user->score
func (r *UserLogRepository) GetUserScore(ctx context.Context, userID uint64) (int32, error) {
	query := "SELECT score FROM users WHERE id = ?"
	var score sql.NullString
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&score)
	if err != nil {
		return 0, err
	}

	if !score.Valid || score.String == "" {
		return 0, nil
	}

	scoreInt, err := strconv.ParseFloat(score.String, 32)
	if err != nil {
		return 0, err
	}

	return int32(scoreInt), nil
}

// GetUserLog retrieves user's activity log
// Implements Laravel: $user->log
func (r *UserLogRepository) GetUserLog(ctx context.Context, userID uint64) (*pb.UserLog, error) {
	query := `
		SELECT id, user_id,
		       COALESCE(transactions_count, '0') as transactions_count,
		       COALESCE(followers_count, '0') as followers_count,
		       COALESCE(deposit_amount, '0') as deposit_amount,
		       COALESCE(activity_hours, '0') as activity_hours,
		       COALESCE(score, '0') as score
		FROM user_logs
		WHERE user_id = ?
	`

	var log pb.UserLog

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&log.Id,
		&log.UserId,
		&log.TransactionsCount,
		&log.FollowersCount,
		&log.DepositAmount,
		&log.ActivityHours,
		&log.Score,
	)

	if err != nil {
		return nil, err
	}

	return &log, nil
}

// UpdateScore updates both user_logs.score and users.score
// Implements Laravel: $log->update(['score' => $sum]); $user->update(['score' => $sum])
func (r *UserLogRepository) UpdateScore(ctx context.Context, userID uint64, score int32) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Update user_logs
	_, err = tx.ExecContext(ctx, "UPDATE user_logs SET score = ?, updated_at = NOW() WHERE user_id = ?", fmt.Sprintf("%d", score), userID)
	if err != nil {
		return err
	}

	// Update users
	_, err = tx.ExecContext(ctx, "UPDATE users SET score = ?, updated_at = NOW() WHERE id = ?", fmt.Sprintf("%d", score), userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateTransactionsCount updates the transactions count in user log
// Implements Laravel: $user->log->update(['transactions_count' => $trades * 2])
func (r *UserLogRepository) UpdateTransactionsCount(ctx context.Context, userID uint64, count string) error {
	query := "UPDATE user_logs SET transactions_count = ?, updated_at = NOW() WHERE user_id = ?"
	_, err := r.db.ExecContext(ctx, query, count, userID)
	return err
}

// IncrementDeposit increments deposit amount
// Implements Laravel: $user->log->increment('deposit_amount', $amount * 0.0001)
func (r *UserLogRepository) IncrementDeposit(ctx context.Context, userID uint64, amount string) error {
	// Parse amount and calculate increment (amount * 0.0001)
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return err
	}

	increment := amountFloat * 0.0001

	query := "UPDATE user_logs SET deposit_amount = deposit_amount + ?, updated_at = NOW() WHERE user_id = ?"
	_, err = r.db.ExecContext(ctx, query, fmt.Sprintf("%.4f", increment), userID)
	return err
}

// UpdateFollowersCount updates followers count
// Implements Laravel: $user->log->update(['followers_count' => $totalFollowers * 0.1])
func (r *UserLogRepository) UpdateFollowersCount(ctx context.Context, userID uint64, totalFollowers int32) error {
	count := float64(totalFollowers) * 0.1
	query := "UPDATE user_logs SET followers_count = ?, updated_at = NOW() WHERE user_id = ?"
	_, err := r.db.ExecContext(ctx, query, fmt.Sprintf("%.1f", count), userID)
	return err
}

// UpdateActivityHours updates activity hours
// Implements Laravel: $user->log->update(['activity_hours' => ceil($totalActiveHours / 60) * 0.1])
func (r *UserLogRepository) UpdateActivityHours(ctx context.Context, userID uint64, totalMinutes int32) error {
	hours := float64(totalMinutes) / 60.0
	ceiledHours := float64(int32(hours) + 1) // ceil function
	activityScore := ceiledHours * 0.1

	query := "UPDATE user_logs SET activity_hours = ?, updated_at = NOW() WHERE user_id = ?"
	_, err := r.db.ExecContext(ctx, query, fmt.Sprintf("%.1f", activityScore), userID)
	return err
}

// GetTotalFollowers counts user's followers
// Implements Laravel: $user->followers->count()
func (r *UserLogRepository) GetTotalFollowers(ctx context.Context, userID uint64) (int32, error) {
	query := "SELECT COUNT(*) FROM followers WHERE followed_id = ?"
	var count int32
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

// CalculateScore sums all score components
// Implements Laravel: array_sum([$log->transactions_count, $log->followers_count, $log->deposit_amount, $log->activity_hours])
func (r *UserLogRepository) CalculateScore(ctx context.Context, userID uint64) (int32, error) {
	log, err := r.GetUserLog(ctx, userID)
	if err != nil {
		return 0, err
	}

	transactions, _ := strconv.ParseFloat(log.TransactionsCount, 64)
	followers, _ := strconv.ParseFloat(log.FollowersCount, 64)
	deposit, _ := strconv.ParseFloat(log.DepositAmount, 64)
	activity, _ := strconv.ParseFloat(log.ActivityHours, 64)

	total := transactions + followers + deposit + activity

	return int32(total), nil
}
