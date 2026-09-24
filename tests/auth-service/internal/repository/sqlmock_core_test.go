package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

func TestAccountSecurityRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewAccountSecurityRepository(db)
	ctx := context.Background()
	now := time.Now()

	t.Run("get not found", func(t *testing.T) {
		mock.ExpectQuery("FROM account_securities").
			WithArgs(uint64(1)).
			WillReturnError(sql.ErrNoRows)
		sec, err := repo.GetByUserID(ctx, 1)
		require.NoError(t, err)
		require.Nil(t, sec)
	})

	t.Run("get success", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "user_id", "unlocked", "until", "length", "last_activity", "created_at", "updated_at"}).
			AddRow(uint64(5), uint64(1), true, int64(123), int64(15), int64(456), now, now)
		mock.ExpectQuery("FROM account_securities").WithArgs(uint64(1)).WillReturnRows(rows)
		sec, err := repo.GetByUserID(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(5), sec.ID)
	})

	t.Run("get success with null timestamps", func(t *testing.T) {
		rows := sqlmock.NewRows([]string{"id", "user_id", "unlocked", "until", "length", "last_activity", "created_at", "updated_at"}).
			AddRow(uint64(6), uint64(2), false, nil, int64(900), nil, nil, nil)
		mock.ExpectQuery("FROM account_securities").WithArgs(uint64(2)).WillReturnRows(rows)
		sec, err := repo.GetByUserID(ctx, 2)
		require.NoError(t, err)
		require.NotNil(t, sec)
		require.Equal(t, uint64(6), sec.ID)
		require.False(t, sec.Unlocked)
		require.False(t, sec.Until.Valid)
		require.False(t, sec.LastActivity.Valid)
		require.True(t, sec.CreatedAt.IsZero())
		require.True(t, sec.UpdatedAt.IsZero())
	})

	t.Run("create update delete otp upsert", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO account_securities").
			WillReturnResult(sqlmock.NewResult(5, 1))
		sec := &models.AccountSecurity{UserID: 1, Unlocked: false, Length: 10}
		require.NoError(t, repo.Create(ctx, sec))
		require.Equal(t, uint64(5), sec.ID)

		mock.ExpectExec("UPDATE account_securities").
			WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.Update(ctx, sec))

		mock.ExpectQuery("FROM otps").
			WithArgs("App\\Models\\AccountSecurity", uint64(5)).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec("INSERT INTO otps").
			WillReturnResult(sqlmock.NewResult(9, 1))
		otp := &models.Otp{UserID: 1, VerifiableID: 5, Code: "123456"}
		require.NoError(t, repo.UpsertOtp(ctx, otp))
		require.Equal(t, uint64(9), otp.ID)

		otpRows := sqlmock.NewRows([]string{"id", "user_id", "verifiable_type", "verifiable_id", "code", "created_at", "updated_at"}).
			AddRow(uint64(9), uint64(1), "App\\Models\\AccountSecurity", uint64(5), "111111", now, now)
		mock.ExpectQuery("FROM otps").
			WithArgs("App\\Models\\AccountSecurity", uint64(5)).
			WillReturnRows(otpRows)
		mock.ExpectExec("UPDATE otps").
			WillReturnResult(sqlmock.NewResult(0, 1))
		otp2 := &models.Otp{UserID: 1, VerifiableID: 5, Code: "654321"}
		require.NoError(t, repo.UpsertOtp(ctx, otp2))

		nullOtpRows := sqlmock.NewRows([]string{"id", "user_id", "verifiable_type", "verifiable_id", "code", "created_at", "updated_at"}).
			AddRow(uint64(10), uint64(1), "App\\Models\\AccountSecurity", uint64(5), "222222", nil, nil)
		mock.ExpectQuery("FROM otps").
			WithArgs("App\\Models\\AccountSecurity", uint64(5)).
			WillReturnRows(nullOtpRows)
		otpWithNullTS, err := repo.GetOtpByAccountSecurity(ctx, 5)
		require.NoError(t, err)
		require.NotNil(t, otpWithNullTS)
		require.Equal(t, uint64(10), otpWithNullTS.ID)
		require.True(t, otpWithNullTS.CreatedAt.IsZero())
		require.True(t, otpWithNullTS.UpdatedAt.IsZero())

		mock.ExpectExec("DELETE FROM otps").
			WithArgs(uint64(9)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.DeleteOtp(ctx, 9))
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func tokenValidateColumns() []string {
	return []string{
		"pat.id", "pat.tokenable_id", "expires_at", "last_used_at", "wallet_login",
		"u.id", "name", "email", "phone", "password", "code", "referrer_id", "score", "ip",
		"last_seen", "email_verified_at", "phone_verified_at", "access_token",
		"refresh_token", "token_type", "expires_in", "wallet_address", "created_at", "updated_at",
	}
}

func waitSQLMockExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		lastErr = mock.ExpectationsWereMet()
		if lastErr == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.NoError(t, lastErr)
}

func TestTokenRepository_SQLMock(t *testing.T) {
	// Each subtest uses its own sqlmock DB so the async last_used_at UPDATE from
	// ValidateToken cannot race into later cases (flaky on slow CI).
	ctx := context.Background()
	now := time.Now()
	future := now.Add(time.Hour)

	t.Run("create", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewTokenRepository(db)

		mock.ExpectExec("INSERT INTO personal_access_tokens").
			WillReturnResult(sqlmock.NewResult(7, 1))
		tok, err := repo.Create(ctx, 1, "auth", future, true)
		require.NoError(t, err)
		require.Contains(t, tok, "7|")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("validate invalid", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewTokenRepository(db)

		mock.ExpectQuery("FROM personal_access_tokens pat").
			WillReturnError(sql.ErrNoRows)
		_, err = repo.ValidateToken(ctx, "plain")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid token")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("validate success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewTokenRepository(db)

		rows := sqlmock.NewRows(tokenValidateColumns()).AddRow(
			uint64(7), uint64(1), future, nil, true,
			uint64(1), "n", "e@x.com", nil, "hash", "c1", nil, int32(0), "1.1.1.1",
			nil, nil, nil, nil, nil, nil, nil, nil, now, now,
		)
		mock.ExpectQuery("FROM personal_access_tokens pat").WillReturnRows(rows)
		mock.ExpectExec("UPDATE personal_access_tokens SET last_used_at").
			WillReturnResult(sqlmock.NewResult(0, 1))
		user, err := repo.ValidateToken(ctx, "7|abcdef")
		require.NoError(t, err)
		require.Equal(t, uint64(1), user.ID)
		waitSQLMockExpectations(t, mock)

		db2, mock2, err := sqlmock.New()
		require.NoError(t, err)
		defer db2.Close()
		repo2 := repository.NewTokenRepository(db2)
		rows2 := sqlmock.NewRows(tokenValidateColumns()).AddRow(
			uint64(7), uint64(1), future, nil, true,
			uint64(1), "n", "e@x.com", nil, "hash", "c1", nil, int32(0), "1.1.1.1",
			nil, nil, nil, nil, nil, nil, nil, nil, now, now,
		)
		mock2.ExpectQuery("FROM personal_access_tokens pat").WillReturnRows(rows2)
		mock2.ExpectExec("UPDATE personal_access_tokens SET last_used_at").
			WillReturnResult(sqlmock.NewResult(0, 1))
		session, err := repo2.ValidateTokenSession(ctx, "7|abcdef")
		require.NoError(t, err)
		require.True(t, session.WalletLogin)
		waitSQLMockExpectations(t, mock2)
	})

	t.Run("validate expired", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewTokenRepository(db)

		past := time.Now().Add(-time.Hour)
		rows := sqlmock.NewRows(tokenValidateColumns()).AddRow(
			uint64(7), uint64(1), past, nil, false,
			uint64(1), "n", "e@x.com", nil, "hash", "c1", nil, int32(0), "1.1.1.1",
			nil, nil, nil, nil, nil, nil, nil, nil, now, now,
		)
		mock.ExpectQuery("FROM personal_access_tokens pat").WillReturnRows(rows)
		_, err = repo.ValidateToken(ctx, "abcdef")
		require.Error(t, err)
		require.Contains(t, err.Error(), "token expired")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("delete and find", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewTokenRepository(db)

		mock.ExpectExec("DELETE FROM personal_access_tokens").
			WithArgs(uint64(1)).
			WillReturnResult(sqlmock.NewResult(0, 2))
		require.NoError(t, repo.DeleteUserTokens(ctx, 1))

		mock.ExpectQuery("FROM personal_access_tokens").
			WithArgs("hash").
			WillReturnError(sql.ErrNoRows)
		tok, err := repo.FindTokenByHash(ctx, "hash")
		require.NoError(t, err)
		require.Nil(t, tok)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUserRepository_SQLMockBasics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewUserRepository(db, "https://admin.example")
	ctx := context.Background()
	now := time.Now()

	userCols := []string{
		"id", "name", "email", "phone", "password", "code", "referrer_id", "score", "ip",
		"last_seen", "email_verified_at", "phone_verified_at", "access_token",
		"refresh_token", "token_type", "expires_in", "wallet_address", "created_at", "updated_at",
	}

	t.Run("find by id", func(t *testing.T) {
		rows := sqlmock.NewRows(userCols).AddRow(
			uint64(1), "n", "e@x.com", nil, "hash", "c1", nil, int32(10), "1.1.1.1",
			nil, nil, nil, nil, nil, nil, nil, nil, now, now,
		)
		mock.ExpectQuery("FROM users").WithArgs(uint64(1)).WillReturnRows(rows)
		u, err := repo.FindByID(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, "e@x.com", u.Email)

		mock.ExpectQuery("FROM users").WithArgs(uint64(99)).WillReturnError(sql.ErrNoRows)
		u, err = repo.FindByID(ctx, 99)
		require.NoError(t, err)
		require.Nil(t, u)
	})

	t.Run("find by email create update", func(t *testing.T) {
		mock.ExpectQuery("FROM users").WithArgs("e@x.com").WillReturnError(sql.ErrNoRows)
		u, err := repo.FindByEmail(ctx, "e@x.com")
		require.NoError(t, err)
		require.Nil(t, u)

		mock.ExpectExec("INSERT INTO users").WillReturnResult(sqlmock.NewResult(42, 1))
		user := &models.User{Name: "n", Email: "e@x.com", Password: "pw", Code: "c", IP: "ip"}
		require.NoError(t, repo.Create(ctx, user))
		require.Equal(t, uint64(42), user.ID)

		mock.ExpectExec("UPDATE users SET name").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.Update(ctx, user))

		mock.ExpectExec("UPDATE users SET last_seen").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.UpdateLastSeen(ctx, 42))
	})

	t.Run("phone wallet helpers", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE phone").
			WithArgs("0912", uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
		taken, err := repo.IsPhoneTaken(ctx, "0912", 1)
		require.NoError(t, err)
		require.False(t, taken)

		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE wallet_address").
			WithArgs("0xabc").
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
		exists, err := repo.ExistsByWalletAddress(ctx, "0xabc", 0)
		require.NoError(t, err)
		require.True(t, exists)

		mock.ExpectExec("UPDATE users SET phone").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.UpdatePhone(ctx, 1, "0912"))
		mock.ExpectExec("UPDATE users SET phone_verified_at").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.MarkPhoneAsVerified(ctx, 1))
		mock.ExpectExec("UPDATE users SET email_verified_at").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.MarkEmailAsVerified(ctx, 1))
	})

	t.Run("find by wallet address", func(t *testing.T) {
		rows := sqlmock.NewRows(userCols).AddRow(
			uint64(7), "n", "e@x.com", nil, "hash", "hm-123", nil, int32(10), "1.1.1.1",
			nil, nil, nil, nil, nil, nil, nil, "0xtf", now, now,
		)
		mock.ExpectQuery("FROM users").WithArgs("0xtf").WillReturnRows(rows)
		u, err := repo.FindByWalletAddress(ctx, "0xtf")
		require.NoError(t, err)
		require.Equal(t, "hm-123", u.Code)

		mock.ExpectQuery("FROM users").WithArgs("0xmissing").WillReturnError(sql.ErrNoRows)
		u, err = repo.FindByWalletAddress(ctx, "0xmissing")
		require.NoError(t, err)
		require.Nil(t, u)

		rows = sqlmock.NewRows(userCols).AddRow(
			uint64(8), "wallet user", nil, nil, "hash", "hm-456", nil, int32(0), "1.1.1.1",
			nil, nil, nil, nil, nil, nil, nil, "0xnullmail", now, now,
		)
		mock.ExpectQuery("FROM users").WithArgs("0xnullmail").WillReturnRows(rows)
		u, err = repo.FindByWalletAddress(ctx, "0xnullmail")
		require.NoError(t, err)
		require.Equal(t, "hm-456", u.Code)
		require.Empty(t, u.Email)
	})

	t.Run("link wallet", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT wallet_address FROM users WHERE id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"wallet_address"}).AddRow(nil))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE wallet_address").
			WithArgs("0xabc").
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
		mock.ExpectExec("UPDATE users SET wallet_address").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		res, err := repo.LinkWalletAddress(ctx, 1, "0xabc")
		require.NoError(t, err)
		require.Equal(t, repository.LinkWalletSuccess, res)

		mock.ExpectBegin()
		mock.ExpectQuery("SELECT wallet_address FROM users WHERE id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"wallet_address"}).AddRow("0xold"))
		mock.ExpectRollback()
		res, err = repo.LinkWalletAddress(ctx, 1, "0xabc")
		require.NoError(t, err)
		require.Equal(t, repository.LinkWalletAlreadyConnected, res)
	})

	t.Run("counts and photos", func(t *testing.T) {
		mock.ExpectQuery("FROM follows WHERE following_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
		c, err := repo.GetFollowersCount(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, int32(3), c)

		mock.ExpectQuery("FROM follows WHERE follower_id").
			WithArgs(uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(2))
		c, err = repo.GetFollowingCount(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, int32(2), c)

		mock.ExpectQuery("FROM images WHERE imageable_type").
			WithArgs(uint64(1)).
			WillReturnError(sql.ErrNoRows)
		url, err := repo.GetLatestProfilePhotoURL(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, "", url)

		mock.ExpectQuery("FROM kycs WHERE user_id").
			WithArgs(uint64(1)).
			WillReturnError(sql.ErrNoRows)
		kyc, err := repo.GetKYC(ctx, 1)
		require.NoError(t, err)
		require.Nil(t, kyc)
	})
}
