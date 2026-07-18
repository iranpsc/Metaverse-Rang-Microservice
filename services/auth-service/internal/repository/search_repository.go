package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"metarang/auth-service/internal/models"
)

type SearchRepository interface {
	// SearchUsers searches users by name, code, and KYC fname/lname
	// Splits searchTerm on spaces and creates OR conditions
	// Returns up to 5 results with profile photos and limited KYC columns
	SearchUsers(ctx context.Context, searchTerm string) ([]*SearchUserResult, error)

	// SearchFeatures searches feature_properties by id and address
	// Returns up to 5 results with feature, owner, and geometry coordinates
	SearchFeatures(ctx context.Context, searchTerm string) ([]*SearchFeatureResult, error)

	// SearchIsicCodes searches isic_codes table by name
	// Returns all matches (no limit)
	SearchIsicCodes(ctx context.Context, searchTerm string) ([]*IsicCodeResult, error)
}

type searchRepository struct {
	db *sql.DB
}

func NewSearchRepository(db *sql.DB) SearchRepository {
	return &searchRepository{db: db}
}

// SearchUserResult represents a user search result with related data
type SearchUserResult struct {
	User          *models.User
	KYC           *models.KYC
	ProfilePhotos []*models.Image
	Followers     int32
	LatestLevel   *UserLevel
}

// SearchFeatureResult represents a feature search result with related data
type SearchFeatureResult struct {
	FeatureID           uint64
	FeaturePropertiesID string
	Address             string
	Karbari             string
	PricePsc            string
	PriceIrr            string
	OwnerCode           string
	Coordinates         []*Coordinate
}

// Coordinate represents a geometry coordinate
type Coordinate struct {
	ID uint64
	X  float64
	Y  float64
}

// IsicCodeResult represents an ISIC code search result
type IsicCodeResult struct {
	ID   uint64
	Name string
	Code uint64
}

// SearchUsers searches users by splitting searchTerm and matching across multiple fields
func (r *searchRepository) SearchUsers(ctx context.Context, searchTerm string) ([]*SearchUserResult, error) {
	// Split search term on spaces
	searchTerms := strings.Fields(searchTerm)
	if len(searchTerms) == 0 {
		return []*SearchUserResult{}, nil
	}

	// Build the query
	// Laravel logic: (ANY term matches name OR code) OR (has KYC where ANY term matches fname OR lname)
	// This translates to: (term1 matches name OR code OR term2 matches name OR code OR ...)
	//                      OR EXISTS (KYC where (term1 matches fname OR lname OR term2 matches fname OR lname OR ...))

	var userConditions []string
	var userArgs []interface{}
	for _, term := range searchTerms {
		userConditions = append(userConditions, "(u.name LIKE ? OR u.code LIKE ?)")
		userArgs = append(userArgs, "%"+term+"%", "%"+term+"%")
	}

	var kycConditions []string
	var kycArgs []interface{}
	for _, term := range searchTerms {
		kycConditions = append(kycConditions, "(k.fname LIKE ? OR k.lname LIKE ?)")
		kycArgs = append(kycArgs, "%"+term+"%", "%"+term+"%")
	}

	// Combine args for the query
	allArgs := append(userArgs, kycArgs...)

	// Build the query with proper grouping
	query := `
		SELECT DISTINCT
			u.id, u.name, u.email, u.phone, u.code, u.referrer_id, u.score, 
			u.last_seen, u.created_at, u.updated_at, u.email_verified_at, u.phone_verified_at,
			k.id as kyc_id, k.user_id, k.fname, k.lname, k.melli_code, k.melli_card, 
			k.video, k.verify_text_id, k.province, k.gender, k.status, k.birthdate, 
			k.errors, k.created_at as kyc_created_at, k.updated_at as kyc_updated_at
		FROM users u
		LEFT JOIN kycs k ON u.id = k.user_id
		WHERE (
			(` + strings.Join(userConditions, " OR ") + `)
			OR EXISTS (
				SELECT 1 FROM kycs k2
				WHERE k2.user_id = u.id
				AND (` + strings.Join(kycConditions, " OR ") + `)
			)
		)
		LIMIT 5
	`

	args := allArgs

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*SearchUserResult
	userMap := make(map[uint64]*SearchUserResult)

	for rows.Next() {
		var user models.User
		var kyc models.KYC
		var kycID sql.NullInt64
		var kycUserID sql.NullInt64
		var kycFname sql.NullString
		var kycLname sql.NullString
		var kycMelliCode sql.NullString
		var kycMelliCard sql.NullString
		var kycVideo sql.NullString
		var kycVerifyTextID sql.NullInt64
		var kycProvince sql.NullString
		var kycGender sql.NullString
		var kycStatus sql.NullInt64
		var kycBirthdate sql.NullTime
		var kycErrors sql.NullString
		var kycCreatedAt sql.NullTime
		var kycUpdatedAt sql.NullTime
		var emailVerifiedAt sql.NullTime
		var phoneVerifiedAt sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Name, &user.Email, &user.Phone, &user.Code, &user.ReferrerID,
			&user.Score, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
			&emailVerifiedAt, &phoneVerifiedAt,
			&kycID, &kycUserID, &kycFname, &kycLname, &kycMelliCode, &kycMelliCard,
			&kycVideo, &kycVerifyTextID, &kycProvince, &kycGender, &kycStatus,
			&kycBirthdate, &kycErrors, &kycCreatedAt, &kycUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		if emailVerifiedAt.Valid {
			user.EmailVerifiedAt = emailVerifiedAt
		}
		if phoneVerifiedAt.Valid {
			user.PhoneVerifiedAt = phoneVerifiedAt
		}

		// Build KYC if exists
		if kycID.Valid {
			kyc.ID = uint64(kycID.Int64)
			if kycUserID.Valid {
				kyc.UserID = uint64(kycUserID.Int64)
			}
			if kycFname.Valid {
				kyc.Fname = kycFname.String
			}
			if kycLname.Valid {
				kyc.Lname = kycLname.String
			}
			if kycMelliCode.Valid {
				kyc.MelliCode = kycMelliCode.String
			}
			if kycMelliCard.Valid {
				kyc.MelliCard = kycMelliCard.String
			}
			kyc.Video = kycVideo
			if kycVerifyTextID.Valid {
				kyc.VerifyTextID = sql.NullInt64{Int64: kycVerifyTextID.Int64, Valid: true}
			}
			if kycProvince.Valid {
				kyc.Province = kycProvince.String
			}
			kyc.Gender = kycGender
			if kycStatus.Valid {
				kyc.Status = int32(kycStatus.Int64)
			}
			kyc.Birthdate = kycBirthdate
			kyc.Errors = kycErrors
			if kycCreatedAt.Valid {
				kyc.CreatedAt = kycCreatedAt.Time
			}
			if kycUpdatedAt.Valid {
				kyc.UpdatedAt = kycUpdatedAt.Time
			}

			kycPtr := kyc
			userMap[user.ID] = &SearchUserResult{
				User: &user,
				KYC:  &kycPtr,
			}
		} else {
			userMap[user.ID] = &SearchUserResult{
				User: &user,
				KYC:  nil,
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	// Convert map to slice
	results = make([]*SearchUserResult, 0, len(userMap))
	for _, result := range userMap {
		results = append(results, result)
	}

	// Load profile photos and followers for each user
	for _, result := range results {
		// Get profile photos
		photos, err := r.getProfilePhotos(ctx, result.User.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get profile photos: %w", err)
		}
		result.ProfilePhotos = photos

		// Get followers count
		count, err := r.getFollowersCount(ctx, result.User.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get followers count: %w", err)
		}
		result.Followers = count

		// Get latest level
		level, err := r.getLatestLevel(ctx, result.User.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest level: %w", err)
		}
		result.LatestLevel = level
	}

	return results, nil
}

// getProfilePhotos retrieves profile photos for a user
func (r *searchRepository) getProfilePhotos(ctx context.Context, userID uint64) ([]*models.Image, error) {
	query := `
		SELECT id, imageable_type, imageable_id, url, created_at, updated_at
		FROM images
		WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var photos []*models.Image
	for rows.Next() {
		var img models.Image
		if err := rows.Scan(&img.ID, &img.ImageableType, &img.ImageableID, &img.URL, &img.CreatedAt, &img.UpdatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, &img)
	}

	return photos, rows.Err()
}

// getFollowersCount returns the number of followers for a user
func (r *searchRepository) getFollowersCount(ctx context.Context, userID uint64) (int32, error) {
	query := `SELECT COUNT(*) FROM follows WHERE following_id = ?`
	var count int32
	if err := r.db.QueryRowContext(ctx, query, userID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// getLatestLevel returns the user's latest level
func (r *searchRepository) getLatestLevel(ctx context.Context, userID uint64) (*UserLevel, error) {
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
		return nil, err
	}
	return &level, nil
}

// SearchFeatures searches feature_properties by id and address
func (r *searchRepository) SearchFeatures(ctx context.Context, searchTerm string) ([]*SearchFeatureResult, error) {
	query := `
		SELECT DISTINCT
			fp.id as feature_properties_id,
			fp.address,
			fp.price_psc,
			fp.price_irr,
			fp.karbari,
			f.id as feature_id,
			u.code as owner_code
		FROM feature_properties fp
		INNER JOIN features f ON fp.feature_id = f.id
		INNER JOIN users u ON f.owner_id = u.id
		WHERE fp.id LIKE ? OR fp.address LIKE ?
		LIMIT 5
	`

	searchPattern := "%" + searchTerm + "%"
	rows, err := r.db.QueryContext(ctx, query, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search features: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*SearchFeatureResult
	for rows.Next() {
		var result SearchFeatureResult
		err := rows.Scan(
			&result.FeaturePropertiesID,
			&result.Address,
			&result.PricePsc,
			&result.PriceIrr,
			&result.Karbari,
			&result.FeatureID,
			&result.OwnerCode,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan feature: %w", err)
		}

		// Get coordinates for this feature's geometry
		coordinates, err := r.getFeatureCoordinates(ctx, result.FeatureID)
		if err != nil {
			return nil, fmt.Errorf("failed to get coordinates: %w", err)
		}
		result.Coordinates = coordinates

		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating features: %w", err)
	}

	return results, nil
}

// getFeatureCoordinates retrieves coordinates for a feature's geometry
func (r *searchRepository) getFeatureCoordinates(ctx context.Context, featureID uint64) ([]*Coordinate, error) {
	query := `
		SELECT c.id, CAST(c.x AS DECIMAL(10,6)) as x, CAST(c.y AS DECIMAL(10,6)) as y
		FROM coordinates c
		INNER JOIN geometries g ON c.geometry_id = g.id
		INNER JOIN features f ON g.feature_id = f.id
		WHERE f.id = ?
		ORDER BY c.id
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var coordinates []*Coordinate
	for rows.Next() {
		var coord Coordinate
		if err := rows.Scan(&coord.ID, &coord.X, &coord.Y); err != nil {
			return nil, err
		}
		coordinates = append(coordinates, &coord)
	}

	return coordinates, rows.Err()
}

// SearchIsicCodes searches isic_codes table by name
func (r *searchRepository) SearchIsicCodes(ctx context.Context, searchTerm string) ([]*IsicCodeResult, error) {
	query := `
		SELECT id, name, code
		FROM isic_codes
		WHERE name LIKE ?
		ORDER BY id
	`

	searchPattern := "%" + searchTerm + "%"
	rows, err := r.db.QueryContext(ctx, query, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search isic codes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*IsicCodeResult
	for rows.Next() {
		var result IsicCodeResult
		var code sql.NullInt64
		err := rows.Scan(&result.ID, &result.Name, &code)
		if err != nil {
			return nil, fmt.Errorf("failed to scan isic code: %w", err)
		}
		if code.Valid {
			result.Code = uint64(code.Int64)
		}
		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating isic codes: %w", err)
	}

	return results, nil
}
