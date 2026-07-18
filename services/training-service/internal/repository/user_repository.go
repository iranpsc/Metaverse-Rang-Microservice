package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/training-service/internal/client"
)

// UserRepositoryInterface defines the interface for user repository operations
type UserRepositoryInterface interface {
	GetUserBasicByCode(ctx context.Context, code string) (*UserBasic, error)
	GetUserByID(ctx context.Context, userID uint64) (*UserBasic, error)
}

type UserRepository struct {
	db         *sql.DB
	authClient *client.AuthClient
}

// NewUserRepository creates a new user repository with optional auth client
// If authClient is nil, falls back to direct database queries
func NewUserRepository(db *sql.DB, authClient *client.AuthClient) *UserRepository {
	return &UserRepository{
		db:         db,
		authClient: authClient,
	}
}

// GetUserBasicByCode retrieves basic user information by code
// Uses auth-service gRPC client if available, otherwise falls back to direct DB query
func (r *UserRepository) GetUserBasicByCode(ctx context.Context, code string) (*UserBasic, error) {
	// Use auth-service client if available
	if r.authClient != nil {
		user, err := r.authClient.GetUserByCode(ctx, code)
		if err != nil {
			// If auth service fails, fall back to DB query
			return r.getUserByCodeFromDB(ctx, code)
		}

		// Convert proto User to UserBasic
		userBasic := &UserBasic{
			ID:    user.Id,
			Name:  user.Name,
			Code:  user.Code,
			Email: user.Email,
		}

		// Get profile photo from database (auth-service User doesn't include profile photo)
		// TODO: Consider adding profile photo to auth-service GetUser response
		photoURL := r.getProfilePhotoFromDB(ctx, user.Id)
		if photoURL != "" {
			userBasic.ProfilePhoto = photoURL
		}

		return userBasic, nil
	}

	// Fall back to direct DB query
	return r.getUserByCodeFromDB(ctx, code)
}

// getUserByCodeFromDB retrieves user from database directly
func (r *UserRepository) getUserByCodeFromDB(ctx context.Context, code string) (*UserBasic, error) {
	query := `
		SELECT id, name, code, email
		FROM users
		WHERE code = ?
		LIMIT 1
	`

	var user UserBasic
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&user.ID,
		&user.Name,
		&user.Code,
		&user.Email,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by code: %w", err)
	}

	// Get latest profile photo
	user.ProfilePhoto = r.getProfilePhotoFromDB(ctx, user.ID)

	return &user, nil
}

// getProfilePhotoFromDB retrieves the latest profile photo URL for a user
func (r *UserRepository) getProfilePhotoFromDB(ctx context.Context, userID uint64) string {
	photoQuery := `
		SELECT url
		FROM images
		WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	var photoURL sql.NullString
	_ = r.db.QueryRowContext(ctx, photoQuery, userID).Scan(&photoURL)
	if photoURL.Valid {
		return photoURL.String
	}
	return ""
}

// GetUserByID retrieves basic user information by ID
// Uses auth-service gRPC client if available, otherwise falls back to direct DB query
func (r *UserRepository) GetUserByID(ctx context.Context, userID uint64) (*UserBasic, error) {
	// Use auth-service client if available
	if r.authClient != nil {
		user, err := r.authClient.GetUser(ctx, userID)
		if err != nil {
			// If auth service fails, fall back to DB query
			return r.getUserByIDFromDB(ctx, userID)
		}

		// Convert proto User to UserBasic
		userBasic := &UserBasic{
			ID:    user.Id,
			Name:  user.Name,
			Code:  user.Code,
			Email: user.Email,
		}

		// Get profile photo from database (auth-service User doesn't include profile photo)
		// TODO: Consider adding profile photo to auth-service GetUser response
		photoURL := r.getProfilePhotoFromDB(ctx, userID)
		if photoURL != "" {
			userBasic.ProfilePhoto = photoURL
		}

		return userBasic, nil
	}

	// Fall back to direct DB query
	return r.getUserByIDFromDB(ctx, userID)
}

// getUserByIDFromDB retrieves user from database directly
func (r *UserRepository) getUserByIDFromDB(ctx context.Context, userID uint64) (*UserBasic, error) {
	query := `
		SELECT id, name, code, email
		FROM users
		WHERE id = ?
		LIMIT 1
	`

	var user UserBasic
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID,
		&user.Name,
		&user.Code,
		&user.Email,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	// Get latest profile photo
	user.ProfilePhoto = r.getProfilePhotoFromDB(ctx, userID)

	return &user, nil
}

// UserBasic represents basic user information
type UserBasic struct {
	ID           uint64
	Name         string
	Code         string
	Email        string
	ProfilePhoto string
}
