package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"metarang/auth-service/internal/models"
)

const accountSecurityVerifiableType = "App\\Models\\AccountSecurity"

var ErrAccountSecurityChallengeChanged = errors.New("account security challenge changed")

type AccountSecurityRepository interface {
	GetByUserID(ctx context.Context, userID uint64) (*models.AccountSecurity, error)
	Create(ctx context.Context, security *models.AccountSecurity) error
	Update(ctx context.Context, security *models.AccountSecurity) error
	GetOtpByAccountSecurity(ctx context.Context, accountSecurityID uint64) (*models.Otp, error)
	UpsertOtp(ctx context.Context, otp *models.Otp) error
	CompleteVerification(ctx context.Context, otp *models.Otp, event *models.UserEvent) error
}

type accountSecurityRepository struct {
	db *sql.DB
}

func NewAccountSecurityRepository(db *sql.DB) AccountSecurityRepository {
	return &accountSecurityRepository{db: db}
}

func (r *accountSecurityRepository) GetByUserID(ctx context.Context, userID uint64) (*models.AccountSecurity, error) {
	query := `
		SELECT id, user_id, unlocked, until, length, last_activity, created_at, updated_at
		FROM account_securities
		WHERE user_id = ?
	`

	security := &models.AccountSecurity{}
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&security.ID,
		&security.UserID,
		&security.Unlocked,
		&security.Until,
		&security.Length,
		&security.LastActivity,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get account security: %w", err)
	}

	// Schema allows NULL timestamps (legacy Laravel rows); map to zero time when absent.
	if createdAt.Valid {
		security.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		security.UpdatedAt = updatedAt.Time
	}

	return security, nil
}

func (r *accountSecurityRepository) Create(ctx context.Context, security *models.AccountSecurity) error {
	now := time.Now()
	query := `
		INSERT INTO account_securities (user_id, unlocked, until, length, last_activity, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.ExecContext(ctx, query,
		security.UserID,
		security.Unlocked,
		security.Until,
		security.Length,
		security.LastActivity,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create account security: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get account security id: %w", err)
	}

	security.ID = uint64(id)
	security.CreatedAt = now
	security.UpdatedAt = now

	return nil
}

func (r *accountSecurityRepository) Update(ctx context.Context, security *models.AccountSecurity) error {
	now := time.Now()
	query := `
		UPDATE account_securities
		SET unlocked = ?, until = ?, length = ?, last_activity = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		security.Unlocked,
		security.Until,
		security.Length,
		security.LastActivity,
		now,
		security.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update account security: %w", err)
	}

	security.UpdatedAt = now
	return nil
}

func (r *accountSecurityRepository) GetOtpByAccountSecurity(ctx context.Context, accountSecurityID uint64) (*models.Otp, error) {
	query := `
		SELECT id, user_id, verifiable_type, verifiable_id, code, phone, created_at, updated_at
		FROM otps
		WHERE verifiable_type = ? AND verifiable_id = ?
		LIMIT 1
	`

	otp := &models.Otp{}
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, accountSecurityVerifiableType, accountSecurityID).Scan(
		&otp.ID,
		&otp.UserID,
		&otp.VerifiableType,
		&otp.VerifiableID,
		&otp.Code,
		&otp.Phone,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get otp: %w", err)
	}

	if createdAt.Valid {
		otp.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		otp.UpdatedAt = updatedAt.Time
	}

	return otp, nil
}

func (r *accountSecurityRepository) UpsertOtp(ctx context.Context, otp *models.Otp) error {
	now := time.Now()

	existing, err := r.GetOtpByAccountSecurity(ctx, otp.VerifiableID)
	if err != nil {
		return err
	}

	if existing == nil {
		query := `
			INSERT INTO otps (user_id, verifiable_type, verifiable_id, code, phone, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`
		result, err := r.db.ExecContext(ctx, query,
			otp.UserID,
			accountSecurityVerifiableType,
			otp.VerifiableID,
			otp.Code,
			otp.Phone,
			now,
			now,
		)
		if err != nil {
			return fmt.Errorf("failed to create otp: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get otp id: %w", err)
		}

		otp.ID = uint64(id)
		otp.VerifiableType = accountSecurityVerifiableType
		otp.CreatedAt = now
		otp.UpdatedAt = now

		return nil
	}

	query := `
		UPDATE otps
		SET code = ?, phone = ?, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, otp.Code, otp.Phone, now, existing.ID)
	if err != nil {
		return fmt.Errorf("failed to update otp: %w", err)
	}
	if err := requireAccountSecurityRow(result); err != nil {
		return err
	}

	otp.ID = existing.ID
	otp.VerifiableType = existing.VerifiableType
	otp.CreatedAt = existing.CreatedAt
	otp.UpdatedAt = now

	return nil
}

func (r *accountSecurityRepository) CompleteVerification(ctx context.Context, otp *models.Otp, event *models.UserEvent) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start verification transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Consume only the challenge that was checked. A resend can replace the code in the same row.
	result, err := tx.ExecContext(ctx, `DELETE FROM otps
		WHERE id = ? AND user_id = ? AND verifiable_type = ? AND verifiable_id = ?
		AND BINARY code = ? AND phone = ?`,
		otp.ID, otp.UserID, accountSecurityVerifiableType, otp.VerifiableID, otp.Code, otp.Phone)
	if err != nil {
		return fmt.Errorf("failed to consume otp: %w", err)
	}
	if err := requireAccountSecurityRow(result); err != nil {
		return err
	}

	var phone sql.NullString
	var verifiedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT phone, phone_verified_at FROM users WHERE id = ? FOR UPDATE`, otp.UserID).
		Scan(&phone, &verifiedAt)
	if err != nil {
		return fmt.Errorf("failed to lock user phone: %w", err)
	}
	now := time.Now()
	savedPhone := strings.TrimSpace(phone.String)
	if phone.Valid && savedPhone != "" && verifiedAt.Valid {
		if savedPhone != otp.Phone.String {
			return ErrAccountSecurityChallengeChanged
		}
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE users SET phone = ?, phone_verified_at = ?, updated_at = ? WHERE id = ?`,
			otp.Phone.String, now, now, otp.UserID)
		if err != nil {
			return fmt.Errorf("failed to save verified phone: %w", mapUserPhoneDuplicate(err))
		}
	}

	result, err = tx.ExecContext(ctx, `UPDATE account_securities
		SET unlocked = 1, until = ? + length, updated_at = ? WHERE id = ? AND user_id = ? AND unlocked = 0`,
		now.Unix(), now, otp.VerifiableID, otp.UserID)
	if err != nil {
		return fmt.Errorf("failed to unlock account security: %w", err)
	}
	if err := requireAccountSecurityRow(result); err != nil {
		return err
	}
	if err := createUserEvent(ctx, tx, event); err != nil {
		return fmt.Errorf("failed to record account security event: %w", err)
	}
	return tx.Commit()
}

func requireAccountSecurityRow(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to read affected rows: %w", err)
	}
	if count != 1 {
		return ErrAccountSecurityChallengeChanged
	}
	return nil
}
