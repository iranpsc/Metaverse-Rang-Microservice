// Package repository provides data access for notifications.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"metarang/notifications-service/internal/models"

	"github.com/google/uuid"
)

// NotificationRepository handles database interactions for notifications.
type NotificationRepository struct {
	db *sql.DB
}

// notificationData represents the JSON structure stored in the database's data column
type notificationData struct {
	Type    string            `json:"type"`
	Title   string            `json:"title"`
	Message string            `json:"message"`
	Data    map[string]string `json:"data,omitempty"`
}

// NewNotificationRepository creates a new repository instance.
func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

// CreateNotification persists a notification record.
// Returns the notification ID as a numeric value (for compatibility with NotificationResult.ID uint64)
func (r *NotificationRepository) CreateNotification(ctx context.Context, notification *models.Notification) (uint64, error) {
	if r.db == nil {
		return 0, fmt.Errorf("database connection is nil")
	}

	// Generate UUID if not provided
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}

	// Prepare data for JSON storage
	dataJSON := notificationData{
		Type:    notification.Type,
		Title:   notification.Title,
		Message: notification.Message,
		Data:    notification.Data,
	}

	dataBytes, err := json.Marshal(dataJSON)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal notification data: %w", err)
	}

	now := time.Now()
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = now
	}

	// Insert notification using Laravel's polymorphic structure
	query := `
		INSERT INTO notifications (id, type, notifiable_type, notifiable_id, data, read_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		notification.ID,
		notification.Type,
		"App\\User", // Laravel's default user model type
		notification.UserID,
		string(dataBytes),
		notification.ReadAt,
		notification.CreatedAt,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create notification: %w", err)
	}

	// Return a numeric ID (hash of UUID for compatibility)
	// In a real scenario, you might want to change NotificationResult.ID to string
	// For now, we'll return a simple numeric hash
	return hashStringToUint64(notification.ID), nil
}

// ListNotifications retrieves notifications for a user along with the total count.
func (r *NotificationRepository) ListNotifications(ctx context.Context, userID uint64, filter models.NotificationFilter) ([]models.Notification, int64, error) {
	if r.db == nil {
		return nil, 0, fmt.Errorf("database connection is nil")
	}

	// Set defaults for pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100 // limit max page size
	}
	offset := (page - 1) * perPage

	// Build WHERE clause with optional unread filter
	whereClause := "notifiable_type = ? AND notifiable_id = ?"
	if filter.UnreadOnly {
		whereClause += " AND read_at IS NULL"
	}

	// Get total count
	var total int64
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) FROM notifications 
		WHERE %s
	`, whereClause)
	err := r.db.QueryRowContext(ctx, countQuery, "App\\User", userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count notifications: %w", err)
	}

	// Fetch notifications
	query := fmt.Sprintf(`
		SELECT id, data, read_at, created_at, updated_at
		FROM notifications
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	rows, err := r.db.QueryContext(ctx, query, "App\\User", userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	notifications := make([]models.Notification, 0)
	for rows.Next() {
		var notif models.Notification
		var dataJSON string
		var readAt sql.NullTime

		err := rows.Scan(
			&notif.ID,
			&dataJSON,
			&readAt,
			&notif.CreatedAt,
			&notif.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan notification: %w", err)
		}

		// Parse JSON data
		var data notificationData
		if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal notification data: %w", err)
		}

		notif.UserID = userID
		notif.Type = data.Type
		notif.Title = data.Title
		notif.Message = data.Message
		notif.Data = data.Data
		if readAt.Valid {
			notif.ReadAt = &readAt.Time
		}

		notifications = append(notifications, notif)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, total, nil
}

// MarkAsRead marks a single notification as read.
func (r *NotificationRepository) MarkAsRead(ctx context.Context, notificationID string, userID uint64) error {
	if r.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `
		UPDATE notifications 
		SET read_at = NOW(), updated_at = NOW()
		WHERE id = ? AND notifiable_type = ? AND notifiable_id = ?
	`

	result, err := r.db.ExecContext(ctx, query, notificationID, "App\\User", userID)
	if err != nil {
		return fmt.Errorf("failed to mark notification as read: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("notification not found or already marked as read")
	}

	return nil
}

// MarkAllAsRead marks all notifications as read for a user.
func (r *NotificationRepository) MarkAllAsRead(ctx context.Context, userID uint64) error {
	if r.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `
		UPDATE notifications 
		SET read_at = NOW(), updated_at = NOW()
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, "App\\User", userID)
	if err != nil {
		return fmt.Errorf("failed to mark all notifications as read: %w", err)
	}

	return nil
}

// GetNotificationByID retrieves a single notification by ID for a specific user.
func (r *NotificationRepository) GetNotificationByID(ctx context.Context, notificationID string, userID uint64) (*models.Notification, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT id, data, read_at, created_at, updated_at
		FROM notifications
		WHERE id = ? AND notifiable_type = ? AND notifiable_id = ?
		LIMIT 1
	`

	var notif models.Notification
	var dataJSON string
	var readAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, notificationID, "App\\User", userID).Scan(
		&notif.ID,
		&dataJSON,
		&readAt,
		&notif.CreatedAt,
		&notif.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get notification: %w", err)
	}

	// Parse JSON data
	var data notificationData
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal notification data: %w", err)
	}

	notif.UserID = userID
	notif.Type = data.Type
	notif.Title = data.Title
	notif.Message = data.Message
	notif.Data = data.Data
	if readAt.Valid {
		notif.ReadAt = &readAt.Time
	}

	return &notif, nil
}

// hashStringToUint64 converts a string to a uint64 hash
// This is a simple hash function for compatibility with NotificationResult.ID
func hashStringToUint64(s string) uint64 {
	var hash uint64
	for _, c := range s {
		hash = hash*31 + uint64(c)
	}
	return hash
}
