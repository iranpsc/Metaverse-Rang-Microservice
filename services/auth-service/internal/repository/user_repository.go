package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"metarang/auth-service/internal/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByID(ctx context.Context, id uint64) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateLastSeen(ctx context.Context, userID uint64) error
	FindByCode(ctx context.Context, code string) (*models.User, error)
	GetSettings(ctx context.Context, userID uint64) (*models.Settings, error)
	CreateSettings(ctx context.Context, settings *models.Settings) error
	GetKYC(ctx context.Context, userID uint64) (*models.KYC, error)
	GetUnreadNotificationsCount(ctx context.Context, userID uint64) (int32, error)
	MarkEmailAsVerified(ctx context.Context, userID uint64) error
	UpdatePhone(ctx context.Context, userID uint64, phone string) error
	MarkPhoneAsVerified(ctx context.Context, userID uint64) error
	IsPhoneTaken(ctx context.Context, phone string, excludeUserID uint64) (bool, error)
	ExistsByWalletAddress(ctx context.Context, address string, excludeUserID uint64) (bool, error)
	LinkWalletAddress(ctx context.Context, userID uint64, address string) (LinkWalletResult, error)
	// Users API methods
	ListUsers(ctx context.Context, search string, orderBy string, page int32, limit int32) ([]*UserWithRelations, int32, error)
	GetUsersLevelsForList(ctx context.Context, userIDs []uint64) (map[uint64]*UserListLevels, error)
	GetFollowersCount(ctx context.Context, userID uint64) (int32, error)
	GetFollowingCount(ctx context.Context, userID uint64) (int32, error)
	GetLatestProfilePhotoURL(ctx context.Context, userID uint64) (string, error)
	GetAllProfilePhotoURLs(ctx context.Context, userID uint64) ([]string, error)
	GetUserLatestLevel(ctx context.Context, userID uint64) (*UserLevel, error)
	GetLevelsBelowScore(ctx context.Context, score int32) ([]*UserLevel, error)
	GetNextLevelScore(ctx context.Context, currentScore int32) (int32, error)
	GetFeatureCounts(ctx context.Context, userID uint64) (maskoni int32, tejari int32, amoozeshi int32, err error)
}

// UserLevel represents level information from database
type UserLevel struct {
	ID    uint64
	Name  string
	Score int32
	Slug  string
	Image string
}

// UserWithRelations represents a user with related data for listing
type UserWithRelations struct {
	User            *models.User
	KYCName         *string // Full name from KYC if available
	ProfilePhotoURL *string
}

// UserListLevel is a level row for GET /api/users (Laravel UserResource levels)
type UserListLevel struct {
	ID    uint64
	Name  string
	Slug  string
	Score int32
	Image string
}

// UserListLevels holds current + all achieved levels for one user in the list endpoint
type UserListLevels struct {
	Current  *UserListLevel
	Previous []*UserListLevel
}

type LinkWalletResult string

const (
	LinkWalletSuccess          LinkWalletResult = "success"
	LinkWalletAlreadyConnected LinkWalletResult = "already_connected"
	LinkWalletAlreadyLinked    LinkWalletResult = "already_linked"
)

type userRepository struct {
	db            *sql.DB
	adminPanelURL string
}

func NewUserRepository(db *sql.DB, adminPanelURL string) UserRepository {
	return &userRepository{
		db:            db,
		adminPanelURL: strings.TrimSuffix(adminPanelURL, "/"),
	}
}

// formatImageURL formats image URL with admin_panel_url + /uploads/ prefix.
// Implements Laravel: config('app.admin_panel_url') . '/uploads/' . $this->image->url
func (r *userRepository) formatImageURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		return imageURL
	}
	if r.adminPanelURL == "" {
		path := strings.TrimPrefix(imageURL, "/")
		if !strings.HasPrefix(path, "uploads/") {
			return "/uploads/" + path
		}
		return "/" + path
	}
	path := strings.TrimPrefix(imageURL, "/")
	if !strings.HasPrefix(path, "uploads/") {
		path = "uploads/" + path
	}
	return r.adminPanelURL + "/" + path
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (name, email, phone, password, code, ip, referrer_id, 
			access_token, refresh_token, token_type, expires_in, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	var phoneValue interface{}
	if user.Phone.Valid {
		phoneValue = user.Phone.String
	} else {
		phoneValue = nil
	}
	result, err := r.db.ExecContext(ctx, query,
		user.Name, user.Email, phoneValue, user.Password, user.Code, user.IP,
		user.ReferrerID, user.AccessToken, user.RefreshToken, user.TokenType,
		user.ExpiresIn, time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	user.ID = uint64(id)

	return nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, name, email, phone, password, code, referrer_id, score, ip, 
			last_seen, email_verified_at, phone_verified_at, access_token, 
			refresh_token, token_type, expires_in, wallet_address, created_at, updated_at
		FROM users
		WHERE email = ?
	`
	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.Password,
		&user.Code, &user.ReferrerID, &user.Score, &user.IP, &user.LastSeen,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.AccessToken,
		&user.RefreshToken, &user.TokenType, &user.ExpiresIn, &user.WalletAddress,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}
	return user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uint64) (*models.User, error) {
	query := `
		SELECT id, name, email, phone, password, code, referrer_id, score, ip, 
			last_seen, email_verified_at, phone_verified_at, access_token, 
			refresh_token, token_type, expires_in, wallet_address, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.Password,
		&user.Code, &user.ReferrerID, &user.Score, &user.IP, &user.LastSeen,
		&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.AccessToken,
		&user.RefreshToken, &user.TokenType, &user.ExpiresIn, &user.WalletAddress,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}
	return user, nil
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users 
		SET name = ?, email = ?, phone = ?, access_token = ?, refresh_token = ?,
			token_type = ?, expires_in = ?, updated_at = ?
		WHERE id = ?
	`
	var phoneValue interface{}
	if user.Phone.Valid {
		phoneValue = user.Phone.String
	} else {
		phoneValue = nil
	}
	_, err := r.db.ExecContext(ctx, query,
		user.Name, user.Email, phoneValue, user.AccessToken, user.RefreshToken,
		user.TokenType, user.ExpiresIn, time.Now(), user.ID)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

func (r *userRepository) UpdateLastSeen(ctx context.Context, userID uint64) error {
	query := `UPDATE users SET last_seen = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}
	return nil
}

func (r *userRepository) FindByCode(ctx context.Context, code string) (*models.User, error) {
	query := `SELECT id FROM users WHERE code = ?`
	var id uint64
	err := r.db.QueryRowContext(ctx, query, code).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by code: %w", err)
	}
	return r.FindByID(ctx, id)
}

func (r *userRepository) GetSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	// Use the SettingsRepository implementation for consistency
	settingsRepo := NewSettingsRepository(r.db)
	return settingsRepo.FindByUserID(ctx, userID)
}

func (r *userRepository) GetKYC(ctx context.Context, userID uint64) (*models.KYC, error) {
	query := `
		SELECT id, user_id, fname, lname, melli_code, status, birthdate, created_at, updated_at
		FROM kycs WHERE user_id = ?
	`
	kyc := &models.KYC{}
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&kyc.ID, &kyc.UserID, &kyc.Fname, &kyc.Lname, &kyc.MelliCode,
		&kyc.Status, &kyc.Birthdate, &kyc.CreatedAt, &kyc.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get kyc: %w", err)
	}
	return kyc, nil
}

func (r *userRepository) GetUnreadNotificationsCount(ctx context.Context, userID uint64) (int32, error) {
	query := `
		SELECT COUNT(*) FROM notifications 
		WHERE notifiable_type = 'App\\Models\\User' 
		AND notifiable_id = ? 
		AND read_at IS NULL
	`
	var count int32
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread notifications count: %w", err)
	}
	return count, nil
}

func (r *userRepository) CreateSettings(ctx context.Context, settings *models.Settings) error {
	// Use the SettingsRepository implementation for consistency
	settingsRepo := NewSettingsRepository(r.db)
	return settingsRepo.Create(ctx, settings)
}

func (r *userRepository) MarkEmailAsVerified(ctx context.Context, userID uint64) error {
	query := `UPDATE users SET email_verified_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to mark email as verified: %w", err)
	}
	return nil
}

func (r *userRepository) UpdatePhone(ctx context.Context, userID uint64, phone string) error {
	query := `UPDATE users SET phone = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, phone, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update user phone: %w", err)
	}
	return nil
}

func (r *userRepository) MarkPhoneAsVerified(ctx context.Context, userID uint64) error {
	query := `UPDATE users SET phone_verified_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, time.Now(), time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to mark phone as verified: %w", err)
	}
	return nil
}

func (r *userRepository) IsPhoneTaken(ctx context.Context, phone string, excludeUserID uint64) (bool, error) {
	query := `SELECT COUNT(*) FROM users WHERE phone = ? AND id != ?`
	var count int
	if err := r.db.QueryRowContext(ctx, query, phone, excludeUserID).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to check phone uniqueness: %w", err)
	}
	return count > 0, nil
}

func (r *userRepository) ExistsByWalletAddress(ctx context.Context, address string, excludeUserID uint64) (bool, error) {
	query := `SELECT COUNT(*) FROM users WHERE wallet_address = ?`
	args := []interface{}{address}
	if excludeUserID > 0 {
		query += ` AND id != ?`
		args = append(args, excludeUserID)
	}

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, fmt.Errorf("failed to check wallet address uniqueness: %w", err)
	}
	return count > 0, nil
}

func (r *userRepository) LinkWalletAddress(ctx context.Context, userID uint64, address string) (LinkWalletResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existingWallet sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT wallet_address FROM users WHERE id = ? FOR UPDATE
	`, userID).Scan(&existingWallet)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found")
	}
	if err != nil {
		return "", fmt.Errorf("failed to lock user row: %w", err)
	}
	if existingWallet.Valid && existingWallet.String != "" {
		return LinkWalletAlreadyConnected, nil
	}

	var linkedCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE wallet_address = ? FOR UPDATE
	`, address).Scan(&linkedCount); err != nil {
		return "", fmt.Errorf("failed to lock wallet address row: %w", err)
	}
	if linkedCount > 0 {
		return LinkWalletAlreadyLinked, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET wallet_address = ?, updated_at = ? WHERE id = ?
	`, address, time.Now(), userID); err != nil {
		return "", fmt.Errorf("failed to update wallet address: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("failed to commit wallet link transaction: %w", err)
	}

	return LinkWalletSuccess, nil
}

// ListUsers returns paginated list of users with search and ordering
// Filters out admin user (code = 'hm-2000000')
func (r *userRepository) ListUsers(ctx context.Context, search string, orderBy string, page int32, limit int32) ([]*UserWithRelations, int32, error) {
	offset := (page - 1) * limit

	// Build WHERE clause
	whereClause := "WHERE u.code != 'hm-2000000'"
	args := []interface{}{}

	if search != "" {
		whereClause += " AND u.name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	// Build ORDER BY clause
	orderClause := "ORDER BY u.score DESC" // Default: descending score
	switch orderBy {
	case "score":
		orderClause = "ORDER BY u.score DESC"
	case "registered_at_asc":
		orderClause = "ORDER BY u.email_verified_at ASC"
	case "registered_at_desc":
		orderClause = "ORDER BY u.email_verified_at DESC"
	}

	// Query to get users with relations (levels loaded in batch via GetUsersLevelsForList)
	query := fmt.Sprintf(`
		SELECT 
			u.id, u.name, u.email, u.phone, u.password, u.code, u.referrer_id, u.score, u.ip,
			u.last_seen, u.email_verified_at, u.phone_verified_at, u.access_token,
			u.refresh_token, u.token_type, u.expires_in, u.created_at, u.updated_at,
			k.fname, k.lname,
			(SELECT url FROM images WHERE imageable_type = 'App\\Models\\User' AND imageable_id = u.id ORDER BY created_at DESC LIMIT 1) as profile_photo_url
		FROM users u
		LEFT JOIN kycs k ON k.user_id = u.id AND k.status = 1
		%s
		%s
		LIMIT ? OFFSET ?
	`, whereClause, orderClause)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*UserWithRelations
	for rows.Next() {
		user := &models.User{}
		ur := &UserWithRelations{User: user}

		var email sql.NullString
		var kycFname, kycLname sql.NullString
		var profilePhotoURL sql.NullString

		err := rows.Scan(
			&user.ID, &user.Name, &email, &user.Phone, &user.Password, &user.Code,
			&user.ReferrerID, &user.Score, &user.IP, &user.LastSeen,
			&user.EmailVerifiedAt, &user.PhoneVerifiedAt, &user.AccessToken,
			&user.RefreshToken, &user.TokenType, &user.ExpiresIn,
			&user.CreatedAt, &user.UpdatedAt,
			&kycFname, &kycLname,
			&profilePhotoURL,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan user: %w", err)
		}

		if email.Valid {
			user.Email = email.String
		}

		// Set KYC name if available
		if kycFname.Valid && kycLname.Valid {
			fullName := kycFname.String + " " + kycLname.String
			ur.KYCName = &fullName
		}

		if profilePhotoURL.Valid {
			url := profilePhotoURL.String
			ur.ProfilePhotoURL = &url
		}

		users = append(users, ur)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating users: %w", err)
	}

	// Get total count for pagination
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users u %s", whereClause)
	var totalCount int32
	countArgs := args[:len(args)-2] // Remove LIMIT and OFFSET
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	return users, totalCount, nil
}

// GetUsersLevelsForList loads all achieved levels per user for GET /api/users (Laravel UserResource).
func (r *userRepository) GetUsersLevelsForList(ctx context.Context, userIDs []uint64) (map[uint64]*UserListLevels, error) {
	result := make(map[uint64]*UserListLevels)
	if len(userIDs) == 0 {
		return result, nil
	}

	placeholders := buildUserIDPlaceholders(len(userIDs))
	query := fmt.Sprintf(`
		SELECT lu.user_id, l.id, l.name, l.slug, CAST(l.score AS SIGNED) as score,
		       COALESCE(i.url, '') as image_url
		FROM level_user lu
		INNER JOIN levels l ON l.id = lu.level_id
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE lu.user_id IN (%s)
		ORDER BY lu.user_id ASC, l.id ASC
	`, placeholders)

	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get users levels for list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID uint64
		var level UserListLevel
		var name, slug, imageURL sql.NullString
		var score sql.NullInt32

		if err := rows.Scan(&userID, &level.ID, &name, &slug, &score, &imageURL); err != nil {
			return nil, fmt.Errorf("failed to scan user level: %w", err)
		}
		if name.Valid {
			level.Name = name.String
		}
		if slug.Valid {
			level.Slug = slug.String
		}
		if score.Valid {
			level.Score = score.Int32
		}
		if imageURL.Valid {
			level.Image = r.formatImageURL(imageURL.String)
		}

		bundle, ok := result[userID]
		if !ok {
			bundle = &UserListLevels{Previous: []*UserListLevel{}}
			result[userID] = bundle
		}
		lvl := level
		bundle.Previous = append(bundle.Previous, &lvl)

		// Laravel latest_level: levels()->orderByDesc('id')->first()
		if bundle.Current == nil || level.ID > bundle.Current.ID {
			currentCopy := lvl
			bundle.Current = &currentCopy
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user levels: %w", err)
	}

	return result, nil
}

func buildUserIDPlaceholders(count int) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}

// GetFollowersCount returns the number of followers for a user
func (r *userRepository) GetFollowersCount(ctx context.Context, userID uint64) (int32, error) {
	query := `SELECT COUNT(*) FROM follows WHERE following_id = ?`
	var count int32
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get followers count: %w", err)
	}
	return count, nil
}

// GetFollowingCount returns the number of users being followed
func (r *userRepository) GetFollowingCount(ctx context.Context, userID uint64) (int32, error) {
	query := `SELECT COUNT(*) FROM follows WHERE follower_id = ?`
	var count int32
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get following count: %w", err)
	}
	return count, nil
}

// GetLatestProfilePhotoURL returns the URL of the latest profile photo for a user
func (r *userRepository) GetLatestProfilePhotoURL(ctx context.Context, userID uint64) (string, error) {
	query := `
		SELECT url FROM images
		WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	var url sql.NullString
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&url)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get latest profile photo: %w", err)
	}
	if url.Valid {
		return url.String, nil
	}
	return "", nil
}

// GetAllProfilePhotoURLs returns all profile photo URLs for a user
func (r *userRepository) GetAllProfilePhotoURLs(ctx context.Context, userID uint64) ([]string, error) {
	query := `
		SELECT url FROM images
		WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile photos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var urls []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return nil, fmt.Errorf("failed to scan profile photo URL: %w", err)
		}
		urls = append(urls, url)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating profile photos: %w", err)
	}

	return urls, nil
}

// GetUserLatestLevel returns the user's latest level
func (r *userRepository) GetUserLatestLevel(ctx context.Context, userID uint64) (*UserLevel, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS SIGNED) as score,
		       COALESCE(i.url, '') as image_url
		FROM level_user lu
		INNER JOIN levels l ON l.id = lu.level_id
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE lu.user_id = ?
		ORDER BY lu.id DESC
		LIMIT 1
	`

	var level UserLevel
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&level.ID, &level.Name, &level.Slug, &level.Score, &level.Image)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest level: %w", err)
	}
	level.Image = r.formatImageURL(level.Image)
	return &level, nil
}

// GetLevelsBelowScore returns all levels below the given score
func (r *userRepository) GetLevelsBelowScore(ctx context.Context, score int32) ([]*UserLevel, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS SIGNED) as score,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE CAST(l.score AS SIGNED) < ?
		ORDER BY CAST(l.score AS SIGNED) ASC
	`

	rows, err := r.db.QueryContext(ctx, query, score)
	if err != nil {
		return nil, fmt.Errorf("failed to get levels below score: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var levels []*UserLevel
	for rows.Next() {
		var level UserLevel
		if err := rows.Scan(&level.ID, &level.Name, &level.Slug, &level.Score, &level.Image); err != nil {
			return nil, fmt.Errorf("failed to scan level: %w", err)
		}
		level.Image = r.formatImageURL(level.Image)
		levels = append(levels, &level)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating levels: %w", err)
	}

	return levels, nil
}

// GetNextLevelScore returns the score of the next level above current score
func (r *userRepository) GetNextLevelScore(ctx context.Context, currentScore int32) (int32, error) {
	query := `
		SELECT CAST(l.score AS SIGNED) as score
		FROM levels l
		WHERE CAST(l.score AS SIGNED) > ?
		ORDER BY CAST(l.score AS SIGNED) ASC
		LIMIT 1
	`

	var nextScore sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, currentScore).Scan(&nextScore)
	if err == sql.ErrNoRows {
		return 0, nil // No next level
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get next level score: %w", err)
	}
	if nextScore.Valid {
		return int32(nextScore.Int64), nil
	}
	return 0, nil
}

// GetFeatureCounts returns feature counts by category for a user
func (r *userRepository) GetFeatureCounts(ctx context.Context, userID uint64) (maskoni int32, tejari int32, amoozeshi int32, err error) {
	query := `
		SELECT 
			SUM(CASE WHEN fp.karbari = 'm' THEN 1 ELSE 0 END) as maskoni_count,
			SUM(CASE WHEN fp.karbari = 't' THEN 1 ELSE 0 END) as tejari_count,
			SUM(CASE WHEN fp.karbari = 'a' THEN 1 ELSE 0 END) as amoozeshi_count
		FROM features f
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.owner_id = ?
	`

	var maskoniCount, tejariCount, amoozeshiCount sql.NullInt64
	err = r.db.QueryRowContext(ctx, query, userID).Scan(&maskoniCount, &tejariCount, &amoozeshiCount)
	if err == sql.ErrNoRows {
		return 0, 0, 0, nil
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get feature counts: %w", err)
	}

	if maskoniCount.Valid {
		maskoni = int32(maskoniCount.Int64)
	}
	if tejariCount.Valid {
		tejari = int32(tejariCount.Int64)
	}
	if amoozeshiCount.Valid {
		amoozeshi = int32(amoozeshiCount.Int64)
	}

	return maskoni, tejari, amoozeshi, nil
}
