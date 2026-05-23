package handler

import (
	"encoding/json"

	pb "metargb/shared/pb/auth"
)

// buildCitizenProfileHTTPResponse maps gRPC CitizenProfileResponse to Laravel PersonalInfo JSON.
// Omits empty optional fields to match Laravel JsonResource mergeWhen behavior.
func buildCitizenProfileHTTPResponse(resp *pb.CitizenProfileResponse) map[string]interface{} {
	if resp == nil {
		return map[string]interface{}{}
	}

	out := map[string]interface{}{}

	if photos := buildProfilePhotos(resp.ProfilePhotos); len(photos) > 0 {
		out["profilePhotos"] = photos
	} else {
		out["profilePhotos"] = []interface{}{}
	}

	if kyc := buildCitizenKYC(resp.Kyc); len(kyc) > 0 {
		out["kyc"] = kyc
	}

	if resp.Code != "" {
		out["code"] = resp.Code
	}
	if resp.Name != "" {
		out["name"] = resp.Name
	}
	if resp.Position != "" {
		out["position"] = resp.Position
	}
	if resp.RegisteredAt != "" {
		out["registered_at"] = resp.RegisteredAt
	}

	if customs := buildCitizenCustomsHTTP(resp.Customs); customs != nil {
		out["customs"] = customs
	}

	if resp.Score >= 0 {
		out["score"] = resp.Score
	}

	// Always present in Laravel PersonalInfo
	out["score_percentage_to_next_level"] = resp.ScorePercentageToNextLevel

	if resp.CurrentLevel != nil {
		out["current_level"] = buildCitizenLevelHTTP(resp.CurrentLevel)
	}
	if levels := buildAchievedLevelsHTTP(resp.AchievedLevels); len(levels) > 0 {
		out["achieved_levels"] = levels
	}

	if resp.Avatar != "" {
		out["avatar"] = resp.Avatar
	}

	return out
}

func buildProfilePhotos(photos []*pb.ProfilePhoto) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(photos))
	for _, p := range photos {
		if p == nil {
			continue
		}
		result = append(result, map[string]interface{}{
			"id":  p.Id,
			"url": p.Url,
		})
	}
	return result
}

func buildCitizenKYC(kyc *pb.CitizenKYC) map[string]interface{} {
	if kyc == nil {
		return nil
	}
	out := map[string]interface{}{}
	if kyc.Nationality != "" {
		out["nationality"] = kyc.Nationality
	}
	if kyc.Fname != "" {
		out["fname"] = kyc.Fname
	}
	if kyc.Lname != "" {
		out["lname"] = kyc.Lname
	}
	if kyc.BirthDate != "" {
		out["birth_date"] = kyc.BirthDate
	}
	if kyc.Phone != "" {
		out["phone"] = kyc.Phone
	}
	if kyc.Email != "" {
		out["email"] = kyc.Email
	}
	if kyc.Address != "" {
		out["address"] = kyc.Address
	}
	return out
}

func buildCitizenCustomsHTTP(customs *pb.CitizenCustoms) map[string]interface{} {
	if customs == nil {
		return nil
	}
	out := map[string]interface{}{}
	has := false

	if customs.Occupation != "" {
		out["occupation"] = customs.Occupation
		has = true
	}
	if customs.Education != "" {
		out["education"] = customs.Education
		has = true
	}
	if customs.LovedCity != "" {
		out["loved_city"] = customs.LovedCity
		has = true
	}
	if customs.LovedCountry != "" {
		out["loved_country"] = customs.LovedCountry
		has = true
	}
	if customs.LovedLanguage != "" {
		out["loved_language"] = customs.LovedLanguage
		has = true
	}
	if customs.Prediction != "" {
		out["prediction"] = customs.Prediction
		has = true
	}
	if customs.Memory != "" {
		out["memory"] = customs.Memory
		has = true
	}
	if customs.About != "" {
		out["about"] = customs.About
		has = true
	}
	if len(customs.Passions) > 0 {
		out["passions"] = customs.Passions
		has = true
	}

	if !has {
		return nil
	}
	return out
}

func buildCitizenLevelHTTP(level *pb.CitizenLevel) map[string]interface{} {
	if level == nil {
		return nil
	}
	out := map[string]interface{}{
		"id": level.Id,
	}
	if level.Name != "" {
		out["name"] = level.Name
	}
	if level.Slug != "" {
		out["slug"] = level.Slug
	}
	if level.Score != 0 {
		out["score"] = level.Score
	}
	if level.Image != "" {
		out["image"] = level.Image
	}
	return out
}

func buildAchievedLevelsHTTP(levels []*pb.CitizenLevel) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(levels))
	for _, level := range levels {
		if level == nil {
			continue
		}
		result = append(result, buildCitizenLevelHTTP(level))
	}
	return result
}

// citizenProfileJSONRoundTrip ensures the HTTP payload matches expected Laravel keys.
func citizenProfileJSONRoundTrip(data map[string]interface{}) map[string]interface{} {
	raw, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return data
	}
	return out
}
