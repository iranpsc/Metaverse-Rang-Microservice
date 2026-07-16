package handler

import (
	"context"
	"fmt"

	"metarang/features-service/internal/service"
	pb "metarang/shared/pb/features"
	"metarang/shared/pkg/helpers"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProfitHandler struct {
	pb.UnimplementedFeatureProfitServiceServer
	service service.ProfitServiceInterface
}

func NewProfitHandler(service service.ProfitServiceInterface) *ProfitHandler {
	return &ProfitHandler{
		service: service,
	}
}

// GetHourlyProfits retrieves all hourly profits for a user with totals by karbari
// Implements Laravel's FeatureHourlyProfitController@index
// Returns HourlyProfitResource format with feature_db_id, feature_id (properties.id), karbari, formatted amounts (3 decimals), and Jalali dates
func (h *ProfitHandler) GetHourlyProfits(ctx context.Context, req *pb.GetHourlyProfitsRequest) (*pb.HourlyProfitsResponse, error) {
	locale := GetProjectLocale()
	validationErrors := ValidateRequired("user_id", req.UserId, locale)
	if len(validationErrors) > 0 {
		return nil, ReturnValidationError(validationErrors)
	}

	// Default page size to 10 if not specified (matching Laravel's simplePaginate(10))
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}

	profits, totalMaskoni, totalTejari, totalAmozeshi, hasMore, err := h.service.GetHourlyProfits(ctx, req.UserId, page, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get hourly profits: %v", err)
	}

	// Convert internal models to protobuf with proper formatting
	// Matching Laravel's HourlyProfitResource format
	profitsProto := []*pb.HourlyProfit{}
	for _, p := range profits {
		// Format amount with 3 decimals (matching Laravel's number_format($this->amount, 3))
		amountFormatted := fmt.Sprintf("%.3f", p.Amount)

		// Format deadline as Jalali date (Y/m/d format)
		deadlineJalali := helpers.FormatJalaliDate(p.Deadline)

		profitProto := &pb.HourlyProfit{
			Id:           p.ID,
			FeatureId:    p.FeatureID,
			FeatureDbId:  p.FeatureDBID,
			PropertiesId: p.PropertiesID,
			Karbari:      p.Karbari,
			UserId:       p.UserID,
			Asset:        p.Asset,
			Amount:       amountFormatted,
			DeadLine:     deadlineJalali,
			IsActive:     p.IsActive,
		}

		profitsProto = append(profitsProto, profitProto)
	}

	return &pb.HourlyProfitsResponse{
		Profits:             profitsProto,
		TotalMaskoniProfit:  totalMaskoni,
		TotalTejariProfit:   totalTejari,
		TotalAmozeshiProfit: totalAmozeshi,
		HasMore:             hasMore,
	}, nil
}

// GetSingleProfit retrieves and processes a single profit
// Implements Laravel's FeatureHourlyProfitController@getSingleProfit
// Returns HourlyProfitResource format after crediting wallet and resetting profit
func (h *ProfitHandler) GetSingleProfit(ctx context.Context, req *pb.GetSingleProfitRequest) (*pb.HourlyProfitResponse, error) {
	locale := GetProjectLocale()
	validationErrors := MergeValidationErrors(
		ValidateRequired("profit_id", req.ProfitId, locale),
		ValidateRequired("user_id", req.UserId, locale),
	)
	if len(validationErrors) > 0 {
		return nil, ReturnValidationError(validationErrors)
	}

	profit, err := h.service.GetSingleProfit(ctx, req.ProfitId, req.UserId)
	if err != nil {
		if err.Error() == "unauthorized" {
			return nil, status.Errorf(codes.PermissionDenied, "unauthorized")
		}
		return nil, status.Errorf(codes.Internal, "failed to get single profit: %v", err)
	}

	// Format amount with 3 decimals (matching Laravel's number_format($this->amount, 3))
	amountFormatted := fmt.Sprintf("%.3f", profit.Amount)

	// Format deadline as Jalali date (Y/m/d format)
	deadlineJalali := helpers.FormatJalaliDate(profit.Deadline)

	return &pb.HourlyProfitResponse{
		Profit: &pb.HourlyProfit{
			Id:           profit.ID,
			FeatureId:    profit.FeatureID,
			FeatureDbId:  profit.FeatureDBID,
			PropertiesId: profit.PropertiesID,
			Karbari:      profit.Karbari,
			UserId:       profit.UserID,
			Asset:        profit.Asset,
			Amount:       amountFormatted,
			DeadLine:     deadlineJalali,
			IsActive:     profit.IsActive,
		},
		Success: true,
	}, nil
}

// GetProfitsByApplication retrieves profits by karbari (m/t/a) and transfers to wallet
// Implements Laravel's FeatureHourlyProfitController@getProfitsByApplication
// Returns empty JSON object {} (HTTP 200) as per Laravel implementation
func (h *ProfitHandler) GetProfitsByApplication(ctx context.Context, req *pb.GetProfitsByApplicationRequest) (*pb.ProfitsByApplicationResponse, error) {
	locale := GetProjectLocale()
	validationErrors := MergeValidationErrors(
		ValidateRequired("user_id", req.UserId, locale),
		ValidateRequired("karbari", req.Karbari, locale),
		ValidateOneOf("karbari", req.Karbari, []string{"m", "t", "a"}, locale),
	)
	if len(validationErrors) > 0 {
		return nil, ReturnValidationError(validationErrors)
	}

	_, err := h.service.GetProfitsByApplication(ctx, req.UserId, req.Karbari)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get profits by application: %v", err)
	}

	// Return success response (Laravel returns empty JSON object {})
	return &pb.ProfitsByApplicationResponse{
		Success: true,
	}, nil
}

// GetHourlyProfitTimePercentage returns elapsed time percentage for the user's oldest hourly profit.
// Implements Laravel's hourlyProfitInfo helper used by AuthenticatedUserResource.
func (h *ProfitHandler) GetHourlyProfitTimePercentage(ctx context.Context, req *pb.GetHourlyProfitTimePercentageRequest) (*pb.GetHourlyProfitTimePercentageResponse, error) {
	locale := GetProjectLocale()
	validationErrors := ValidateRequired("user_id", req.UserId, locale)
	if len(validationErrors) > 0 {
		return nil, ReturnValidationError(validationErrors)
	}

	percentage, err := h.service.GetHourlyProfitTimePercentage(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get hourly profit time percentage: %v", err)
	}

	return &pb.GetHourlyProfitTimePercentageResponse{
		Percentage: percentage,
	}, nil
}
