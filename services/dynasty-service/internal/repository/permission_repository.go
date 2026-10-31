package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/dynasty-service/internal/models"
)

type PermissionRepository struct {
	db *sql.DB
}

func NewPermissionRepository(db *sql.DB) *PermissionRepository {
	return &PermissionRepository{db: db}
}

// GetByUserID retrieves child permissions by user ID
func (r *PermissionRepository) GetByUserID(ctx context.Context, userID uint64) (*models.ChildPermission, error) {
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
		return nil, fmt.Errorf("failed to get permission: %w", err)
	}

	return &perm, nil
}

// GetDefaultPermissions retrieves default dynasty permissions
func (r *PermissionRepository) GetDefaultPermissions(ctx context.Context) (*models.DynastyPermission, error) {
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
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no default permissions found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get default permissions: %w", err)
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

// CreatePermission creates child permissions
func (r *PermissionRepository) CreatePermission(ctx context.Context, perm *models.ChildPermission) error {
	query := `
		INSERT INTO children_permissions 
		(user_id, verified, BFR, SF, W, JU, DM, PIUP, PITC, PIC, ESOO, COTB, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		perm.UserID, perm.Verified, perm.BFR, perm.SF, perm.W, perm.JU,
		perm.DM, perm.PIUP, perm.PITC, perm.PIC, perm.ESOO, perm.COTB,
	)
	if err != nil {
		return fmt.Errorf("failed to create permission: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get permission ID: %w", err)
	}

	perm.ID = uint64(id)
	return nil
}

// UpdatePermission updates a single permission field
func (r *PermissionRepository) UpdatePermission(ctx context.Context, userID uint64, permissionKey string, status bool) error {
	allowedPermissions := map[string]struct{}{
		"BFR": {}, "SF": {}, "W": {}, "JU": {}, "DM": {},
		"PIUP": {}, "PITC": {}, "PIC": {}, "ESOO": {}, "COTB": {},
	}
	if _, ok := allowedPermissions[permissionKey]; !ok {
		return fmt.Errorf("invalid permission key %q", permissionKey)
	}

	query := fmt.Sprintf(`UPDATE children_permissions SET %s = ?, updated_at = NOW() WHERE user_id = ?`, permissionKey)

	_, err := r.db.ExecContext(ctx, query, status, userID)
	if err != nil {
		return fmt.Errorf("failed to update permission %s: %w", permissionKey, err)
	}

	return nil
}

// VerifyPermissions marks permissions as verified
func (r *PermissionRepository) VerifyPermissions(ctx context.Context, userID uint64) error {
	query := `UPDATE children_permissions SET verified = 1, updated_at = NOW() WHERE user_id = ?`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to verify permissions: %w", err)
	}

	return nil
}

// UpdateAllPermissions updates all permission fields
func (r *PermissionRepository) UpdateAllPermissions(ctx context.Context, perm *models.ChildPermission) error {
	query := `
		UPDATE children_permissions 
		SET verified = ?, BFR = ?, SF = ?, W = ?, JU = ?, DM = ?, 
		    PIUP = ?, PITC = ?, PIC = ?, ESOO = ?, COTB = ?, updated_at = NOW()
		WHERE user_id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		perm.Verified, perm.BFR, perm.SF, perm.W, perm.JU, perm.DM,
		perm.PIUP, perm.PITC, perm.PIC, perm.ESOO, perm.COTB, perm.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update permissions: %w", err)
	}

	return nil
}
