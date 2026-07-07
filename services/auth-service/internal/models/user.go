package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID              uint64         `db:"id"`
	Name            string         `db:"name"`
	Email           string         `db:"email"`
	Phone           sql.NullString `db:"phone"`
	Password        string         `db:"password"`
	Code            string         `db:"code"`
	ReferrerID      sql.NullInt64  `db:"referrer_id"`
	Score           int32          `db:"score"`
	IP              string         `db:"ip"`
	LastSeen        sql.NullTime   `db:"last_seen"`
	EmailVerifiedAt sql.NullTime   `db:"email_verified_at"`
	PhoneVerifiedAt sql.NullTime   `db:"phone_verified_at"`
	AccessToken     sql.NullString `db:"access_token"`
	RefreshToken    sql.NullString `db:"refresh_token"`
	TokenType       sql.NullString `db:"token_type"`
	ExpiresIn       sql.NullInt64  `db:"expires_in"`
	WalletAddress   sql.NullString `db:"wallet_address"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

type PersonalAccessToken struct {
	ID            uint64       `db:"id"`
	TokenableType string       `db:"tokenable_type"`
	TokenableID   uint64       `db:"tokenable_id"`
	Name          string       `db:"name"`
	Token         string       `db:"token"`
	Abilities     string       `db:"abilities"`
	LastUsedAt    sql.NullTime `db:"last_used_at"`
	ExpiresAt     sql.NullTime `db:"expires_at"`
	CreatedAt     time.Time    `db:"created_at"`
	UpdatedAt     time.Time    `db:"updated_at"`
}

type KYC struct {
	ID           uint64         `db:"id"`
	UserID       uint64         `db:"user_id"`
	Fname        string         `db:"fname"`
	Lname        string         `db:"lname"`
	MelliCode    string         `db:"melli_code"`
	MelliCard    string         `db:"melli_card"`
	Video        sql.NullString `db:"video"`
	VerifyTextID sql.NullInt64  `db:"verify_text_id"`
	Province     string         `db:"province"`
	Gender       sql.NullString `db:"gender"`
	Status       int32          `db:"status"`
	Birthdate    sql.NullTime   `db:"birthdate"`
	Errors       sql.NullString `db:"errors"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

func (k *KYC) FullName() string {
	return k.Fname + " " + k.Lname
}

// Rejected returns true if KYC status is rejected (-1)
func (k *KYC) Rejected() bool {
	return k.Status == -1
}

// Pending returns true if KYC status is pending (0)
func (k *KYC) Pending() bool {
	return k.Status == 0
}

// Approved returns true if KYC status is approved (1)
func (k *KYC) Approved() bool {
	return k.Status == 1
}

type Settings struct {
	ID                uint64          `db:"id"`
	UserID            uint64          `db:"user_id"`
	Status            bool            `db:"status"`
	Level             bool            `db:"level"`
	Details           bool            `db:"details"`
	CheckoutDaysCount uint32          `db:"checkout_days_count"`
	AutomaticLogout   int32           `db:"automatic_logout"`
	Privacy           map[string]int  `db:"privacy"`       // JSON: key -> 0|1 (0=private, 1=public)
	Notifications     map[string]bool `db:"notifications"` // JSON: channel -> bool
	CreatedAt         time.Time       `db:"created_at"`
	UpdatedAt         time.Time       `db:"updated_at"`
}

// DefaultPrivacySettings returns default privacy settings with all fields set to 1 (public) except contact fields
func DefaultPrivacySettings() map[string]int {
	return map[string]int{
		"nationality":                          1,
		"fname":                                1,
		"birthdate":                            1,
		"phone":                                0, // private by default
		"email":                                0, // private by default
		"address":                              0, // private by default
		"about":                                1,
		"name":                                 1,
		"registered_at":                        1,
		"position":                             1,
		"level":                                1,
		"score":                                1,
		"licenses":                             1,
		"license_score":                        1,
		"avatar":                               1,
		"occupation":                           1,
		"education":                            1,
		"loved_city":                           1,
		"loved_country":                        1,
		"loved_language":                       1,
		"prediction":                           1,
		"memory":                               1,
		"passions":                             1,
		"amoozeshi_features":                   1,
		"maskoni_features":                     1,
		"tejari_features":                      1,
		"gardeshgari_features":                 1,
		"fazasabz_features":                    1,
		"behdashti_features":                   1,
		"edari_features":                       1,
		"nemayeshgah_features":                 1,
		"bought_golden_keys":                   1,
		"used_golden_keys":                     1,
		"recieved_golden_keys":                 1,
		"bought_bronze_keys":                   1,
		"used_bronze_keys":                     1,
		"recieved_bronze_keys":                 1,
		"establish_store_license":              1,
		"establish_union_license":              1,
		"establish_taxi_license":               1,
		"establish_amoozeshgah_license":        1,
		"reporter_license":                     1,
		"cooporation_license":                  1,
		"developer_license":                    1,
		"inspection_license":                   1,
		"trading_license":                      1,
		"lawyer_license":                       1,
		"city_council_license":                 1,
		"governer_license":                     1,
		"ostandar_license":                     1,
		"level_one_judge_license":              1,
		"level_two_judge_license":              1,
		"level_three_judge_license":            1,
		"gate_license":                         1,
		"all_licenses":                         1,
		"referrals":                            1,
		"irr_income":                           1,
		"psc_income":                           1,
		"complaint":                            1,
		"warnings":                             1,
		"commited_crimes":                      1,
		"satisfaction":                         1,
		"referral_profit":                      1,
		"irr_transactions":                     1,
		"psc_transactions":                     1,
		"blue_transactions":                    1,
		"yellow_transactions":                  1,
		"red_transactions":                     1,
		"sold_features":                        1,
		"bought_features":                      1,
		"sold_products":                        1,
		"bought_products":                      1,
		"recieved_irr_prizes":                  1,
		"recieved_psc_prizes":                  1,
		"recieved_yellow_prizes":               1,
		"recieved_blue_prizes":                 1,
		"recieved_red_prizes":                  1,
		"recieved_satisfaction_prizes":         1,
		"dynasty_members_photo":                1,
		"dynasty_members_info":                 1,
		"recieved_dynasty_satisfaction_prizes": 1,
		"recieved_dynasty_referral_profit_prizes":             1,
		"recieved_dynasty_accumulated_capital_reserve_prizes": 1,
		"recieved_dynasty_data_storage_prizes":                1,
		"followers":                                           1,
		"followers_count":                                     1,
		"following":                                           1,
		"following_count":                                     1,
		"violations":                                          1,
		"breaking_laws":                                       1,
		"paid_psc_fine":                                       1,
		"paid_irr_fine":                                       1,
		"life_style":                                          1,
		"negative_score":                                      1,
		"code":                                                1,
	}
}

// DefaultNotificationSettings returns default notification settings with all channels enabled
func DefaultNotificationSettings() map[string]bool {
	return map[string]bool{
		"announcements_sms":        true,
		"announcements_email":      true,
		"reports_sms":              true,
		"reports_email":            true,
		"login_verification_sms":   true,
		"login_verification_email": true,
		"transactions_sms":         true,
		"transactions_email":       true,
		"trades_sms":               true,
		"trades_email":             true,
	}
}

type AccountSecurity struct {
	ID           uint64        `db:"id"`
	UserID       uint64        `db:"user_id"`
	Unlocked     bool          `db:"unlocked"`
	Until        sql.NullInt64 `db:"until"`
	Length       int64         `db:"length"`
	LastActivity sql.NullInt64 `db:"last_activity"`
	CreatedAt    time.Time     `db:"created_at"`
	UpdatedAt    time.Time     `db:"updated_at"`
}

type Otp struct {
	ID             uint64    `db:"id"`
	UserID         uint64    `db:"user_id"`
	VerifiableType string    `db:"verifiable_type"`
	VerifiableID   uint64    `db:"verifiable_id"`
	Code           string    `db:"code"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type BankAccount struct {
	ID           uint64         `db:"id"`
	BankableType string         `db:"bankable_type"`
	BankableID   uint64         `db:"bankable_id"`
	BankName     string         `db:"bank_name"`
	ShabaNum     string         `db:"shaba_num"`
	CardNum      string         `db:"card_num"`
	Status       int32          `db:"status"`
	Errors       sql.NullString `db:"errors"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
}

// Pending returns true if bank account status is pending (0)
func (b *BankAccount) Pending() bool {
	return b.Status == 0
}

// Verified returns true if bank account status is verified/approved (1)
func (b *BankAccount) Verified() bool {
	return b.Status == 1
}

// Rejected returns true if bank account status is rejected (-1)
func (b *BankAccount) Rejected() bool {
	return b.Status == -1
}

// UserEvent represents login/logout and other user events
type UserEvent struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	Event     string    `db:"event"`
	IP        string    `db:"ip"`
	Device    string    `db:"device"`
	Status    int32     `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// UserEventReport represents a report for a user event
type UserEventReport struct {
	ID                uint64         `db:"id"`
	UserEventID       uint64         `db:"user_event_id"`
	SuspeciousCitizen sql.NullString `db:"suspecious_citizen"` // Note: Laravel uses 'suspecious' (typo)
	EventDescription  string         `db:"event_description"`
	Status            int32          `db:"status"`
	Closed            bool           `db:"closed"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
}

// UserEventReportResponse represents a response to a user event report
type UserEventReportResponse struct {
	ID                uint64    `db:"id"`
	UserEventReportID uint64    `db:"user_event_report_id"`
	Response          string    `db:"response"`
	ResponserName     string    `db:"responser_name"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

// UserActivity represents user session activity tracking
type UserActivity struct {
	ID        uint64       `db:"id"`
	UserID    uint64       `db:"user_id"`
	Start     time.Time    `db:"start"`
	End       sql.NullTime `db:"end"`
	Total     int32        `db:"total"` // Total minutes
	IP        string       `db:"ip"`
	CreatedAt time.Time    `db:"created_at"`
	UpdatedAt time.Time    `db:"updated_at"`
}

// UserLog represents user scoring and activity statistics
type UserLog struct {
	ID                uint64    `db:"id"`
	UserID            uint64    `db:"user_id"`
	TransactionsCount float64   `db:"transactions_count"`
	FollowersCount    float64   `db:"followers_count"`
	DepositAmount     float64   `db:"deposit_amount"`
	ActivityHours     float64   `db:"activity_hours"`
	Score             float64   `db:"score"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

// UserVariables represents per-user settings and limits
type UserVariables struct {
	ID             uint64    `db:"id"`
	UserID         uint64    `db:"user_id"`
	WithdrawProfit int32     `db:"withdraw_profit"` // Days
	ReferralProfit float64   `db:"referral_profit"` // Limit amount
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}
