package repository_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

func TestResetRepository_SQLMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	repo := repository.NewResetRepository(db)
	ctx := context.Background()

	t.Run("create unverified mobile reset", func(t *testing.T) {
		mock.ExpectExec("INSERT INTO resets").
			WithArgs(uint64(1), models.ResetTypeMobile, "09121112233", false, sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(17, 1))

		reset := &models.Reset{
			UserID:   1,
			Type:     models.ResetTypeMobile,
			Value:    "09121112233",
			Verified: false,
		}
		require.NoError(t, repo.Create(ctx, reset))
		require.Equal(t, uint64(17), reset.ID)
	})

	t.Run("count verified by user and type", func(t *testing.T) {
		mock.ExpectQuery("SELECT COUNT").
			WithArgs(uint64(1), models.ResetTypeMobile).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

		count, err := repo.CountVerifiedByUserAndType(ctx, 1, models.ResetTypeMobile)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})

	t.Run("mark verified", func(t *testing.T) {
		mock.ExpectExec("UPDATE resets").
			WithArgs(sqlmock.AnyArg(), uint64(17)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.MarkVerified(ctx, 17))
	})

	t.Run("delete", func(t *testing.T) {
		mock.ExpectExec("DELETE FROM resets").
			WithArgs(uint64(17)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.Delete(ctx, 17))
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
