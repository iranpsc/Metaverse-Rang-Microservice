package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"metarang/auth-service/internal/models"
)

type CitizenRepository interface {
	GetCitizenByCode(ctx context.Context, code string) (*models.CitizenProfile, error)
	GetCitizenUserInfoByCode(ctx context.Context, code string) (*models.CitizenUserInfo, error)
	GetCitizenReferrals(ctx context.Context, referrerID uint64, search string, page int, pageSize int) ([]*models.CitizenReferral, *models.PaginationMeta, error)
	GetCitizenReferralOrders(ctx context.Context, referralID uint64) ([]*models.ReferrerOrder, error)
	GetCitizenReferralChartData(ctx context.Context, referrerID uint64, rangeType string) (*models.ReferralChartData, error)
	GetCitizenLevels(ctx context.Context, userID uint64) (*models.CitizenLevel, []*models.CitizenLevel, error)
}

type citizenRepository struct {
	db *sql.DB
}

func NewCitizenRepository(db *sql.DB) CitizenRepository {
	return &citizenRepository{db: db}
}

// GetCitizenUserInfoByCode returns user_id and privacy settings for a citizen code.
func (r *citizenRepository) GetCitizenUserInfoByCode(ctx context.Context, code string) (*models.CitizenUserInfo, error) {
	query := `
		SELECT id
		FROM users
		WHERE LOWER(code) = LOWER(?)
		LIMIT 1
	`
	var userID uint64
	err := r.db.QueryRowContext(ctx, query, code).Scan(&userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find citizen by code: %w", err)
	}

	info := &models.CitizenUserInfo{
		UserID:  userID,
		Privacy: map[string]int32{},
	}

	settingsQuery := `
		SELECT privacy
		FROM settings
		WHERE user_id = ?
		LIMIT 1
	`
	var privacyJSON sql.NullString
	err = r.db.QueryRowContext(ctx, settingsQuery, userID).Scan(&privacyJSON)
	if err == nil && privacyJSON.Valid && privacyJSON.String != "" {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(privacyJSON.String), &raw); err == nil {
			for key, value := range raw {
				info.Privacy[key] = privacyValueToInt32(value)
			}
		}
	}

	return info, nil
}

func privacyValueToInt32(value interface{}) int32 {
	switch v := value.(type) {
	case bool:
		if v {
			return 1
		}
		return 0
	case float64:
		return int32(v)
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0
		}
		return int32(i)
	case string:
		if v == "1" || strings.EqualFold(v, "true") {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// GetCitizenByCode retrieves a citizen's profile data by code
func (r *citizenRepository) GetCitizenByCode(ctx context.Context, code string) (*models.CitizenProfile, error) {
	// Get user by code (case-insensitive)
	query := `
		SELECT id, name, email, phone, code, score, email_verified_at
		FROM users
		WHERE LOWER(code) = LOWER(?)
		LIMIT 1
	`

	user := &models.CitizenProfile{}
	var emailVerifiedAt sql.NullTime
	var name, email, phone sql.NullString
	err := r.db.QueryRowContext(ctx, query, code).Scan(
		&user.ID, &name, &email, &phone, &user.Code, &user.Score, &emailVerifiedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find citizen by code: %w", err)
	}
	user.Name = name.String
	user.Email = email.String
	user.Phone = phone.String
	if emailVerifiedAt.Valid {
		user.EmailVerifiedAt = emailVerifiedAt.Time
	}

	// Get KYC data
	kycQuery := `
		SELECT id, user_id, fname, lname, melli_code, status, birthdate
		FROM kycs
		WHERE user_id = ?
		LIMIT 1
	`
	kyc := &models.CitizenKYC{}
	var birthdate sql.NullTime
	var nationalCode string
	err = r.db.QueryRowContext(ctx, kycQuery, user.ID).Scan(
		&kyc.ID, &kyc.UserID, &kyc.Fname, &kyc.Lname, &nationalCode, &kyc.Status, &birthdate,
	)
	kyc.NationalCode = nationalCode
	if err == nil {
		if birthdate.Valid {
			kyc.Birthdate = birthdate.Time
		}
		user.KYC = kyc
	}

	// Get settings with privacy flags
	settingsQuery := `
		SELECT id, user_id, privacy
		FROM settings
		WHERE user_id = ?
		LIMIT 1
	`
	var privacyJSON sql.NullString
	var settingsID, settingsUserID uint64
	err = r.db.QueryRowContext(ctx, settingsQuery, user.ID).Scan(
		&settingsID, &settingsUserID, &privacyJSON,
	)
	if err == nil && privacyJSON.Valid {
		var privacy map[string]bool
		if err := json.Unmarshal([]byte(privacyJSON.String), &privacy); err == nil {
			user.Privacy = privacy
		}
	}

	// Get profile photos
	photosQuery := `
		SELECT id, url
		FROM images
		WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, photosQuery, user.ID)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var photo models.ProfilePhoto
			if err := rows.Scan(&photo.ID, &photo.URL); err == nil {
				user.ProfilePhotos = append(user.ProfilePhotos, &photo)
			}
		}
	}

	// Get personal info
	personalInfoQuery := `
		SELECT id, user_id, occupation, education, memory, loved_city, loved_country,
			loved_language, problem_solving, prediction, about, passions
		FROM personal_infos
		WHERE user_id = ?
		LIMIT 1
	`
	personalInfo := &models.CitizenPersonalInfo{}
	var passionsJSON sql.NullString
	err = r.db.QueryRowContext(ctx, personalInfoQuery, user.ID).Scan(
		&personalInfo.ID, &personalInfo.UserID, &personalInfo.Occupation, &personalInfo.Education,
		&personalInfo.Memory, &personalInfo.LovedCity, &personalInfo.LovedCountry,
		&personalInfo.LovedLanguage, &personalInfo.ProblemSolving, &personalInfo.Prediction,
		&personalInfo.About, &passionsJSON,
	)
	if err == nil {
		if passionsJSON.Valid {
			var passions map[string]bool
			if err := json.Unmarshal([]byte(passionsJSON.String), &passions); err == nil {
				personalInfo.Passions = passions
			}
		}
		user.PersonalInfo = personalInfo
	}
	// If err == sql.ErrNoRows, personal info doesn't exist - that's fine

	// Get avatar URL (static 3D avatar - this would typically come from a config or service)
	// For now, we'll set it to empty and let the service/handler populate it if needed
	// The avatar URL format is typically: /uploads/avatars/{user_id}.svg or similar
	user.Avatar = ""

	return user, nil
}

// GetCitizenReferrals retrieves referrals for a citizen with pagination and search
func (r *citizenRepository) GetCitizenReferrals(ctx context.Context, referrerID uint64, search string, page int, pageSize int) ([]*models.CitizenReferral, *models.PaginationMeta, error) {
	// Build base query - get users referred by this referrer
	baseQuery := `
		SELECT DISTINCT u.id, u.code, u.name, u.created_at
		FROM users u
		WHERE u.referrer_id = ?
	`

	args := []interface{}{referrerID}

	// Add search filter if provided
	if search != "" {
		baseQuery += ` AND (LOWER(u.name) LIKE ? OR LOWER(u.code) LIKE ?)`
		searchPattern := "%" + strings.ToLower(search) + "%"
		args = append(args, searchPattern, searchPattern)
	}

	// Get total count for pagination
	countQuery := `SELECT COUNT(*) FROM (` + baseQuery + `) AS count_query`
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to count referrals: %w", err)
	}

	// Ordering matches Laravel ReferralService::getReferrals (subquery on referrer_id).
	query := `
		SELECT u.id, u.code, u.name, u.created_at
		FROM users u
		WHERE u.referrer_id = ?
	`

	queryArgs := []interface{}{referrerID}

	if search != "" {
		query += ` AND (LOWER(u.name) LIKE ? OR LOWER(u.code) LIKE ?)`
		searchPattern := "%" + strings.ToLower(search) + "%"
		queryArgs = append(queryArgs, searchPattern, searchPattern)
	}

	query += `
		ORDER BY (
			SELECT roh.created_at
			FROM referral_order_histories roh
			WHERE roh.referral_id = u.referrer_id
			ORDER BY roh.created_at DESC
			LIMIT 1
		) DESC, u.id DESC
		LIMIT ? OFFSET ?
	`

	offset := (page - 1) * pageSize
	queryArgs = append(queryArgs, pageSize, offset)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get referrals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var referrals []*models.CitizenReferral
	for rows.Next() {
		ref := &models.CitizenReferral{}
		var createdAt time.Time
		err := rows.Scan(&ref.ID, &ref.Code, &ref.Name, &createdAt)
		if err != nil {
			continue
		}
		ref.CreatedAt = createdAt
		ref.ReferrerOrders = []*models.ReferrerOrder{}

		// Get KYC for name
		kycQuery := `
			SELECT fname, lname
			FROM kycs
			WHERE user_id = ?
			LIMIT 1
		`
		var fname, lname sql.NullString
		if err := r.db.QueryRowContext(ctx, kycQuery, ref.ID).Scan(&fname, &lname); err == nil {
			if fname.Valid && lname.Valid {
				ref.Name = fname.String + " " + lname.String
			}
		}

		// Get latest profile photo
		photoQuery := `
			SELECT url
			FROM images
			WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
			ORDER BY created_at DESC
			LIMIT 1
		`
		var photoURL sql.NullString
		if err := r.db.QueryRowContext(ctx, photoQuery, ref.ID).Scan(&photoURL); err == nil {
			if photoURL.Valid {
				ref.Image = photoURL.String
			}
		}

		referrals = append(referrals, ref)
	}

	// Build pagination meta
	meta := &models.PaginationMeta{
		CurrentPage: int32(page),
	}
	if page*pageSize < totalCount {
		// Next page exists (simplified - in real implementation, you'd construct the full URL)
		meta.NextPageURL = fmt.Sprintf("?page=%d", page+1)
		if search != "" {
			meta.NextPageURL += fmt.Sprintf("&search=%s", search)
		}
	}
	if page > 1 {
		meta.PrevPageURL = fmt.Sprintf("?page=%d", page-1)
		if search != "" {
			meta.PrevPageURL += fmt.Sprintf("&search=%s", search)
		}
	}

	return referrals, meta, nil
}

// GetCitizenReferralOrders retrieves referral order history for a referral
func (r *citizenRepository) GetCitizenReferralOrders(ctx context.Context, referralID uint64) ([]*models.ReferrerOrder, error) {
	query := `
		SELECT id, amount, created_at
		FROM referral_order_histories
		WHERE referral_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, referralID)
	if err != nil {
		return nil, fmt.Errorf("failed to get referral orders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var orders []*models.ReferrerOrder
	for rows.Next() {
		order := &models.ReferrerOrder{}
		var createdAt time.Time
		err := rows.Scan(&order.ID, &order.Amount, &createdAt)
		if err != nil {
			continue
		}
		order.CreatedAt = createdAt
		orders = append(orders, order)
	}

	if orders == nil {
		return []*models.ReferrerOrder{}, nil
	}

	return orders, nil
}

// GetCitizenReferralChartData retrieves aggregated referral chart data
func (r *citizenRepository) GetCitizenReferralChartData(ctx context.Context, referrerID uint64, rangeType string) (*models.ReferralChartData, error) {
	// Get all referrals for this referrer
	referralsQuery := `
		SELECT u.id
		FROM users u
		WHERE u.referrer_id = ?
	`

	referralRows, err := r.db.QueryContext(ctx, referralsQuery, referrerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get referrals: %w", err)
	}
	defer func() { _ = referralRows.Close() }()

	var referralIDs []uint64
	for referralRows.Next() {
		var id uint64
		if err := referralRows.Scan(&id); err == nil {
			referralIDs = append(referralIDs, id)
		}
	}

	if len(referralIDs) == 0 {
		return &models.ReferralChartData{
			TotalReferralsCount:       "0",
			TotalReferralOrdersAmount: "0",
			ChartData:                 []*models.ChartDataPoint{},
		}, nil
	}

	// Build query based on range type
	// Laravel behavior:
	// - yearly: ALL data, no time filter (groups by year from first to current)
	// - monthly: last 12 months
	// - weekly: last 7 days
	// - daily: last 24 hours
	var dateFormat string
	var timeFilter string

	now := time.Now()
	switch rangeType {
	case "yearly":
		timeFilter = "1=1" // No time filter for yearly - show all data
		dateFormat = "%Y/%m"
	case "monthly":
		timeFilter = "DATE(created_at) >= DATE_SUB(?, INTERVAL 12 MONTH)" // Last 12 months, not 1 month
		dateFormat = "%Y/%m/%d"
	case "weekly":
		timeFilter = "DATE(created_at) >= DATE_SUB(?, INTERVAL 7 DAY)" // Last 7 days, not 1 week
		dateFormat = "%Y/%m/%d"
	default: // daily
		timeFilter = "DATE(created_at) >= DATE_SUB(?, INTERVAL 1 DAY)"
		dateFormat = "%Y/%m/%d %H"
	}

	// Get total count and amount
	totalQuery := `
		SELECT COUNT(DISTINCT referral_id) as total_count, COALESCE(SUM(amount), 0) as total_amount
		FROM referral_order_histories
		WHERE referral_id IN (` + buildPlaceholders(len(referralIDs)) + `)
		AND ` + timeFilter
	args := make([]interface{}, len(referralIDs))
	for i, id := range referralIDs {
		args[i] = id
	}
	// Only add time parameter if not yearly (yearly uses "1=1" which needs no parameter)
	if rangeType != "yearly" {
		args = append(args, now)
	}

	var totalCount int
	var totalAmount int64
	err = r.db.QueryRowContext(ctx, totalQuery, args...).Scan(&totalCount, &totalAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to get totals: %w", err)
	}

	// Get chart data points
	// Use DATE_FORMAT in GROUP BY to comply with MySQL's only_full_group_by mode
	chartQuery := `
		SELECT 
			DATE_FORMAT(created_at, ?) as label,
			COUNT(DISTINCT referral_id) as count,
			COALESCE(SUM(amount), 0) as total_amount
		FROM referral_order_histories
		WHERE referral_id IN (` + buildPlaceholders(len(referralIDs)) + `)
		AND ` + timeFilter + `
		GROUP BY DATE_FORMAT(created_at, ?)
		ORDER BY DATE_FORMAT(created_at, ?) ASC
	`

	chartArgs := make([]interface{}, 0)
	chartArgs = append(chartArgs, dateFormat) // For SELECT DATE_FORMAT
	for _, id := range referralIDs {
		chartArgs = append(chartArgs, id)
	}
	// Only add time parameter if not yearly (yearly uses "1=1" which needs no parameter)
	if rangeType != "yearly" {
		chartArgs = append(chartArgs, now)
	}
	chartArgs = append(chartArgs, dateFormat) // For GROUP BY DATE_FORMAT
	chartArgs = append(chartArgs, dateFormat) // For ORDER BY DATE_FORMAT

	chartRows, err := r.db.QueryContext(ctx, chartQuery, chartArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get chart data: %w", err)
	}
	defer func() { _ = chartRows.Close() }()

	var chartData []*models.ChartDataPoint
	for chartRows.Next() {
		point := &models.ChartDataPoint{}
		var label string
		err := chartRows.Scan(&label, &point.Count, &point.TotalAmount)
		if err != nil {
			continue
		}
		// Convert Gregorian date to Jalali format
		point.Label = label // TODO: Convert to Jalali if needed
		chartData = append(chartData, point)
	}

	return &models.ReferralChartData{
		TotalReferralsCount:       fmt.Sprintf("%d", totalCount),
		TotalReferralOrdersAmount: fmt.Sprintf("%d", totalAmount),
		ChartData:                 chartData,
	}, nil
}

// GetCitizenLevels is deprecated; use UserRepository.GetUserLatestLevel and GetLevelsBelowScore.
func (r *citizenRepository) GetCitizenLevels(ctx context.Context, userID uint64) (*models.CitizenLevel, []*models.CitizenLevel, error) {
	return nil, nil, nil
}

func buildPlaceholders(count int) string {
	placeholders := make([]string, count)
	for i := range placeholders {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}
