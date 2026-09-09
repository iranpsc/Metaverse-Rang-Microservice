package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/repository"
)

func TestRepository_NilDB(t *testing.T) {
	repo := repository.NewNotificationRepository(nil)
	ctx := context.Background()

	_, err := repo.CreateNotification(ctx, &models.Notification{Type: "system"})
	assert.Error(t, err)

	_, _, err = repo.ListNotifications(ctx, 1, models.NotificationFilter{})
	assert.Error(t, err)

	err = repo.MarkAsRead(ctx, "n1", 1)
	assert.Error(t, err)

	err = repo.MarkAllAsRead(ctx, 1)
	assert.Error(t, err)

	_, err = repo.GetNotificationByID(ctx, "n1", 1)
	assert.Error(t, err)

	_, err = repo.GetUserContact(ctx, 1)
	assert.Error(t, err)
}

func TestCreateNotification_GeneratesUUIDWhenEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewNotificationRepository(db)

	notif := &models.Notification{
		UserID:  123,
		Type:    "system",
		Title:   "T",
		Message: "M",
	}
	mock.ExpectExec(`INSERT INTO notifications`).
		WithArgs(
			sqlmock.AnyArg(),
			"system",
			"App\\User",
			uint64(123),
			sqlmock.AnyArg(),
			(*time.Time)(nil),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	id, err := repo.CreateNotification(context.Background(), notif)
	require.NoError(t, err)
	assert.NotZero(t, id)
	assert.NotEmpty(t, notif.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListNotifications_PaginationAndErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("page less than 1 defaults to 1", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
			WithArgs("App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
			WithArgs("App\\User", uint64(123), 10, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}))

		_, _, err = repo.ListNotifications(ctx, 123, models.NotificationFilter{Page: 0, PerPage: 10})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("perPage less than 1 defaults to 10", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
			WithArgs("App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
			WithArgs("App\\User", uint64(123), 10, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}))

		_, _, err = repo.ListNotifications(ctx, 123, models.NotificationFilter{Page: 1, PerPage: 0})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("perPage greater than 100 is capped", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
			WithArgs("App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))
		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
			WithArgs("App\\User", uint64(123), 100, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}))

		_, _, err = repo.ListNotifications(ctx, 123, models.NotificationFilter{Page: 1, PerPage: 500})
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("count query error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
			WithArgs("App\\User", uint64(123)).
			WillReturnError(errors.New("count failed"))

		_, _, err = repo.ListNotifications(ctx, 123, models.NotificationFilter{Page: 1, PerPage: 10})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("list query error", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
			WithArgs("App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
			WithArgs("App\\User", uint64(123), 10, 0).
			WillReturnError(errors.New("list failed"))

		_, _, err = repo.ListNotifications(ctx, 123, models.NotificationFilter{Page: 1, PerPage: 10})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("invalid json unmarshal", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
			WithArgs("App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
			WithArgs("App\\User", uint64(123), 10, 0).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
				AddRow("n1", `{not-json}`, nil, time.Now(), time.Now()))

		_, _, err = repo.ListNotifications(ctx, 123, models.NotificationFilter{Page: 1, PerPage: 10})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMarkAsRead_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("exec error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE id = \? AND notifiable_type = \? AND notifiable_id = \?`).
			WillReturnError(sql.ErrConnDone)

		err = repo.MarkAsRead(ctx, "n1", 123)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("rows affected error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE id = \? AND notifiable_type = \? AND notifiable_id = \?`).
			WillReturnResult(sqlmock.NewErrorResult(errors.New("rows affected boom")))

		err = repo.MarkAsRead(ctx, "n1", 123)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestMarkAllAsRead_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewNotificationRepository(db)

	mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE notifiable_type = \? AND notifiable_id = \? AND read_at IS NULL`).
		WillReturnError(sql.ErrConnDone)

	err = repo.MarkAllAsRead(context.Background(), 123)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetNotificationByID_JSONAndReadAt(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid json", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
			WithArgs("n1", "App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
				AddRow("n1", `{not-json}`, nil, time.Now(), time.Now()))

		_, err = repo.GetNotificationByID(ctx, "n1", 123)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("valid read_at", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		readAt := time.Date(2024, 3, 10, 14, 30, 0, 0, time.UTC)
		mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
			WithArgs("n1", "App\\User", uint64(123)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
				AddRow("n1", `{"type":"system","title":"T","message":"M","data":{}}`, readAt, time.Now(), time.Now()))

		notif, err := repo.GetNotificationByID(ctx, "n1", 123)
		require.NoError(t, err)
		require.NotNil(t, notif)
		require.NotNil(t, notif.ReadAt)
		assert.True(t, notif.ReadAt.Equal(readAt))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
