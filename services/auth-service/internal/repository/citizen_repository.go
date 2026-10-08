package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"metarang/auth-service/internal/models"
	"metarang/shared/pkg/helpers"
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
		UserID: userID,
	}

	settingsQuery := `
		SELECT privacy
		FROM settings
		WHERE user_id = ?
		LIMIT 1
	`
	var privacyJSON sql.NullString
	err = r.db.QueryRowContext(ctx, settingsQuery, userID).Scan(&privacyJSON)
	raw := ""
	if err == nil && privacyJSON.Valid {
		raw = privacyJSON.String
	}
	info.Privacy = models.PrivacyIntToInt32Map(models.ParsePrivacyJSON(raw))

	return info, nil
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
	rawPrivacy := ""
	if err == nil && privacyJSON.Valid {
		rawPrivacy = privacyJSON.String
	}
	user.Privacy = models.PrivacyIntToBoolMap(models.ParsePrivacyJSON(rawPrivacy))

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
	baseQuery, args = appendReferralNameSearch(baseQuery, args, search)

	// Get total count for pagination
	countQuery := `SELECT COUNT(*) FROM (` + baseQuery + `) AS count_query`
	var totalCount int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to count referrals: %w", err)
	}

	query := `
		SELECT u.id, u.code, u.name, u.created_at
		FROM users u
		WHERE u.referrer_id = ?
	`

	queryArgs := []interface{}{referrerID}
	query, queryArgs = appendReferralNameSearch(query, queryArgs, search)

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

// appendReferralNameSearch filters referrals by account name, citizen code, and
// KYC name. The list returns "fname lname" once KYC exists, and those values
// are the Persian names users search for. LOWER() is omitted on purpose:
// utf8mb4_unicode_ci is already case-insensitive for ASCII, and LOWER() makes
// LIKE miss multibyte Persian text.
func appendReferralNameSearch(query string, args []interface{}, search string) (string, []interface{}) {
	search = strings.TrimSpace(search)
	if search == "" {
		return query, args
	}

	query += ` AND (
		u.name LIKE ? OR u.code LIKE ?
		OR EXISTS (
			SELECT 1 FROM kycs k
			WHERE k.user_id = u.id
			AND (
				k.fname LIKE ?
				OR k.lname LIKE ?
				OR CONCAT(k.fname, ' ', k.lname) LIKE ?
			)
		)
	)`
	pattern := "%" + search + "%"
	args = append(args, pattern, pattern, pattern, pattern, pattern)
	return query, args
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

// GetCitizenReferralChartData retrieves aggregated referral chart data.
// total_referrals_count is the number of referred users (users.referrer_id),
// matching GET /referrals. Order amounts come from referral_order_histories
// and are independent — referrals without orders still count.
func (r *citizenRepository) GetCitizenReferralChartData(ctx context.Context, referrerID uint64, rangeType string) (*models.ReferralChartData, error) {
	// yearly: all data, grouped by Gregorian year/month (converted to Jalali Y/m)
	// monthly: last 12 months, grouped by month (converted to Jalali Y/m)
	// weekly: last 7 days, grouped by day (converted to Jalali Y/m/d)
	// daily: last 24 hours, grouped by day (converted to Jalali Y/m/d)
	dateFormat, userTimeFilter, orderTimeFilter, needsTimeArg := referralChartRangeSQL(rangeType)
	now := time.Now()

	countArgs := []interface{}{referrerID}
	if needsTimeArg {
		countArgs = append(countArgs, now)
	}
	countQuery := `
		SELECT COUNT(*)
		FROM users u
		WHERE u.referrer_id = ?
		AND ` + userTimeFilter

	var totalCount int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("failed to count referrals: %w", err)
	}

	amountArgs := []interface{}{referrerID}
	if needsTimeArg {
		amountArgs = append(amountArgs, now)
	}
	amountQuery := `
		SELECT COALESCE(SUM(roh.amount), 0)
		FROM referral_order_histories roh
		INNER JOIN users u ON u.id = roh.referral_id
		WHERE u.referrer_id = ?
		AND ` + orderTimeFilter

	var totalAmount int64
	if err := r.db.QueryRowContext(ctx, amountQuery, amountArgs...).Scan(&totalAmount); err != nil {
		return nil, fmt.Errorf("failed to get referral order totals: %w", err)
	}

	buckets := make(map[string]*models.ChartDataPoint)

	referralChartArgs := []interface{}{dateFormat, referrerID}
	if needsTimeArg {
		referralChartArgs = append(referralChartArgs, now)
	}
	referralChartArgs = append(referralChartArgs, dateFormat, dateFormat)
	referralChartQuery := `
		SELECT
			DATE_FORMAT(u.created_at, ?) as label,
			COUNT(*) as count
		FROM users u
		WHERE u.referrer_id = ?
		AND ` + userTimeFilter + `
		GROUP BY DATE_FORMAT(u.created_at, ?)
		ORDER BY DATE_FORMAT(u.created_at, ?) ASC
	`

	referralRows, err := r.db.QueryContext(ctx, referralChartQuery, referralChartArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get referral chart data: %w", err)
	}

	for referralRows.Next() {
		var label string
		var count int64
		if err := referralRows.Scan(&label, &count); err != nil {
			continue
		}
		point := chartBucket(buckets, label)
		point.Count = int32(count)
	}
	_ = referralRows.Close()

	orderChartArgs := []interface{}{dateFormat, referrerID}
	if needsTimeArg {
		orderChartArgs = append(orderChartArgs, now)
	}
	orderChartArgs = append(orderChartArgs, dateFormat, dateFormat)
	orderChartQuery := `
		SELECT
			DATE_FORMAT(roh.created_at, ?) as label,
			COALESCE(SUM(roh.amount), 0) as total_amount
		FROM referral_order_histories roh
		INNER JOIN users u ON u.id = roh.referral_id
		WHERE u.referrer_id = ?
		AND ` + orderTimeFilter + `
		GROUP BY DATE_FORMAT(roh.created_at, ?)
		ORDER BY DATE_FORMAT(roh.created_at, ?) ASC
	`

	orderRows, err := r.db.QueryContext(ctx, orderChartQuery, orderChartArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get referral order chart data: %w", err)
	}
	defer func() { _ = orderRows.Close() }()

	for orderRows.Next() {
		var label string
		var amount int64
		if err := orderRows.Scan(&label, &amount); err != nil {
			continue
		}
		point := chartBucket(buckets, label)
		point.TotalAmount = amount
	}

	labels := make([]string, 0, len(buckets))
	for label := range buckets {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	chartData := make([]*models.ChartDataPoint, 0, len(labels))
	for _, label := range labels {
		point := buckets[label]
		point.Label = gregorianChartLabelToJalali(label, rangeType)
		chartData = append(chartData, point)
	}

	return &models.ReferralChartData{
		TotalReferralsCount:       fmt.Sprintf("%d", totalCount),
		TotalReferralOrdersAmount: fmt.Sprintf("%d", totalAmount),
		ChartData:                 chartData,
	}, nil
}

func referralChartRangeSQL(rangeType string) (dateFormat, userTimeFilter, orderTimeFilter string, needsTimeArg bool) {
	switch rangeType {
	case "yearly":
		return "%Y/%m", "1=1", "1=1", false
	case "monthly":
		return "%Y/%m",
			"DATE(u.created_at) >= DATE_SUB(?, INTERVAL 12 MONTH)",
			"DATE(roh.created_at) >= DATE_SUB(?, INTERVAL 12 MONTH)",
			true
	case "weekly":
		return "%Y/%m/%d",
			"DATE(u.created_at) >= DATE_SUB(?, INTERVAL 7 DAY)",
			"DATE(roh.created_at) >= DATE_SUB(?, INTERVAL 7 DAY)",
			true
	default:
		return "%Y/%m/%d",
			"DATE(u.created_at) >= DATE_SUB(?, INTERVAL 1 DAY)",
			"DATE(roh.created_at) >= DATE_SUB(?, INTERVAL 1 DAY)",
			true
	}
}

func chartBucket(buckets map[string]*models.ChartDataPoint, label string) *models.ChartDataPoint {
	if point, ok := buckets[label]; ok {
		return point
	}
	point := &models.ChartDataPoint{}
	buckets[label] = point
	return point
}

// GetCitizenLevels is deprecated; use UserRepository.GetUserLatestLevel and GetLevelsBelowScore.
func (r *citizenRepository) GetCitizenLevels(ctx context.Context, userID uint64) (*models.CitizenLevel, []*models.CitizenLevel, error) {
	return nil, nil, nil
}

func gregorianChartLabelToJalali(label, rangeType string) string {
	parsed, ok := parseGregorianChartLabel(label)
	if !ok {
		return label
	}

	jalali := helpers.FormatJalaliDate(parsed)
	switch rangeType {
	case "yearly", "monthly":
		if len(jalali) >= 7 {
			return jalali[:7]
		}
		return jalali
	default:
		return jalali
	}
}

func parseGregorianChartLabel(label string) (time.Time, bool) {
	for _, layout := range []string{"2006/01/02 15", "2006/01/02", "2006/01"} {
		if t, err := time.Parse(layout, label); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
