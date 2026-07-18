package handler

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
	"metarang/shared/pkg/helpers"
)

type citizenHandler struct {
	pb.UnimplementedCitizenServiceServer
	citizenService service.CitizenService
}

func RegisterCitizenHandler(grpcServer *grpc.Server, citizenService service.CitizenService) {
	pb.RegisterCitizenServiceServer(grpcServer, &citizenHandler{
		citizenService: citizenService,
	})
}

// GetCitizenUserInfo returns user_id + privacy for public citizen feature assets.
func (h *citizenHandler) GetCitizenUserInfo(ctx context.Context, req *pb.GetCitizenUserInfoRequest) (*pb.GetCitizenUserInfoResponse, error) {
	if req.Code == "" {
		locale := "en"
		t := helpers.GetLocaleTranslations(locale)
		validationErrors := map[string]string{
			"code": fmt.Sprintf(t.Required, "code"),
		}
		return nil, returnValidationError(validationErrors)
	}

	info, err := h.citizenService.GetCitizenUserInfo(ctx, req.Code)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get citizen user info: %v", err)
	}
	if info == nil {
		return nil, status.Errorf(codes.NotFound, "citizen not found")
	}

	privacy := info.Privacy
	if privacy == nil {
		privacy = map[string]int32{}
	}

	return &pb.GetCitizenUserInfoResponse{
		UserId:  info.UserID,
		Privacy: privacy,
	}, nil
}

// GetCitizenProfile returns the public profile for a citizen identified by code.
func (h *citizenHandler) GetCitizenProfile(ctx context.Context, req *pb.GetCitizenProfileRequest) (*pb.CitizenProfileResponse, error) {
	if req.Code == "" {
		locale := "en"
		t := helpers.GetLocaleTranslations(locale)
		validationErrors := map[string]string{
			"code": fmt.Sprintf(t.Required, "code"),
		}
		return nil, returnValidationError(validationErrors)
	}

	profile, err := h.citizenService.GetCitizenProfile(ctx, req.Code)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get citizen profile: %v", err)
	}
	if profile == nil {
		return nil, status.Errorf(codes.NotFound, "citizen not found")
	}

	return buildCitizenProfileResponse(ctx, h.citizenService, profile), nil
}

func buildCitizenProfileResponse(ctx context.Context, svc service.CitizenService, profile *models.CitizenProfile) *pb.CitizenProfileResponse {
	privacy := profile.Privacy
	check := func(field string) bool {
		if privacy == nil {
			return false
		}
		return privacy[field]
	}

	resp := &pb.CitizenProfileResponse{
		ScorePercentageToNextLevel: svc.ScorePercentageToNextLevel(ctx, profile.ID, profile.Score),
	}

	for _, photo := range profile.ProfilePhotos {
		resp.ProfilePhotos = append(resp.ProfilePhotos, &pb.ProfilePhoto{
			Id:  photo.ID,
			Url: photo.URL,
		})
	}

	if profile.KYC != nil {
		kyc := &pb.CitizenKYC{}
		hasKYC := false
		if check("nationality") {
			kyc.Nationality = svc.NationalityFlagURL()
			hasKYC = true
		}
		if check("fname") && profile.KYC.Fname != "" {
			kyc.Fname = profile.KYC.Fname
			hasKYC = true
		}
		if check("lname") && profile.KYC.Lname != "" {
			kyc.Lname = profile.KYC.Lname
			hasKYC = true
		}
		if check("birthdate") && !profile.KYC.Birthdate.IsZero() {
			kyc.BirthDate = service.FormatJalaliDate(profile.KYC.Birthdate)
			hasKYC = true
		}
		if check("phone") && profile.Phone != "" {
			kyc.Phone = profile.Phone
			hasKYC = true
		}
		if check("email") && profile.Email != "" {
			kyc.Email = profile.Email
			hasKYC = true
		}
		if check("address") && profile.KYC.Address != "" {
			kyc.Address = profile.KYC.Address
			hasKYC = true
		}
		if hasKYC {
			resp.Kyc = kyc
		}
	}

	if check("code") {
		resp.Code = profile.Code
	}
	if check("name") {
		resp.Name = profile.Name
	}
	if check("position") {
		resp.Position = svc.CitizenPosition()
	}
	if check("registered_at") && !profile.EmailVerifiedAt.IsZero() {
		resp.RegisteredAt = service.FormatRegisteredAt(profile.EmailVerifiedAt)
	}

	if profile.PersonalInfo != nil {
		customs := buildCitizenCustoms(profile.PersonalInfo, check, svc)
		if customs != nil {
			resp.Customs = customs
		}
	}

	if check("score") {
		resp.Score = profile.Score
	} else {
		resp.Score = -1 // sentinel: hidden from public profile (gateway omits score key)
	}

	if profile.CurrentLevel != nil && check("level") {
		resp.CurrentLevel = citizenLevelToProto(profile.CurrentLevel)
		for _, level := range profile.AchievedLevels {
			resp.AchievedLevels = append(resp.AchievedLevels, citizenLevelToProto(level))
		}
	}

	if check("avatar") {
		resp.Avatar = svc.CitizenAvatar()
	}

	return resp
}

func buildCitizenCustoms(
	pi *models.CitizenPersonalInfo,
	check func(string) bool,
	svc service.CitizenService,
) *pb.CitizenCustoms {
	customs := &pb.CitizenCustoms{}
	hasField := false

	if check("occupation") && pi.Occupation != "" {
		customs.Occupation = pi.Occupation
		hasField = true
	}
	if check("education") && pi.Education != "" {
		customs.Education = pi.Education
		hasField = true
	}
	if check("loved_city") && pi.LovedCity != "" {
		customs.LovedCity = pi.LovedCity
		hasField = true
	}
	if check("loved_country") && pi.LovedCountry != "" {
		customs.LovedCountry = pi.LovedCountry
		hasField = true
	}
	if check("loved_language") && pi.LovedLanguage != "" {
		customs.LovedLanguage = pi.LovedLanguage
		hasField = true
	}
	if check("prediction") && pi.Prediction != "" {
		customs.Prediction = pi.Prediction
		hasField = true
	}
	if check("memory") && pi.Memory != "" {
		customs.Memory = pi.Memory
		hasField = true
	}
	if check("about") && pi.About != "" {
		customs.About = pi.About
		hasField = true
	}

	if check("passions") && pi.Passions != nil {
		passions := make(map[string]string)
		for _, key := range passionKeysList() {
			if pi.Passions[key] {
				passions[key] = svc.PassionIconURL(key)
			}
		}
		if len(passions) > 0 {
			customs.Passions = passions
			hasField = true
		}
	}

	if !hasField {
		return nil
	}
	return customs
}

func citizenLevelToProto(level *models.CitizenLevel) *pb.CitizenLevel {
	if level == nil {
		return nil
	}
	return &pb.CitizenLevel{
		Id:    level.ID,
		Name:  level.Name,
		Slug:  level.Slug,
		Score: level.Score,
		Image: level.Image,
	}
}

func passionKeysList() []string {
	return []string{
		"music", "sport_health", "art", "language_culture", "philosophy",
		"animals_nature", "aliens", "food_cooking", "travel_leature", "manufacturing",
		"science_technology", "space_time", "history", "politics_economy",
	}
}

// GetCitizenReferrals lists referrals for a citizen with pagination
func (h *citizenHandler) GetCitizenReferrals(ctx context.Context, req *pb.GetCitizenReferralsRequest) (*pb.CitizenReferralsResponse, error) {
	if req.Code == "" {
		locale := "en"
		t := helpers.GetLocaleTranslations(locale)
		validationErrors := map[string]string{
			"code": fmt.Sprintf(t.Required, "code"),
		}
		return nil, returnValidationError(validationErrors)
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}

	referrals, meta, err := h.citizenService.GetCitizenReferrals(ctx, req.Code, req.Search, page)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get referrals: %v", err)
	}
	if referrals == nil {
		return &pb.CitizenReferralsResponse{
			Data: []*pb.CitizenReferral{},
			Meta: &pb.PaginationMeta{CurrentPage: 1},
		}, nil
	}

	response := &pb.CitizenReferralsResponse{
		Meta: &pb.PaginationMeta{
			CurrentPage: meta.CurrentPage,
			NextPageUrl: meta.NextPageURL,
			PrevPageUrl: meta.PrevPageURL,
		},
	}

	for _, ref := range referrals {
		pbRef := &pb.CitizenReferral{
			Id:             ref.ID,
			Code:           ref.Code,
			Name:           ref.Name,
			ReferrerOrders: []*pb.ReferrerOrder{},
		}
		if ref.Image != "" {
			pbRef.Image = ref.Image
		}
		for _, order := range ref.ReferrerOrders {
			pbRef.ReferrerOrders = append(pbRef.ReferrerOrders, &pb.ReferrerOrder{
				Id:        order.ID,
				Amount:    order.Amount,
				CreatedAt: formatJalaliDateTime(order.CreatedAt),
			})
		}
		response.Data = append(response.Data, pbRef)
	}

	return response, nil
}

// GetCitizenReferralChart provides aggregated referral analytics
func (h *citizenHandler) GetCitizenReferralChart(ctx context.Context, req *pb.GetCitizenReferralChartRequest) (*pb.CitizenReferralChartResponse, error) {
	if req.Code == "" {
		locale := "en"
		t := helpers.GetLocaleTranslations(locale)
		validationErrors := map[string]string{
			"code": fmt.Sprintf(t.Required, "code"),
		}
		return nil, returnValidationError(validationErrors)
	}

	rangeType := req.Range
	if rangeType == "" {
		rangeType = "daily"
	}

	chartData, err := h.citizenService.GetCitizenReferralChart(ctx, req.Code, rangeType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get chart data: %v", err)
	}
	if chartData == nil {
		return &pb.CitizenReferralChartResponse{
			Data: &pb.ReferralChartData{
				TotalReferralsCount:       "0",
				TotalReferralOrdersAmount: "0",
				ChartData:                 []*pb.ChartDataPoint{},
			},
		}, nil
	}

	response := &pb.CitizenReferralChartResponse{
		Data: &pb.ReferralChartData{
			TotalReferralsCount:       chartData.TotalReferralsCount,
			TotalReferralOrdersAmount: chartData.TotalReferralOrdersAmount,
		},
	}

	for _, point := range chartData.ChartData {
		response.Data.ChartData = append(response.Data.ChartData, &pb.ChartDataPoint{
			Label:       point.Label,
			Count:       point.Count,
			TotalAmount: point.TotalAmount,
		})
	}

	return response, nil
}

func formatJalaliDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return service.FormatJalaliDateTime(t)
}
