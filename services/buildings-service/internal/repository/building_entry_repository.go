package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"metarang/buildings-service/internal/models"
)

// BuildingEntryRepository persists entry configs, coupons, and visit sessions.
type BuildingEntryRepository struct {
	db *sql.DB
}

func NewBuildingEntryRepository(db *sql.DB) *BuildingEntryRepository {
	return &BuildingEntryRepository{db: db}
}

func (r *BuildingEntryRepository) UpsertConfig(ctx context.Context, config models.BuildingEntryConfig) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO building_entry_configs (
			feature_id, fee_psc, fee_irr, about, level_scope_type, level_slug, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			fee_psc = VALUES(fee_psc),
			fee_irr = VALUES(fee_irr),
			about = VALUES(about),
			level_scope_type = VALUES(level_scope_type),
			level_slug = VALUES(level_slug),
			is_active = VALUES(is_active),
			updated_at = NOW()
	`, config.FeatureID, config.FeePSC, config.FeeIRR, config.About,
		nullIfEmpty(config.LevelScopeType), nullIfEmpty(config.LevelSlug), boolToTiny(config.IsActive))
	if err != nil {
		return fmt.Errorf("failed to save entry config: %w", err)
	}
	return nil
}

func (r *BuildingEntryRepository) FindConfig(ctx context.Context, featureID uint64) (*models.BuildingEntryConfig, error) {
	var config models.BuildingEntryConfig
	var feePSC, feeIRR string
	var scopeType, levelSlug sql.NullString
	var active int

	err := r.db.QueryRowContext(ctx, `
		SELECT feature_id, CAST(fee_psc AS CHAR), CAST(fee_irr AS CHAR), about,
		       level_scope_type, level_slug, is_active
		FROM building_entry_configs
		WHERE feature_id = ?
		LIMIT 1
	`, featureID).Scan(&config.FeatureID, &feePSC, &feeIRR, &config.About, &scopeType, &levelSlug, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find entry config: %w", err)
	}
	config.FeePSC = feePSC
	config.FeeIRR = feeIRR
	if scopeType.Valid {
		config.LevelScopeType = scopeType.String
	}
	if levelSlug.Valid {
		config.LevelSlug = levelSlug.String
	}
	config.IsActive = active == 1
	return &config, nil
}

func (r *BuildingEntryRepository) CreateCoupon(ctx context.Context, coupon models.BuildingEntryCoupon) (*models.BuildingEntryCoupon, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO building_entry_coupons (
			feature_id, code, discount_percentage, max_usage_count, created_at, updated_at
		) VALUES (?, ?, ?, ?, NOW(), NOW())
	`, coupon.FeatureID, coupon.Code, coupon.DiscountPercentage, coupon.MaxUsageCount)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, fmt.Errorf("coupon code already exists")
		}
		return nil, fmt.Errorf("failed to create coupon: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to create coupon: %w", err)
	}
	created, err := r.FindCouponByID(ctx, coupon.FeatureID, uint64(id))
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, fmt.Errorf("failed to create coupon: coupon not found")
	}
	return created, nil
}

func (r *BuildingEntryRepository) ListCoupons(ctx context.Context, featureID uint64) ([]models.BuildingEntryCoupon, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.feature_id, c.code, c.discount_percentage, c.max_usage_count,
		       COUNT(s.id) AS real_usage_count
		FROM building_entry_coupons c
		LEFT JOIN building_entry_sessions s ON s.coupon_id = c.id
		WHERE c.feature_id = ?
		GROUP BY c.id, c.feature_id, c.code, c.discount_percentage, c.max_usage_count
		ORDER BY c.id ASC
	`, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to list coupons: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]models.BuildingEntryCoupon, 0)
	for rows.Next() {
		coupon, err := scanCoupon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, coupon)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list coupons: %w", err)
	}
	return out, nil
}

func (r *BuildingEntryRepository) FindCouponByCode(ctx context.Context, featureID uint64, code string) (*models.BuildingEntryCoupon, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.feature_id, c.code, c.discount_percentage, c.max_usage_count,
		       COUNT(s.id) AS real_usage_count
		FROM building_entry_coupons c
		LEFT JOIN building_entry_sessions s ON s.coupon_id = c.id
		WHERE c.feature_id = ? AND c.code = ?
		GROUP BY c.id, c.feature_id, c.code, c.discount_percentage, c.max_usage_count
		LIMIT 1
	`, featureID, code)
	coupon, err := scanCoupon(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find coupon: %w", err)
	}
	return &coupon, nil
}

func (r *BuildingEntryRepository) FindCouponByID(ctx context.Context, featureID, couponID uint64) (*models.BuildingEntryCoupon, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.feature_id, c.code, c.discount_percentage, c.max_usage_count,
		       COUNT(s.id) AS real_usage_count
		FROM building_entry_coupons c
		LEFT JOIN building_entry_sessions s ON s.coupon_id = c.id
		WHERE c.feature_id = ? AND c.id = ?
		GROUP BY c.id, c.feature_id, c.code, c.discount_percentage, c.max_usage_count
		LIMIT 1
	`, featureID, couponID)
	coupon, err := scanCoupon(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find coupon: %w", err)
	}
	return &coupon, nil
}

// FindCurrentAccess returns the latest visit window that started after validAfter.
// Inside is true when the guest has not exited that window.
func (r *BuildingEntryRepository) FindCurrentAccess(ctx context.Context, featureID, userID uint64, validAfter time.Time) (*models.BuildingEntryAccess, error) {
	var access models.BuildingEntryAccess
	var exitedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, exited_at
		FROM building_entry_sessions
		WHERE feature_id = ? AND user_id = ? AND entered_at > ?
		ORDER BY entered_at DESC, id DESC
		LIMIT 1
	`, featureID, userID, validAfter).Scan(&access.ID, &exitedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find entry window: %w", err)
	}
	access.Inside = !exitedAt.Valid
	return &access, nil
}

// WithUserLock serializes entry and exit for one guest. Wallet calls belong inside fn,
// before any session insert, so two overlapping enters cannot both charge.
func (r *BuildingEntryRepository) WithUserLock(ctx context.Context, featureID, userID uint64, fn func(context.Context) error) error {
	return r.withEntryLock(ctx, featureID, userID, fn)
}

// ReopenSession lets the guest back in without a new fee while the paid window is still valid.
func (r *BuildingEntryRepository) ReopenSession(ctx context.Context, featureID, userID, sessionID uint64, validAfter time.Time) error {
	return r.withEntryLock(ctx, featureID, userID, func(ctx context.Context) error {
		return r.reopenSession(ctx, featureID, userID, sessionID, validAfter)
	})
}

func (r *BuildingEntryRepository) reopenSession(ctx context.Context, featureID, userID, sessionID uint64, validAfter time.Time) error {
	return r.withTx(ctx, func(tx *sql.Tx) error {
		var exitedAt sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT exited_at
			FROM building_entry_sessions
			WHERE id = ? AND feature_id = ? AND user_id = ? AND entered_at > ?
			LIMIT 1
			FOR UPDATE
		`, sessionID, featureID, userID, validAfter).Scan(&exitedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("entry window expired")
		}
		if err != nil {
			return fmt.Errorf("failed to reopen entry: %w", err)
		}
		if !exitedAt.Valid {
			return fmt.Errorf("user already inside this building")
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE building_entry_sessions
			SET exited_at = NULL, updated_at = NOW()
			WHERE id = ? AND feature_id = ? AND user_id = ?
		`, sessionID, featureID, userID); err != nil {
			return fmt.Errorf("failed to reopen entry: %w", err)
		}
		return nil
	})
}

// CreateSession records a new paid window and returns its id.
// An unexpired window is left in place. Open visits older than validAfter are closed
// so the new window is the only open one.
func (r *BuildingEntryRepository) CreateSession(ctx context.Context, session models.BuildingEntrySession, validAfter time.Time) (uint64, error) {
	var id uint64
	err := r.withEntryLock(ctx, session.FeatureID, session.UserID, func(ctx context.Context) error {
		created, err := r.insertSession(ctx, session, validAfter)
		if err != nil {
			return err
		}
		id = created
		return nil
	})
	return id, err
}

func (r *BuildingEntryRepository) insertSession(ctx context.Context, session models.BuildingEntrySession, validAfter time.Time) (uint64, error) {
	var id uint64
	err := r.withTx(ctx, func(tx *sql.Tx) error {
		var existing uint64
		var exitedAt sql.NullTime
		err := tx.QueryRowContext(ctx, `
			SELECT id, exited_at
			FROM building_entry_sessions
			WHERE feature_id = ? AND user_id = ? AND entered_at > ?
			ORDER BY entered_at DESC, id DESC
			LIMIT 1
			FOR UPDATE
		`, session.FeatureID, session.UserID, validAfter).Scan(&existing, &exitedAt)
		if err == nil {
			if !exitedAt.Valid {
				return fmt.Errorf("user already inside this building")
			}
			return fmt.Errorf("entry window still open")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("failed to check entry window: %w", err)
		}

		if _, err = tx.ExecContext(ctx, `
			UPDATE building_entry_sessions
			SET exited_at = NOW(), updated_at = NOW()
			WHERE feature_id = ? AND user_id = ? AND exited_at IS NULL AND entered_at <= ?
		`, session.FeatureID, session.UserID, validAfter); err != nil {
			return fmt.Errorf("failed to close expired entry: %w", err)
		}

		if session.CouponID != nil {
			var maxUsage int
			err = tx.QueryRowContext(ctx, `
				SELECT max_usage_count
				FROM building_entry_coupons
				WHERE id = ? AND feature_id = ?
				FOR UPDATE
			`, *session.CouponID, session.FeatureID).Scan(&maxUsage)
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("coupon not found")
			}
			if err != nil {
				return fmt.Errorf("failed to lock coupon: %w", err)
			}
			var used int
			if err = tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM building_entry_sessions WHERE coupon_id = ?
			`, *session.CouponID).Scan(&used); err != nil {
				return fmt.Errorf("failed to count coupon usage: %w", err)
			}
			if used >= maxUsage {
				return fmt.Errorf("coupon usage limit reached")
			}
		}

		var couponID interface{}
		if session.CouponID != nil {
			couponID = *session.CouponID
		}
		feePSC := session.FeePSC
		feeIRR := session.FeeIRR
		if feePSC == "" {
			feePSC = "0.00"
		}
		if feeIRR == "" {
			feeIRR = "0.00"
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO building_entry_sessions (
				feature_id, user_id, fee_psc_paid, fee_irr_paid, coupon_id, entered_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, NOW(), NOW(), NOW())
		`, session.FeatureID, session.UserID, feePSC, feeIRR, couponID)
		if err != nil {
			return fmt.Errorf("failed to create entry session: %w", err)
		}
		inserted, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to create entry session: %w", err)
		}
		id = uint64(inserted)
		return nil
	})
	return id, err
}

// DeleteSession removes a visit that was recorded but could not be kept,
// for example when the wallet ledger write failed and the fee was reversed.
func (r *BuildingEntryRepository) DeleteSession(ctx context.Context, featureID, userID, sessionID uint64) error {
	return r.withEntryLock(ctx, featureID, userID, func(ctx context.Context) error {
		return r.withTx(ctx, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx, `
				DELETE FROM building_entry_sessions
				WHERE id = ? AND feature_id = ? AND user_id = ?
			`, sessionID, featureID, userID)
			if err != nil {
				return fmt.Errorf("failed to delete entry session: %w", err)
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("failed to delete entry session: %w", err)
			}
			if rows == 0 {
				return fmt.Errorf("entry session not found")
			}
			return nil
		})
	})
}

// CloseActiveSession marks the open visit as exited.
func (r *BuildingEntryRepository) CloseActiveSession(ctx context.Context, featureID, userID uint64) error {
	return r.withEntryLock(ctx, featureID, userID, func(ctx context.Context) error {
		return r.withTx(ctx, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx, `
				UPDATE building_entry_sessions
				SET exited_at = NOW(), updated_at = NOW()
				WHERE feature_id = ? AND user_id = ? AND exited_at IS NULL
			`, featureID, userID)
			if err != nil {
				return fmt.Errorf("failed to exit building: %w", err)
			}
			rows, err := result.RowsAffected()
			if err != nil {
				return fmt.Errorf("failed to exit building: %w", err)
			}
			if rows == 0 {
				return fmt.Errorf("active session not found")
			}
			return nil
		})
	})
}

type couponScanner interface {
	Scan(dest ...interface{}) error
}

func scanCoupon(row couponScanner) (models.BuildingEntryCoupon, error) {
	var coupon models.BuildingEntryCoupon
	err := row.Scan(
		&coupon.ID,
		&coupon.FeatureID,
		&coupon.Code,
		&coupon.DiscountPercentage,
		&coupon.MaxUsageCount,
		&coupon.RealUsageCount,
	)
	return coupon, err
}

func entryLockName(featureID, userID uint64) string {
	return fmt.Sprintf("buildings:entry:%d:%d", featureID, userID)
}

func nullIfEmpty(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func boolToTiny(value bool) int {
	if value {
		return 1
	}
	return 0
}

func isDuplicateKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

type entryLockCtxKey struct{}

func (r *BuildingEntryRepository) withEntryLock(ctx context.Context, featureID, userID uint64, fn func(context.Context) error) error {
	lockName := entryLockName(featureID, userID)
	if held, _ := ctx.Value(entryLockCtxKey{}).(string); held == lockName {
		return fn(ctx)
	}

	conn, err := r.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("lock building entry: %w", err)
	}
	defer func() { _ = conn.Close() }()

	var locked sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", lockName).Scan(&locked); err != nil {
		return fmt.Errorf("lock building entry: %w", err)
	}
	if locked.Int64 != 1 {
		return fmt.Errorf("timed out locking building entry")
	}
	defer func() {
		var released sql.NullInt64
		_ = conn.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName).Scan(&released)
	}()

	ctx = context.WithValue(ctx, entryLockCtxKey{}, lockName)
	return fn(ctx)
}

func (r *BuildingEntryRepository) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin entry transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit entry transaction: %w", err)
	}
	committed = true
	return nil
}
