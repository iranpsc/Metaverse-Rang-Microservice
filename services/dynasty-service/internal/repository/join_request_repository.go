package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/dynasty-service/internal/models"
)

type JoinRequestRepository struct {
	db *sql.DB
}

func NewJoinRequestRepository(db *sql.DB) *JoinRequestRepository {
	return &JoinRequestRepository{db: db}
}

// CreateJoinRequest creates a new join request
func (r *JoinRequestRepository) CreateJoinRequest(ctx context.Context, req *models.JoinRequest) error {
	query := `INSERT INTO join_requests (from_user, to_user, status, relationship, message, created_at, updated_at) 
	          VALUES (?, ?, ?, ?, ?, NOW(), NOW())`

	result, err := r.db.ExecContext(ctx, query,
		req.FromUser, req.ToUser, req.Status, req.Relationship, req.Message)
	if err != nil {
		return fmt.Errorf("failed to create join request: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get join request ID: %w", err)
	}

	req.ID = uint64(id)
	return nil
}

// GetJoinRequestByID retrieves a join request by ID
func (r *JoinRequestRepository) GetJoinRequestByID(ctx context.Context, id uint64) (*models.JoinRequest, error) {
	query := `SELECT id, from_user, to_user, status, relationship, message, created_at, updated_at 
	          FROM join_requests WHERE id = ?`

	var req models.JoinRequest
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&req.ID,
		&req.FromUser,
		&req.ToUser,
		&req.Status,
		&req.Relationship,
		&req.Message,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get join request: %w", err)
	}

	return &req, nil
}

// GetSentRequests retrieves join requests sent by a user
func (r *JoinRequestRepository) GetSentRequests(ctx context.Context, userID uint64, page, perPage int32) ([]*models.JoinRequest, int32, error) {
	offset := (page - 1) * perPage

	// Get total count
	countQuery := `SELECT COUNT(*) FROM join_requests WHERE from_user = ?`
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sent requests: %w", err)
	}

	// Get requests
	query := `SELECT id, from_user, to_user, status, relationship, message, created_at, updated_at 
	          FROM join_requests 
	          WHERE from_user = ? 
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get sent requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var requests []*models.JoinRequest
	for rows.Next() {
		var req models.JoinRequest
		if err := rows.Scan(
			&req.ID,
			&req.FromUser,
			&req.ToUser,
			&req.Status,
			&req.Relationship,
			&req.Message,
			&req.CreatedAt,
			&req.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan join request: %w", err)
		}
		requests = append(requests, &req)
	}

	return requests, total, nil
}

// GetReceivedRequests retrieves join requests received by a user
func (r *JoinRequestRepository) GetReceivedRequests(ctx context.Context, userID uint64, page, perPage int32) ([]*models.JoinRequest, int32, error) {
	offset := (page - 1) * perPage

	// Get total count
	countQuery := `SELECT COUNT(*) FROM join_requests WHERE to_user = ? AND status = 0`
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count received requests: %w", err)
	}

	// Get requests
	query := `SELECT id, from_user, to_user, status, relationship, message, created_at, updated_at 
	          FROM join_requests 
	          WHERE to_user = ? AND status = 0
	          ORDER BY created_at DESC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get received requests: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var requests []*models.JoinRequest
	for rows.Next() {
		var req models.JoinRequest
		if err := rows.Scan(
			&req.ID,
			&req.FromUser,
			&req.ToUser,
			&req.Status,
			&req.Relationship,
			&req.Message,
			&req.CreatedAt,
			&req.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan join request: %w", err)
		}
		requests = append(requests, &req)
	}

	return requests, total, nil
}

// UpdateJoinRequestStatus updates the status of a join request
func (r *JoinRequestRepository) UpdateJoinRequestStatus(ctx context.Context, id uint64, status int16) error {
	query := `UPDATE join_requests SET status = ?, updated_at = NOW() WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update join request status: %w", err)
	}

	return nil
}

// DeleteJoinRequest deletes a join request
func (r *JoinRequestRepository) DeleteJoinRequest(ctx context.Context, id uint64) error {
	query := `DELETE FROM join_requests WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete join request: %w", err)
	}

	return nil
}

// GetUserBasicInfo retrieves basic user information
func (r *JoinRequestRepository) GetUserBasicInfo(ctx context.Context, userID uint64) (*models.UserBasic, error) {
	query := `
		SELECT u.id, u.code, u.name 
		FROM users u 
		WHERE u.id = ?
	`

	var user models.UserBasic
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&user.ID, &user.Code, &user.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	// Get profile photo
	photoQuery := `
		SELECT url FROM images 
		WHERE imageable_type = 'App\\Models\\User' 
		AND imageable_id = ? 
		ORDER BY id DESC LIMIT 1
	`
	var photoURL string
	err = r.db.QueryRowContext(ctx, photoQuery, userID).Scan(&photoURL)
	if err == nil {
		user.ProfilePhoto = &photoURL
	}

	return &user, nil
}

// CreateChildPermission creates child permissions
func (r *JoinRequestRepository) CreateChildPermission(ctx context.Context, perm *models.ChildPermission) error {
	query := `
		INSERT INTO children_permissions 
		(user_id, verified, BFR, SF, W, JU, DM, PIUP, PITC, PIC, ESOO, COTB, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	_, err := r.db.ExecContext(ctx, query,
		perm.UserID, perm.Verified, perm.BFR, perm.SF, perm.W, perm.JU,
		perm.DM, perm.PIUP, perm.PITC, perm.PIC, perm.ESOO, perm.COTB,
	)
	if err != nil {
		return fmt.Errorf("failed to create child permission: %w", err)
	}

	return nil
}

// UpdateChildPermission updates child permissions
func (r *JoinRequestRepository) UpdateChildPermission(ctx context.Context, userID uint64, perm *models.ChildPermission) error {
	query := `
		UPDATE children_permissions 
		SET verified = ?, BFR = ?, SF = ?, W = ?, JU = ?, DM = ?, 
		    PIUP = ?, PITC = ?, PIC = ?, ESOO = ?, COTB = ?, updated_at = NOW()
		WHERE user_id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		perm.Verified, perm.BFR, perm.SF, perm.W, perm.JU, perm.DM,
		perm.PIUP, perm.PITC, perm.PIC, perm.ESOO, perm.COTB, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to update child permission: %w", err)
	}

	return nil
}

// GetChildPermission retrieves child permissions
func (r *JoinRequestRepository) GetChildPermission(ctx context.Context, userID uint64) (*models.ChildPermission, error) {
	query := `
		SELECT id, user_id, verified, BFR, SF, W, JU, DM, PIUP, PITC, PIC, ESOO, COTB, created_at, updated_at
		FROM children_permissions 
		WHERE user_id = ?
	`

	var perm models.ChildPermission
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&perm.ID, &perm.UserID, &perm.Verified, &perm.BFR, &perm.SF, &perm.W, &perm.JU,
		&perm.DM, &perm.PIUP, &perm.PITC, &perm.PIC, &perm.ESOO, &perm.COTB,
		&perm.CreatedAt, &perm.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get child permission: %w", err)
	}

	return &perm, nil
}

// GetDynastyPermission retrieves default dynasty permissions
func (r *JoinRequestRepository) GetDynastyPermission(ctx context.Context) (*models.DynastyPermission, error) {
	query := `
		SELECT id, BFR, SF, W, JU, DM, PIUP, PITC, PIC, ESOO, COTB, created_at, updated_at
		FROM dynasty_permissions 
		LIMIT 1
	`

	var perm models.DynastyPermission
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query).Scan(
		&perm.ID, &perm.BFR, &perm.SF, &perm.W, &perm.JU, &perm.DM,
		&perm.PIUP, &perm.PITC, &perm.PIC, &perm.ESOO, &perm.COTB,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty permission: %w", err)
	}

	// Convert sql.NullTime to *time.Time
	if createdAt.Valid {
		perm.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		perm.UpdatedAt = &updatedAt.Time
	}

	return &perm, nil
}

// CheckUserAge checks if user is under 18 based on birthdate
func (r *JoinRequestRepository) CheckUserAge(ctx context.Context, userID uint64) (bool, error) {
	query := `
		SELECT TIMESTAMPDIFF(YEAR, birthdate, CURDATE()) < 18 as is_under_18
		FROM kycs 
		WHERE user_id = ?
		LIMIT 1
	`

	var isUnder18 bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&isUnder18)
	if err == sql.ErrNoRows {
		return false, nil // Default to false if no KYC record
	}
	if err != nil {
		return false, fmt.Errorf("failed to check user age: %w", err)
	}

	return isUnder18, nil
}

// GetLatestRequest gets the latest join request between two users
func (r *JoinRequestRepository) GetLatestRequest(ctx context.Context, fromUser, toUser uint64) (*models.JoinRequest, error) {
	query := `SELECT id, from_user, to_user, status, relationship, message, created_at, updated_at 
	          FROM join_requests 
	          WHERE from_user = ? AND to_user = ?
	          ORDER BY created_at DESC 
	          LIMIT 1`

	var req models.JoinRequest
	err := r.db.QueryRowContext(ctx, query, fromUser, toUser).Scan(
		&req.ID,
		&req.FromUser,
		&req.ToUser,
		&req.Status,
		&req.Relationship,
		&req.Message,
		&req.CreatedAt,
		&req.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest request: %w", err)
	}

	return &req, nil
}
