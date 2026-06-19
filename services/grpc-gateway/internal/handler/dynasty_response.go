package handler

import (
	"strings"

	commonpb "metargb/shared/pb/common"
	dynastypb "metargb/shared/pb/dynasty"
)

// buildDynastyHTTPResponse formats a dynasty gRPC response to match Laravel DynastyResource JSON.
func buildDynastyHTTPResponse(resp *dynastypb.DynastyResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{}
	}

	data := map[string]interface{}{
		"user-has-dynasty": resp.UserHasDynasty,
	}

	if resp.UserHasDynasty {
		data["id"] = resp.Id
		data["family_id"] = resp.FamilyId
		data["created_at"] = resp.CreatedAt
		if resp.ProfileImage != "" {
			data["profile-image"] = resp.ProfileImage
		}
		if resp.DynastyFeature != nil {
			data["dynasty-feature"] = buildDynastyFeatureHTTP(resp.DynastyFeature)
		}
	}

	if len(resp.Features) > 0 {
		features := make([]map[string]interface{}, 0, len(resp.Features))
		for _, feature := range resp.Features {
			features = append(features, map[string]interface{}{
				"id":            feature.Id,
				"properties_id": feature.PropertiesId,
				"density":       feature.Density,
				"stability":     feature.Stability,
				"area":          feature.Area,
			})
		}
		data["features"] = features
	}

	if len(resp.Prizes) > 0 {
		prizes := make([]map[string]interface{}, 0, len(resp.Prizes))
		for _, prize := range resp.Prizes {
			prizes = append(prizes, map[string]interface{}{
				"member":                       prize.Member,
				"satisfaction":                 prize.Satisfaction,
				"introduction_profit_increase":   prize.IntroductionProfitIncrease,
				"accumulated_capital_reserve":  prize.AccumulatedCapitalReserve,
				"data_storage":                 prize.DataStorage,
				"psc":                          prize.Psc,
			})
		}
		data["prizes"] = prizes
	}

	return data
}

func buildDynastyFeatureHTTP(feature *dynastypb.DynastyFeature) map[string]interface{} {
	return map[string]interface{}{
		"id":                      feature.Id,
		"properties_id":           feature.PropertiesId,
		"area":                    feature.Area,
		"density":                 feature.Density,
		"feature-profit-increase": feature.FeatureProfitIncrease,
		"family-members-count":    feature.FamilyMembersCount,
		"last-updated":            feature.LastUpdated,
	}
}

// buildFamilyMembersHTTPResponse formats GET /api/dynasty/{dynasty}/family/{family}
// to match Laravel FamilyMemberResource::collection().
func buildFamilyMembersHTTPResponse(resp *dynastypb.FamilyResponse) []map[string]interface{} {
	if resp == nil || len(resp.Members) == 0 {
		return []map[string]interface{}{}
	}

	members := make([]map[string]interface{}, 0, len(resp.Members))
	for _, member := range resp.Members {
		item := map[string]interface{}{
			"relationship": member.Relationship,
			"online":         false,
		}

		if member.UserInfo != nil {
			item["id"] = member.UserInfo.Id
			item["code"] = member.UserInfo.Code
			if member.UserInfo.ProfilePhoto != "" {
				item["profile_photo"] = member.UserInfo.ProfilePhoto
			}
		} else if member.UserId > 0 {
			item["id"] = member.UserId
		}

		members = append(members, item)
	}

	return members
}

func buildSentJoinRequestsHTTPResponse(resp *dynastypb.JoinRequestsResponse) []map[string]interface{} {
	if resp == nil || len(resp.Requests) == 0 {
		return []map[string]interface{}{}
	}

	requests := make([]map[string]interface{}, 0, len(resp.Requests))
	for _, req := range resp.Requests {
		requests = append(requests, buildSentJoinRequestHTTP(req))
	}
	return requests
}

func buildReceivedJoinRequestsHTTPResponse(resp *dynastypb.JoinRequestsResponse) []map[string]interface{} {
	if resp == nil || len(resp.Requests) == 0 {
		return []map[string]interface{}{}
	}

	requests := make([]map[string]interface{}, 0, len(resp.Requests))
	for _, req := range resp.Requests {
		requests = append(requests, buildReceivedJoinRequestHTTP(req))
	}
	return requests
}

func buildSentJoinRequestHTTP(req *dynastypb.JoinRequestResponse) map[string]interface{} {
	if req == nil {
		return map[string]interface{}{}
	}

	date, timeValue := splitJalaliDateTime(req.CreatedAt)
	item := map[string]interface{}{
		"id":           req.Id,
		"status":       req.Status,
		"relationship": relationshipTitle(req.Relationship),
		"date":         date,
		"time":         timeValue,
	}

	if req.ToUserInfo != nil {
		item["to_user"] = buildJoinRequestUserHTTP(req.ToUserInfo)
	}

	if req.RequestPrize != nil {
		item["prize"] = buildJoinRequestPrizeHTTP(req.RequestPrize)
	}

	return item
}

func buildReceivedJoinRequestHTTP(req *dynastypb.JoinRequestResponse) map[string]interface{} {
	if req == nil {
		return map[string]interface{}{}
	}

	date, timeValue := splitJalaliDateTime(req.CreatedAt)
	item := map[string]interface{}{
		"id":           req.Id,
		"status":       req.Status,
		"relationship": relationshipTitle(req.Relationship),
		"date":         date,
		"time":         timeValue,
	}

	if req.ToUserInfo != nil {
		item["from_user"] = buildJoinRequestUserHTTP(req.ToUserInfo)
	}

	return item
}

func buildJoinRequestUserHTTP(user *commonpb.UserBasic) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{}
	}

	result := map[string]interface{}{
		"id":   user.Id,
		"code": user.Code,
		"name": user.Name,
	}
	if user.ProfilePhoto != "" {
		result["profile_photo"] = user.ProfilePhoto
	}
	return result
}

func buildJoinRequestPrizeHTTP(prize *dynastypb.DynastyPrize) map[string]interface{} {
	if prize == nil {
		return nil
	}

	result := map[string]interface{}{
		"id":     prize.Id,
		"psc":    prize.Psc,
		"member": prize.Member,
	}
	if prize.Satisfaction != "" {
		result["satisfaction"] = prize.Satisfaction
	}
	if prize.IntroductionProfitIncrease != "" {
		result["introducation_profit_increase"] = prize.IntroductionProfitIncrease
	}
	if prize.AccumulatedCapitalReserve != "" {
		result["accumulated_capital_reserve"] = prize.AccumulatedCapitalReserve
	}
	if prize.DataStorage != "" {
		result["data_storage"] = prize.DataStorage
	}
	return result
}

func relationshipTitle(relationship string) string {
	switch relationship {
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
	case "spouse":
		return "همسر"
	default:
		return relationship
	}
}

func splitJalaliDateTime(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}

	parts := strings.Fields(value)
	if len(parts) == 1 {
		return parts[0], ""
	}

	timeValue := parts[1]
	if strings.Count(timeValue, ":") >= 2 {
		timeParts := strings.Split(timeValue, ":")
		timeValue = timeParts[0] + ":" + timeParts[1]
	}

	return parts[0], timeValue
}
