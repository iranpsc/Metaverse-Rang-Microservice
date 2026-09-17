package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"metarang/auth-service/internal/repository"
)

// Must include the unread filter. A looser "SELECT COUNT" matcher would not catch
// regressions that drop read_at IS NULL or stop parameterizing notifiable_type.
const unreadNotificationsCountQuery = `(?s)SELECT COUNT\(\*\) FROM notifications\s+WHERE notifiable_type = \?\s+AND notifiable_id = \?\s+AND read_at IS NULL`

func TestUserRepository_GetUnreadNotificationsCount_SQLMock(t *testing.T) {
	ctx := context.Background()

	t.Run("counts unread rows for App\\User", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewUserRepository(db, "https://gw")

		mock.ExpectQuery(unreadNotificationsCountQuery).
			WithArgs("App\\User", uint64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int32(7)))

		n, err := repo.GetUnreadNotificationsCount(ctx, 42)
		require.NoError(t, err)
		require.Equal(t, int32(7), n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns zero when there are no unread notifications", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewUserRepository(db, "https://gw")

		mock.ExpectQuery(unreadNotificationsCountQuery).
			WithArgs("App\\User", uint64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int32(0)))

		n, err := repo.GetUnreadNotificationsCount(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, int32(0), n)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("wraps database errors", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewUserRepository(db, "https://gw")

		dbErr := errors.New("connection refused")
		mock.ExpectQuery(unreadNotificationsCountQuery).
			WithArgs("App\\User", uint64(9)).
			WillReturnError(dbErr)

		n, err := repo.GetUnreadNotificationsCount(ctx, 9)
		require.Error(t, err)
		require.Equal(t, int32(0), n)
		require.Contains(t, err.Error(), "failed to get unread notifications count")
		require.ErrorIs(t, err, dbErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
