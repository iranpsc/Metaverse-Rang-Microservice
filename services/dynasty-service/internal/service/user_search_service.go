package service

import (
	"context"
	"database/sql"
	"fmt"
)

// UserSearchService provides user search functionality
type UserSearchService struct {
	db *sql.DB

	// Would call Auth service for complete user details
	// authClient auth.AuthServiceClient
}

func NewUserSearchService(db *sql.DB) *UserSearchService {
	return &UserSearchService{db: db}
}

// UserSearchResult represents a search result
type UserSearchResult struct {
	ID          uint64  `json:"id"`
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	DisplayName string  `json:"-"` // Internal use
	Image       *string `json:"image"`
	Level       string  `json:"level"`
}

// SearchUsers searches for users by code or name
// Implements Laravel UserSearchService::searchUsers
func (s *UserSearchService) SearchUsers(
	ctx context.Context,
	searchTerm string,
	limit int,
) ([]*UserSearchResult, error) {
	query := `
		SELECT u.id, u.code, u.name, 
		       COALESCE(CONCAT(k.fname, ' ', k.lname), u.name) as display_name
		FROM users u
		LEFT JOIN kycs k ON k.user_id = u.id
		WHERE u.code LIKE ? OR u.name LIKE ? OR CONCAT(k.fname, ' ', k.lname) LIKE ?
		LIMIT ?
	`

	term := "%" + searchTerm + "%"
	rows, err := s.db.QueryContext(ctx, query, term, term, term, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*UserSearchResult
	for rows.Next() {
		var r UserSearchResult
		var displayName string

		if err := rows.Scan(&r.ID, &r.Code, &r.Name, &displayName); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		// Use display name (full_name from KYC if available)
		r.Name = displayName

		// Get profile photo
		photo, _ := s.getLatestProfilePhoto(ctx, r.ID)
		r.Image = photo

		// Get level title
		level, _ := s.getLevelTitle(ctx, r.ID)
		r.Level = level

		results = append(results, &r)
	}

	return results, nil
}

// getLatestProfilePhoto retrieves user's latest profile photo
func (s *UserSearchService) getLatestProfilePhoto(ctx context.Context, userID uint64) (*string, error) {
	query := `
		SELECT url FROM images 
		WHERE imageable_type = 'App\\Models\\User' 
		  AND imageable_id = ? 
		ORDER BY id DESC 
		LIMIT 1
	`

	var url string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&url)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &url, nil
}

// getLevelTitle retrieves user's current level title
func (s *UserSearchService) getLevelTitle(ctx context.Context, userID uint64) (string, error) {
	query := `
		SELECT l.title 
		FROM level_user lu
		INNER JOIN levels l ON l.id = lu.level_id
		WHERE lu.user_id = ?
		ORDER BY lu.id DESC
		LIMIT 1
	`

	var title string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&title)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return title, nil
}
