// Package repository provides data access for the levels service.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	pb "metarang/shared/pb/levels"
)

// ActivityRepository handles user_activities table operations
// Implements Laravel's UserActivity model
type ActivityRepository struct {
	db *sql.DB
}

type ActivityRepositoryInterface interface {
	CreateActivity(ctx context.Context, req *pb.LogActivityRequest) (uint64, error)
	FindByUserID(ctx context.Context, userID uint64, limit int32) ([]*pb.UserActivity, error)
	GetLatestActivity(ctx context.Context, userID uint64) (*pb.UserActivity, error)
	UpdateActivity(ctx context.Context, activityID uint64, endTime time.Time, totalMinutes int32) error
	GetTotalActivityMinutes(ctx context.Context, userID uint64) (int32, error)
	CreateUserEvent(ctx context.Context, userID uint64, event, ip, device string, status int8) error
	GetVariableRate(ctx context.Context, name string) (float64, error)
	GetSignificantTradeCount(ctx context.Context, userID uint64, minIrrAmount, minPscAmount float64) (int32, error)
}

func NewActivityRepository(db *sql.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// CreateActivity creates a new user activity record
// Implements Laravel: $user->activities()->create([...])
func (r *ActivityRepository) CreateActivity(ctx context.Context, req *pb.LogActivityRequest) (uint64, error) {
	query := `
		INSERT INTO user_activities (user_id, start, ip, created_at, updated_at)
		VALUES (?, NOW(), ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query, req.UserId, req.Ip)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return uint64(id), nil
}

// FindByUserID retrieves user's activity history
// Implements Laravel: $user->activities
func (r *ActivityRepository) FindByUserID(ctx context.Context, userID uint64, limit int32) ([]*pb.UserActivity, error) {
	query := `
		SELECT id, user_id, start, end, COALESCE(total, 0) as total, ip
		FROM user_activities
		WHERE user_id = ?
		ORDER BY id DESC
	`

	if limit > 0 {
		query += " LIMIT ?"
	}

	var rows *sql.Rows
	var err error

	if limit > 0 {
		rows, err = r.db.QueryContext(ctx, query, userID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, query, userID)
	}

	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var activities []*pb.UserActivity
	for rows.Next() {
		var activity pb.UserActivity
		var start, end sql.NullTime

		if err := rows.Scan(&activity.Id, &activity.UserId, &start, &end, &activity.Total, &activity.Ip); err != nil {
			return nil, err
		}

		if start.Valid {
			activity.Start = start.Time.Format(time.RFC3339)
		}
		if end.Valid {
			activity.End = end.Time.Format(time.RFC3339)
		}

		activities = append(activities, &activity)
	}

	return activities, nil
}

// GetLatestActivity retrieves user's latest activity session
// Implements Laravel: $user->latestActivity
func (r *ActivityRepository) GetLatestActivity(ctx context.Context, userID uint64) (*pb.UserActivity, error) {
	query := `
		SELECT id, user_id, start, end, COALESCE(total, 0) as total, ip
		FROM user_activities
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT 1
	`

	var activity pb.UserActivity
	var start, end sql.NullTime

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&activity.Id,
		&activity.UserId,
		&start,
		&end,
		&activity.Total,
		&activity.Ip,
	)

	if err != nil {
		return nil, err
	}

	if start.Valid {
		activity.Start = start.Time.Format(time.RFC3339)
	}
	if end.Valid {
		activity.End = end.Time.Format(time.RFC3339)
	}

	return &activity, nil
}

// UpdateActivity updates an activity session (for logout)
// Implements Laravel: $latestActivity->update(['end' => now(), 'total' => $minutes])
func (r *ActivityRepository) UpdateActivity(ctx context.Context, activityID uint64, endTime time.Time, totalMinutes int32) error {
	query := `
		UPDATE user_activities
		SET end = ?, total = ?, updated_at = NOW()
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query, endTime, totalMinutes, activityID)
	return err
}

// GetTotalActivityMinutes calculates total activity time for user
// Implements Laravel: $user->activities->sum('total')
func (r *ActivityRepository) GetTotalActivityMinutes(ctx context.Context, userID uint64) (int32, error) {
	query := "SELECT COALESCE(SUM(total), 0) FROM user_activities WHERE user_id = ?"
	var total int32
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&total)
	return total, err
}

// CreateUserEvent creates an event record (login, logout, etc.)
// Implements Laravel: $user->events()->create([...])
func (r *ActivityRepository) CreateUserEvent(ctx context.Context, userID uint64, event, ip, device string, status int8) error {
	query := `
		INSERT INTO user_events (user_id, event, ip, device, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query, userID, event, ip, device, status)
	return err
}

// GetVariableRate returns numeric value from system_variables table.
func (r *ActivityRepository) GetVariableRate(ctx context.Context, name string) (float64, error) {
	query := "SELECT value FROM system_variables WHERE name = ? LIMIT 1"
	var value string
	if err := r.db.QueryRowContext(ctx, query, name).Scan(&value); err != nil {
		return 0, err
	}

	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse variable %s rate: %w", name, err)
	}
	return rate, nil
}

// GetSignificantTradeCount counts user trades above the significant threshold.
func (r *ActivityRepository) GetSignificantTradeCount(ctx context.Context, userID uint64, minIrrAmount, minPscAmount float64) (int32, error) {
	query := `
		SELECT COUNT(*)
		FROM trades
		WHERE (buyer_id = ? AND (irr_amount > ? OR psc_amount > ?))
		   OR (seller_id = ? AND (irr_amount > ? OR psc_amount > ?))
	`

	var count int32
	err := r.db.QueryRowContext(
		ctx,
		query,
		userID, minIrrAmount, minPscAmount,
		userID, minIrrAmount, minPscAmount,
	).Scan(&count)
	return count, err
}
