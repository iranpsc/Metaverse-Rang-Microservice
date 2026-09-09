package handler

import (
	"strings"

	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
)

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
		data["features"] = buildSelectedFeatureHTTP(resp)
	} else if len(resp.Features) > 0 {
		features := make([]map[string]interface{}, 0, len(resp.Features))
		for _, feature := range resp.Features {
			features = append(features, availableFeatureHTTP(feature))
		}
		data["features"] = features
	}

	if len(resp.Prizes) > 0 {
		prizes := make([]map[string]interface{}, 0, len(resp.Prizes))
		for _, prize := range resp.Prizes {
			prizes = append(prizes, map[string]interface{}{
				"member":                       prize.Member,
				"satisfaction":                 prize.Satisfaction,
				"introduction_profit_increase": prize.IntroductionProfitIncrease,
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

func buildSelectedFeatureHTTP(resp *dynastypb.DynastyResponse) map[string]interface{} {
	if resp == nil || resp.DynastyFeature == nil {
		return map[string]interface{}{
			"id":            "",
			"properties_id": "",
			"density":       "",
			"area":          "",
			"stability":     "",
		}
	}

	stability := ""
	for _, feature := range resp.Features {
		if feature != nil && feature.Id == resp.DynastyFeature.Id {
			stability = feature.Stability
			break
		}
	}

	return map[string]interface{}{
		"id":            resp.DynastyFeature.Id,
		"properties_id": resp.DynastyFeature.PropertiesId,
		"density":       resp.DynastyFeature.Density,
		"area":          resp.DynastyFeature.Area,
		"stability":     stability,
	}
}

func availableFeatureHTTP(feature *dynastypb.AvailableFeature) map[string]interface{} {
	if feature == nil {
		return map[string]interface{}{
			"id":            "",
			"properties_id": "",
			"density":       "",
			"stability":     "",
			"area":          "",
		}
	}
	return map[string]interface{}{
		"id":            feature.Id,
		"properties_id": feature.PropertiesId,
		"density":       feature.Density,
		"stability":     feature.Stability,
		"area":          feature.Area,
	}
}

// buildFamilyMembersHTTPResponse formats GET /api/dynasty/{dynasty}/family/{family}
func buildFamilyMembersHTTPResponse(resp *dynastypb.FamilyResponse) []map[string]interface{} {
	if resp == nil || len(resp.Members) == 0 {
		return []map[string]interface{}{}
	}

	members := make([]map[string]interface{}, 0, len(resp.Members))
	for _, member := range resp.Members {
		item := map[string]interface{}{
			"relationship": member.Relationship,
			"online":       false,
		}

		if member.UserInfo != nil {
			item["id"] = member.UserInfo.Id
			item["code"] = member.UserInfo.Code
			item["level"] = member.UserInfo.Level
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
		"message":      req.Message,
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
		"message":      req.Message,
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

	return parts[0], parts[1]
}

func buildUserSearchHTTPResponse(resp *dynastypb.SearchUsersResponse) []map[string]interface{} {
	if resp == nil || len(resp.Data) == 0 {
		return []map[string]interface{}{}
	}

	out := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		if item == nil {
			continue
		}
		row := map[string]interface{}{
			"id":       item.Id,
			"code":     item.Code,
			"name":     item.Name,
			"image":    nilIfEmpty(item.Image),
			"level":    item.Level,
			"verified": item.Verified,
			"age":      item.Age,
			"levels":   buildUserSearchLevelsHTTP(item.Levels),
		}
		out = append(out, row)
	}
	return out
}

func buildUserSearchLevelsHTTP(levels []*dynastypb.UserSearchLevelItem) []map[string]interface{} {
	if len(levels) == 0 {
		return []map[string]interface{}{}
	}

	out := make([]map[string]interface{}, 0, len(levels))
	for _, lvl := range levels {
		if lvl == nil {
			continue
		}
		item := map[string]interface{}{
			"id":   lvl.Id,
			"slug": lvl.Slug,
			"gem":  nil,
		}
		if lvl.Gem != nil {
			item["gem"] = map[string]interface{}{
				"id":    lvl.Gem.Id,
				"name":  lvl.Gem.Name,
				"image": lvl.Gem.Image,
			}
		}
		out = append(out, item)
	}
	return out
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
