package handler

import (
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/models"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	"metarang/shared/pkg/helpers"
)

// Helper functions shared across all handlers

func formatJalaliDate(t time.Time) string {
	return helpers.FormatJalaliDate(t)
}

func formatJalaliDateTime(t time.Time) string {
	return helpers.FormatJalaliDateTime(t)
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func buildDynastyFeature(details map[string]interface{}, memberCount int32, updatedAt time.Time) *dynastypb.DynastyFeature {
	if details == nil {
		return nil
	}

	profitIncrease := "0"
	if stability, ok := details["stability"].(string); ok {
		if stabilityInt, err := strconv.ParseFloat(stability, 64); err == nil && stabilityInt > 10000 {
			profitIncrease = fmt.Sprintf("%.3f", stabilityInt/10000-1)
		}
	}

	return &dynastypb.DynastyFeature{
		Id:                    getUint64(details["id"]),
		PropertiesId:          getString(details["properties_id"]),
		Area:                  getString(details["area"]),
		Density:               getString(details["density"]),
		FeatureProfitIncrease: profitIncrease,
		FamilyMembersCount:    memberCount,
		LastUpdated:           formatJalaliDateTime(updatedAt),
	}
}

func buildAvailableFeatures(features []map[string]interface{}) []*dynastypb.AvailableFeature {
	var result []*dynastypb.AvailableFeature
	for _, f := range features {
		result = append(result, &dynastypb.AvailableFeature{
			Id:           getUint64(f["id"]),
			PropertiesId: getString(f["properties_id"]),
			Density:      getString(f["density"]),
			Stability:    getString(f["stability"]),
			Area:         getString(f["area"]),
		})
	}
	return result
}

func memberTitle(member string) string {
	switch member {
	case "brother":
		return "برادر"
	case "sister":
		return "خواهر"
	case "offspring":
		return "فرزند"
	case "father":
		return "پدر"
	case "mother":
		return "مادر"
	case "husband":
		return "شوهر"
	case "wife":
		return "زن"
	default:
		return member
	}
}

func buildIntroductionPrizes(prizes []*models.DynastyPrize, pscRate float64) []*dynastypb.IntroductionPrize {
	if pscRate == 0 {
		pscRate = 1
	}

	var result []*dynastypb.IntroductionPrize
	for _, prize := range prizes {
		psc := fmt.Sprintf("%.2f", float64(prize.PSC)/pscRate)
		result = append(result, &dynastypb.IntroductionPrize{
			Member:                     memberTitle(prize.Member),
			Satisfaction:               prize.Satisfaction,
			IntroductionProfitIncrease: int32(prize.IntroductionProfitIncrease * 100),
			AccumulatedCapitalReserve:  int32(prize.AccumulatedCapitalReserve * 100),
			DataStorage:                int32(prize.DataStorage * 100),
			Psc:                        psc,
		})
	}
	return result
}

func buildJoinRequestResponse(req *models.JoinRequest, userInfo *models.UserBasic, prize *models.DynastyPrize) *dynastypb.JoinRequestResponse {
	resp := &dynastypb.JoinRequestResponse{
		Id:           req.ID,
		FromUser:     req.FromUser,
		ToUser:       req.ToUser,
		Status:       int32(req.Status),
		Relationship: req.Relationship,
		CreatedAt:    formatJalaliDateTime(req.CreatedAt),
	}

	if req.Message != nil {
		resp.Message = *req.Message
	}

	if userInfo != nil {
		resp.ToUserInfo = buildUserBasic(userInfo)
	}

	if prize != nil {
		resp.RequestPrize = buildDynastyPrize(prize)
	}

	return resp
}

func buildDynastyPrize(prize *models.DynastyPrize) *dynastypb.DynastyPrize {
	return &dynastypb.DynastyPrize{
		Id:                         prize.ID,
		Member:                     prize.Member,
		Satisfaction:               fmt.Sprintf("%.0f", prize.Satisfaction*100),
		IntroductionProfitIncrease: fmt.Sprintf("%.0f", prize.IntroductionProfitIncrease*100),
		AccumulatedCapitalReserve:  fmt.Sprintf("%.0f", prize.AccumulatedCapitalReserve*100),
		DataStorage:                fmt.Sprintf("%.0f", prize.DataStorage*100),
		Psc:                        int32(prize.PSC),
	}
}

func buildUserBasic(user *models.UserBasic) *commonpb.UserBasic {
	if user == nil {
		return nil
	}
	return &commonpb.UserBasic{
		Id:           user.ID,
		Code:         user.Code,
		Name:         user.Name,
		ProfilePhoto: stringOrEmpty(user.ProfilePhoto),
	}
}

func getUint64(v interface{}) uint64 {
	switch val := v.(type) {
	case uint64:
		return val
	case int64:
		return uint64(val)
	case int:
		return uint64(val)
	case uint:
		return uint64(val)
	case float64:
		return uint64(val)
	default:
		return 0
	}
}

func getString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func mapServiceError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Map common errors to gRPC status codes
	switch {
	case contains(errStr, "not found"):
		return status.Errorf(codes.NotFound, "%s", errStr)
	case contains(errStr, "unauthorized") || contains(errStr, "permission denied"):
		return status.Errorf(codes.PermissionDenied, "%s", errStr)
	case contains(errStr, "invalid") || contains(errStr, "validation"):
		return status.Errorf(codes.InvalidArgument, "%s", errStr)
	case contains(errStr, "already exists") || contains(errStr, "already has") || contains(errStr, "duplicate"):
		return status.Errorf(codes.AlreadyExists, "%s", errStr)
	default:
		return status.Errorf(codes.Internal, "%s", errStr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOfSubstring(s, substr) >= 0)))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
