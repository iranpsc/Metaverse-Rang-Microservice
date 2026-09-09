package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/repository"
)

func TestCreateNotification(t *testing.T) {

	tests := []struct {
		name         string
		notification *models.Notification
		setupMock    func(sqlmock.Sqlmock)
		expectError  bool
	}{
		{
			name: "successful creation",
			notification: &models.Notification{
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				UserID:    123,
				Type:      "system",
				Title:     "Test Title",
				Message:   "Test Message",
				Data:      map[string]string{"key": "value"},
				CreatedAt: time.Now(),
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO notifications`).
					WithArgs(
						sqlmock.AnyArg(),  // id
						"system",          // type
						"App\\User",       // notifiable_type
						uint64(123),       // notifiable_id
						sqlmock.AnyArg(),  // data (JSON)
						(*time.Time)(nil), // read_at
						sqlmock.AnyArg(),  // created_at
						sqlmock.AnyArg(),  // updated_at
					).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectError: false,
		},
		{
			name: "database error",
			notification: &models.Notification{
				ID:        "550e8400-e29b-41d4-a716-446655440001",
				UserID:    123,
				Type:      "system",
				Title:     "Test Title",
				Message:   "Test Message",
				CreatedAt: time.Now(),
			},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO notifications`).
					WillReturnError(sql.ErrConnDone)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new mock for each test
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := repository.NewNotificationRepository(db)

			tt.setupMock(mock)

			id, err := repo.CreateNotification(context.Background(), tt.notification)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, uint64(0), id)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uint64(0), id)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListNotifications(t *testing.T) {

	tests := []struct {
		name        string
		userID      uint64
		filter      models.NotificationFilter
		setupMock   func(sqlmock.Sqlmock)
		expectError bool
		expectedLen int
	}{
		{
			name:   "successful list",
			userID: 123,
			filter: models.NotificationFilter{Page: 1, PerPage: 10},
			setupMock: func(mock sqlmock.Sqlmock) {
				// Count query
				mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
					WithArgs("App\\User", uint64(123)).
					WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))

				// List query
				rows := sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
					AddRow(
						"550e8400-e29b-41d4-a716-446655440000",
						`{"type":"system","title":"Test","message":"Message","data":{}}`,
						nil,
						time.Now(),
						time.Now(),
					).
					AddRow(
						"550e8400-e29b-41d4-a716-446655440001",
						`{"type":"alert","title":"Alert","message":"Alert Message","data":{}}`,
						time.Now(),
						time.Now(),
						time.Now(),
					)

				mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
					WithArgs("App\\User", uint64(123), 10, 0).
					WillReturnRows(rows)
			},
			expectError: false,
			expectedLen: 2,
		},
		{
			name:   "empty result",
			userID: 456,
			filter: models.NotificationFilter{Page: 1, PerPage: 10},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ?`).
					WithArgs("App\\User", uint64(456)).
					WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

				mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`).
					WithArgs("App\\User", uint64(456), 10, 0).
					WillReturnRows(sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}))
			},
			expectError: false,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new mock for each test
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()
			repo := repository.NewNotificationRepository(db)

			tt.setupMock(mock)

			notifications, _, err := repo.ListNotifications(context.Background(), tt.userID, tt.filter)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, notifications, tt.expectedLen)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMarkAsRead(t *testing.T) {

	tests := []struct {
		name           string
		notificationID string
		userID         uint64
		setupMock      func(sqlmock.Sqlmock)
		expectError    bool
	}{
		{
			name:           "successful mark as read",
			notificationID: "550e8400-e29b-41d4-a716-446655440000",
			userID:         123,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE id = \? AND notifiable_type = \? AND notifiable_id = \?`).
					WithArgs("550e8400-e29b-41d4-a716-446655440000", "App\\User", uint64(123)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:           "notification not found",
			notificationID: "550e8400-e29b-41d4-a716-446655440001",
			userID:         123,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE id = \? AND notifiable_type = \? AND notifiable_id = \?`).
					WithArgs("550e8400-e29b-41d4-a716-446655440001", "App\\User", uint64(123)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new mock for each test
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := repository.NewNotificationRepository(db)

			tt.setupMock(mock)

			err = repo.MarkAsRead(context.Background(), tt.notificationID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMarkAllAsRead(t *testing.T) {

	tests := []struct {
		name        string
		userID      uint64
		setupMock   func(sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name:   "successful mark all as read",
			userID: 123,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE notifications SET read_at = NOW\(\), updated_at = NOW\(\) WHERE notifiable_type = \? AND notifiable_id = \? AND read_at IS NULL`).
					WithArgs("App\\User", uint64(123)).
					WillReturnResult(sqlmock.NewResult(0, 5))
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new mock for each test
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := repository.NewNotificationRepository(db)

			tt.setupMock(mock)

			err = repo.MarkAllAsRead(context.Background(), tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListNotifications_UnreadOnly(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)
	defer db.Close()
	repo := repository.NewNotificationRepository(db)

	filter := models.NotificationFilter{
		Page:       1,
		PerPage:    10,
		UnreadOnly: true,
	}

	// Count query with unread filter
	mock.ExpectQuery(`SELECT COUNT(*) FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL`).
		WithArgs("App\\User", uint64(123)).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))

	// List query with unread filter
	rows := sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
		AddRow(
			"550e8400-e29b-41d4-a716-446655440000",
			`{"type":"system","title":"Test","message":"Message","data":{}}`,
			nil,
			time.Now(),
			time.Now(),
		)

	mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`).
		WithArgs("App\\User", uint64(123), 10, 0).
		WillReturnRows(rows)

	notifications, total, err := repo.ListNotifications(context.Background(), 123, filter)

	assert.NoError(t, err)
	assert.Len(t, notifications, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetNotificationByID(t *testing.T) {
	tests := []struct {
		name           string
		notificationID string
		userID         uint64
		setupMock      func(sqlmock.Sqlmock)
		expectError    bool
		expectNil      bool
	}{
		{
			name:           "successful get",
			notificationID: "550e8400-e29b-41d4-a716-446655440000",
			userID:         123,
			setupMock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "data", "read_at", "created_at", "updated_at"}).
					AddRow(
						"550e8400-e29b-41d4-a716-446655440000",
						`{"type":"system","title":"Test","message":"Message","data":{"key":"value"}}`,
						nil,
						time.Now(),
						time.Now(),
					)
				mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
					WithArgs("550e8400-e29b-41d4-a716-446655440000", "App\\User", uint64(123)).
					WillReturnRows(rows)
			},
			expectError: false,
			expectNil:   false,
		},
		{
			name:           "not found",
			notificationID: "550e8400-e29b-41d4-a716-446655440001",
			userID:         123,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
					WithArgs("550e8400-e29b-41d4-a716-446655440001", "App\\User", uint64(123)).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name:           "database error",
			notificationID: "550e8400-e29b-41d4-a716-446655440002",
			userID:         123,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT id, data, read_at, created_at, updated_at FROM notifications WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? LIMIT 1`).
					WithArgs("550e8400-e29b-41d4-a716-446655440002", "App\\User", uint64(123)).
					WillReturnError(sql.ErrConnDone)
			},
			expectError: true,
			expectNil:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()
			repo := repository.NewNotificationRepository(db)

			tt.setupMock(mock)

			notification, err := repo.GetNotificationByID(context.Background(), tt.notificationID, tt.userID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectNil {
				assert.Nil(t, notification)
			} else if !tt.expectError {
				assert.NotNil(t, notification)
				if notification != nil {
					assert.Equal(t, tt.notificationID, notification.ID)
					assert.Equal(t, tt.userID, notification.UserID)
				}
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestGetUserContact(t *testing.T) {
	ctx := context.Background()

	t.Run("returns phone and email", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email"}).AddRow("09120000000", "user@example.com"))

		contact, err := repo.GetUserContact(ctx, 42)
		require.NoError(t, err)
		require.NotNil(t, contact)
		assert.Equal(t, "09120000000", contact.Phone)
		assert.Equal(t, "user@example.com", contact.Email)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("null phone is empty string", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"phone", "email"}).AddRow(nil, "a@b.com"))

		contact, err := repo.GetUserContact(ctx, 7)
		require.NoError(t, err)
		require.NotNil(t, contact)
		assert.Equal(t, "", contact.Phone)
		assert.Equal(t, "a@b.com", contact.Email)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("missing user returns nil", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(9)).
			WillReturnError(sql.ErrNoRows)

		contact, err := repo.GetUserContact(ctx, 9)
		require.NoError(t, err)
		assert.Nil(t, contact)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := repository.NewNotificationRepository(db)

		mock.ExpectQuery(`SELECT phone, email FROM users WHERE id = \? LIMIT 1`).
			WithArgs(uint64(1)).
			WillReturnError(sql.ErrConnDone)

		_, err = repo.GetUserContact(ctx, 1)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
