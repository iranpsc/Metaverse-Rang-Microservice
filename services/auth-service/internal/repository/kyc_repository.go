package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/auth-service/internal/models"
)

type KYCRepository interface {
	Create(ctx context.Context, kyc *models.KYC) error
	FindByUserID(ctx context.Context, userID uint64) (*models.KYC, error)
	Update(ctx context.Context, kyc *models.KYC) error
	CheckUniqueMelliCode(ctx context.Context, melliCode string, excludeUserID uint64) (bool, error)
	CheckVerifyTextExists(ctx context.Context, verifyTextID uint64) (bool, error)
	CreateBankAccount(ctx context.Context, bankAccount *models.BankAccount) error
	FindBankAccountsByUserID(ctx context.Context, userID uint64) ([]*models.BankAccount, error)
	FindBankAccountByID(ctx context.Context, bankAccountID uint64) (*models.BankAccount, error)
	UpdateBankAccount(ctx context.Context, bankAccount *models.BankAccount) error
	DeleteBankAccount(ctx context.Context, bankAccountID uint64) error
	CheckUniqueShaba(ctx context.Context, shabaNum string, excludeID uint64) (bool, error)
	CheckUniqueCard(ctx context.Context, cardNum string, excludeID uint64) (bool, error)
}

type kycRepository struct {
	db *sql.DB
}

func NewKYCRepository(db *sql.DB) KYCRepository {
	return &kycRepository{db: db}
}

func (r *kycRepository) Create(ctx context.Context, kyc *models.KYC) error {
	query := `
		INSERT INTO kycs (user_id, fname, lname, melli_code, melli_card, video, verify_text_id, province, gender, status, birthdate, errors, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		kyc.UserID, kyc.Fname, kyc.Lname, kyc.MelliCode, kyc.MelliCard,
		kyc.Video, kyc.VerifyTextID, kyc.Province, kyc.Gender, kyc.Status,
		kyc.Birthdate, kyc.Errors, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create kyc: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	kyc.ID = uint64(id)

	return nil
}

func (r *kycRepository) FindByUserID(ctx context.Context, userID uint64) (*models.KYC, error) {
	// Note: fname and lname are read as-is; charset is handled at connection level
	query := `
		SELECT id, user_id, fname, lname, melli_code, melli_card, video, verify_text_id, province, gender, status, birthdate, errors, created_at, updated_at
		FROM kycs
		WHERE user_id = ?
	`
	kyc := &models.KYC{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&kyc.ID, &kyc.UserID, &kyc.Fname, &kyc.Lname, &kyc.MelliCode,
		&kyc.MelliCard, &kyc.Video, &kyc.VerifyTextID, &kyc.Province, &kyc.Gender,
		&kyc.Status, &kyc.Birthdate, &kyc.Errors, &kyc.CreatedAt, &kyc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find kyc: %w", err)
	}
	return kyc, nil
}

func (r *kycRepository) Update(ctx context.Context, kyc *models.KYC) error {
	query := `
		UPDATE kycs
		SET fname = ?, lname = ?, melli_code = ?, melli_card = ?, video = ?, verify_text_id = ?, province = ?, gender = ?, status = ?, birthdate = ?, errors = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		kyc.Fname, kyc.Lname, kyc.MelliCode, kyc.MelliCard, kyc.Video,
		kyc.VerifyTextID, kyc.Province, kyc.Gender, kyc.Status, kyc.Birthdate,
		kyc.Errors, time.Now(), kyc.ID)
	if err != nil {
		return fmt.Errorf("failed to update kyc: %w", err)
	}
	return nil
}

func (r *kycRepository) CreateBankAccount(ctx context.Context, bankAccount *models.BankAccount) error {
	query := `
		INSERT INTO bank_accounts (bankable_type, bankable_id, bank_name, shaba_num, card_num, status, errors, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query,
		bankAccount.BankableType, bankAccount.BankableID, bankAccount.BankName,
		bankAccount.ShabaNum, bankAccount.CardNum, bankAccount.Status,
		bankAccount.Errors, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create bank account: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	bankAccount.ID = uint64(id)

	return nil
}

func (r *kycRepository) FindBankAccountsByUserID(ctx context.Context, userID uint64) ([]*models.BankAccount, error) {
	query := `
		SELECT id, bankable_type, bankable_id, bank_name, shaba_num, card_num, status, errors, created_at, updated_at
		FROM bank_accounts
		WHERE bankable_type = 'App\\Models\\User' AND bankable_id = ?
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to find bank accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var accounts []*models.BankAccount
	for rows.Next() {
		account := &models.BankAccount{}
		err := rows.Scan(
			&account.ID, &account.BankableType, &account.BankableID,
			&account.BankName, &account.ShabaNum, &account.CardNum,
			&account.Status, &account.Errors, &account.CreatedAt, &account.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bank account: %w", err)
		}
		accounts = append(accounts, account)
	}

	return accounts, nil
}

func (r *kycRepository) FindBankAccountByID(ctx context.Context, bankAccountID uint64) (*models.BankAccount, error) {
	query := `
		SELECT id, bankable_type, bankable_id, bank_name, shaba_num, card_num, status, errors, created_at, updated_at
		FROM bank_accounts
		WHERE id = ?
	`
	account := &models.BankAccount{}
	err := r.db.QueryRowContext(ctx, query, bankAccountID).Scan(
		&account.ID, &account.BankableType, &account.BankableID,
		&account.BankName, &account.ShabaNum, &account.CardNum,
		&account.Status, &account.Errors, &account.CreatedAt, &account.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find bank account: %w", err)
	}
	return account, nil
}

func (r *kycRepository) UpdateBankAccount(ctx context.Context, bankAccount *models.BankAccount) error {
	query := `
		UPDATE bank_accounts
		SET bank_name = ?, shaba_num = ?, card_num = ?, status = ?, errors = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		bankAccount.BankName, bankAccount.ShabaNum, bankAccount.CardNum,
		bankAccount.Status, bankAccount.Errors, time.Now(), bankAccount.ID)
	if err != nil {
		return fmt.Errorf("failed to update bank account: %w", err)
	}
	return nil
}

func (r *kycRepository) DeleteBankAccount(ctx context.Context, bankAccountID uint64) error {
	query := `DELETE FROM bank_accounts WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, bankAccountID)
	if err != nil {
		return fmt.Errorf("failed to delete bank account: %w", err)
	}
	return nil
}

func (r *kycRepository) CheckUniqueShaba(ctx context.Context, shabaNum string, excludeID uint64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM bank_accounts WHERE shaba_num = ? AND id != ?`
	err := r.db.QueryRowContext(ctx, query, shabaNum, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check unique shaba: %w", err)
	}
	return count == 0, nil
}

func (r *kycRepository) CheckUniqueCard(ctx context.Context, cardNum string, excludeID uint64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM bank_accounts WHERE card_num = ? AND id != ?`
	err := r.db.QueryRowContext(ctx, query, cardNum, excludeID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check unique card: %w", err)
	}
	return count == 0, nil
}

func (r *kycRepository) CheckUniqueMelliCode(ctx context.Context, melliCode string, excludeUserID uint64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM kycs WHERE melli_code = ? AND user_id != ?`
	err := r.db.QueryRowContext(ctx, query, melliCode, excludeUserID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check unique melli code: %w", err)
	}
	return count == 0, nil
}

func (r *kycRepository) CheckVerifyTextExists(ctx context.Context, verifyTextID uint64) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM kyc_verify_texts WHERE id = ?`
	err := r.db.QueryRowContext(ctx, query, verifyTextID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check verify_text_id existence: %w", err)
	}
	return count > 0, nil
}
