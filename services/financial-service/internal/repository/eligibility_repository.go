package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type EligibilityRepository interface {
	GetUserBirthdate(ctx context.Context, userID uint64) (*time.Time, error)
	GetChildPermissions(ctx context.Context, userID uint64) (verified bool, bfr bool, found bool, err error)
}

type eligibilityRepository struct {
	db *sql.DB
}

func NewEligibilityRepository(db *sql.DB) EligibilityRepository {
	return &eligibilityRepository{db: db}
}

func (r *eligibilityRepository) GetUserBirthdate(ctx context.Context, userID uint64) (*time.Time, error) {
	var birthdate sql.NullTime
	err := r.db.QueryRowContext(ctx,
		"SELECT birthdate FROM kycs WHERE user_id = ?",
		userID,
	).Scan(&birthdate)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user birthdate: %w", err)
	}
	if !birthdate.Valid {
		return nil, nil
	}
	return &birthdate.Time, nil
}

func (r *eligibilityRepository) GetChildPermissions(ctx context.Context, userID uint64) (verified bool, bfr bool, found bool, err error) {
	var verifiedVal sql.NullBool
	var bfrVal sql.NullBool
	err = r.db.QueryRowContext(ctx,
		"SELECT verified, BFR FROM child_permissions WHERE user_id = ?",
		userID,
	).Scan(&verifiedVal, &bfrVal)
	if err == sql.ErrNoRows {
		return false, false, false, nil
	}
	if err != nil {
		return false, false, false, fmt.Errorf("failed to get child permissions: %w", err)
	}
	return verifiedVal.Bool, bfrVal.Bool, verifiedVal.Valid && bfrVal.Valid, nil
}
