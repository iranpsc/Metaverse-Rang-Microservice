// Package formatter shapes API responses for the dynasty service.
package formatter

import (
	"fmt"

	"metarang/dynasty-service/internal/models"
	"metarang/shared/pkg/helpers"
)

// DynastyResponse matches Laravel DynastyResource exactly
type DynastyResponse struct {
	UserHasDynasty bool               `json:"user-has-dynasty"` // Note: kebab-case!
	ID             uint64             `json:"id,omitempty"`
	FamilyID       uint64             `json:"family_id,omitempty"`
	CreatedAt      string             `json:"created_at,omitempty"`
	ProfileImage   string             `json:"profile-image,omitempty"`   // Note: kebab-case!
	DynastyFeature *DynastyFeature    `json:"dynasty-feature,omitempty"` // Note: kebab-case!
	Features       []AvailableFeature `json:"features,omitempty"`
}

// DynastyFeature represents the dynasty's selected feature
type DynastyFeature struct {
	ID                    uint64 `json:"id"`
	PropertiesID          string `json:"properties_id"`
	Area                  string `json:"area"`
	Density               string `json:"density"`
	FeatureProfitIncrease string `json:"feature-profit-increase"` // Note: kebab-case!
	FamilyMembersCount    int    `json:"family-members-count"`    // Note: kebab-case!
	LastUpdated           string `json:"last-updated"`            // Note: kebab-case!
}

// AvailableFeature represents a feature available for dynasty
type AvailableFeature struct {
	ID           uint64 `json:"id"`
	PropertiesID string `json:"properties_id"`
	Density      string `json:"density"`
	Stability    string `json:"stability"`
	Area         string `json:"area"`
}

// FormatDynastyResponse formats dynasty to match Laravel DynastyResource exactly
func FormatDynastyResponse(
	dynasty *models.Dynasty,
	familyID uint64,
	familyMembersCount int,
	featureID uint64,
	featurePropsID string,
	area, density string,
	stability float64,
	dynastyUpdatedAt string,
	profilePhoto *string,
	userFeatures []AvailableFeature,
) *DynastyResponse {
	// Calculate feature profit increase
	// Implements Laravel: (stability / 10000 - 1) if > 10000, else 0
	var profitIncrease string
	if stability > 10000 {
		increase := (stability / 10000) - 1
		profitIncrease = fmt.Sprintf("%.3f", increase)
	} else {
		profitIncrease = "0"
	}

	profileImageStr := ""
	if profilePhoto != nil {
		profileImageStr = *profilePhoto
	}

	return &DynastyResponse{
		UserHasDynasty: true,
		ID:             dynasty.ID,
		FamilyID:       familyID,
		CreatedAt:      helpers.FormatJalaliDate(dynasty.CreatedAt),
		ProfileImage:   profileImageStr,
		DynastyFeature: &DynastyFeature{
			ID:                    featureID,
			PropertiesID:          featurePropsID,
			Area:                  area,
			Density:               density,
			FeatureProfitIncrease: profitIncrease,
			FamilyMembersCount:    familyMembersCount,
			LastUpdated:           dynastyUpdatedAt, // Already in Jalali format
		},
		Features: userFeatures,
	}
}

// SentRequestResource matches Laravel SentRequestsResource
type SentRequestResource struct {
	ID           uint64     `json:"id"`
	ToUser       UserBasic  `json:"to_user"`
	Relationship string     `json:"relationship"`
	Status       int16      `json:"status"`
	Prize        *PrizeInfo `json:"prize"`
	CreatedAt    string     `json:"created_at"`
}

// ReceivedRequestResource matches Laravel RecievedJoinRequest
type ReceivedRequestResource struct {
	ID           uint64    `json:"id"`
	FromUser     UserBasic `json:"from_user"`
	Relationship string    `json:"relationship"`
	Message      string    `json:"message"`
	Status       int16     `json:"status"`
	CreatedAt    string    `json:"created_at"`
}

// FamilyMemberResource matches Laravel FamilyMemberResource
type FamilyMemberResource struct {
	ID           uint64        `json:"id"`
	User         UserWithLevel `json:"user"`
	Relationship string        `json:"relationship"`
}

// UserBasic represents basic user info
type UserBasic struct {
	ID    uint64  `json:"id"`
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Image *string `json:"image"`
}

// UserWithLevel represents user with level
type UserWithLevel struct {
	ID    uint64  `json:"id"`
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Image *string `json:"image"`
	Level string  `json:"level"`
}

// PrizeInfo represents prize details
type PrizeInfo struct {
	Satisfaction               float64 `json:"satisfaction"`
	PSC                        int     `json:"psc"`
	IntroductionProfitIncrease float64 `json:"introduction_profit_increase"`
	AccumulatedCapitalReserve  float64 `json:"accumulated_capital_reserve"`
	DataStorage                float64 `json:"data_storage"`
}

// FormatSentRequest formats sent join request
func FormatSentRequest(
	req *models.JoinRequest,
	toUser UserBasic,
	prize *models.DynastyPrize,
) *SentRequestResource {
	var prizeInfo *PrizeInfo
	if prize != nil {
		prizeInfo = &PrizeInfo{
			Satisfaction:               prize.Satisfaction,
			PSC:                        prize.PSC,
			IntroductionProfitIncrease: prize.IntroductionProfitIncrease,
			AccumulatedCapitalReserve:  prize.AccumulatedCapitalReserve,
			DataStorage:                prize.DataStorage,
		}
	}

	return &SentRequestResource{
		ID:           req.ID,
		ToUser:       toUser,
		Relationship: req.Relationship,
		Status:       req.Status,
		Prize:        prizeInfo,
		CreatedAt:    helpers.FormatJalaliDateTime(req.CreatedAt),
	}
}

// FormatReceivedRequest formats received join request
func FormatReceivedRequest(
	req *models.JoinRequest,
	fromUser UserBasic,
) *ReceivedRequestResource {
	message := ""
	if req.Message != nil {
		message = *req.Message
	}

	return &ReceivedRequestResource{
		ID:           req.ID,
		FromUser:     fromUser,
		Relationship: req.Relationship,
		Message:      message,
		Status:       req.Status,
		CreatedAt:    helpers.FormatJalaliDateTime(req.CreatedAt),
	}
}

// FormatFamilyMember formats family member
func FormatFamilyMember(
	member *models.FamilyMember,
	user UserWithLevel,
) *FamilyMemberResource {
	return &FamilyMemberResource{
		ID:           member.ID,
		User:         user,
		Relationship: member.Relationship,
	}
}

// FormatUserSearchResponse formats user search response
// CRITICAL: Laravel has typo - uses 'date' instead of 'data'!
func FormatUserSearchResponse(results interface{}) map[string]interface{} {
	return map[string]interface{}{
		"date": results, // Yes, 'date' not 'data'! Must preserve typo!
	}
}
