package handler

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
)

type userHandler struct {
	pb.UnimplementedUserServiceServer
	userService              service.UserService
	profileLimitationService service.ProfileLimitationService
	helperService            service.HelperService
}

func RegisterUserHandler(grpcServer *grpc.Server, userService service.UserService, profileLimitationService service.ProfileLimitationService, helperService service.HelperService) {
	pb.RegisterUserServiceServer(grpcServer, &userHandler{
		userService:              userService,
		profileLimitationService: profileLimitationService,
		helperService:            helperService,
	})
}

func (h *userHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	user, err := h.userService.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	response := &pb.User{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Code:      user.Code,
		Score:     user.Score,
		Ip:        user.IP,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}

	if user.Phone.Valid {
		response.Phone = user.Phone.String
	}

	if user.ReferrerID.Valid {
		response.ReferrerId = uint64(user.ReferrerID.Int64)
	}

	if user.LastSeen.Valid {
		response.LastSeen = timestamppb.New(user.LastSeen.Time)
	}

	if user.EmailVerifiedAt.Valid {
		response.EmailVerifiedAt = timestamppb.New(user.EmailVerifiedAt.Time)
	}

	if user.PhoneVerifiedAt.Valid {
		response.PhoneVerifiedAt = timestamppb.New(user.PhoneVerifiedAt.Time)
	}

	if user.AccessToken.Valid {
		response.AccessToken = user.AccessToken.String
	}

	if user.RefreshToken.Valid {
		response.RefreshToken = user.RefreshToken.String
	}

	if user.TokenType.Valid {
		response.TokenType = user.TokenType.String
	}

	if user.ExpiresIn.Valid {
		response.ExpiresIn = user.ExpiresIn.Int64
	}

	return response, nil
}

func (h *userHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.User, error) {
	user, err := h.userService.UpdateProfile(ctx, req.UserId, req.Name, req.Email, req.Phone)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update profile: %v", err)
	}

	response := &pb.User{
		Id:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Code:      user.Code,
		Score:     user.Score,
		Ip:        user.IP,
		CreatedAt: timestamppb.New(user.CreatedAt),
	}

	if user.Phone.Valid {
		response.Phone = user.Phone.String
	}

	return response, nil
}

func (h *userHandler) GetUserWallet(ctx context.Context, req *pb.GetUserWalletRequest) (*pb.UserWalletResponse, error) {
	if h.helperService == nil {
		return nil, status.Errorf(codes.Unimplemented, "wallet service not available")
	}

	// Get wallet from commercial service via helper service
	wallet, err := h.helperService.GetUserWallet(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user wallet: %v", err)
	}

	if wallet == nil {
		// Return empty wallet response
		return &pb.UserWalletResponse{
			Psc:          "0",
			Irr:          "0",
			Red:          "0",
			Blue:         "0",
			Yellow:       "0",
			Satisfaction: "0",
			Effect:       0,
		}, nil
	}

	return &pb.UserWalletResponse{
		Psc:          wallet.Psc,
		Irr:          wallet.Irr,
		Red:          wallet.Red,
		Blue:         wallet.Blue,
		Yellow:       wallet.Yellow,
		Satisfaction: wallet.Satisfaction,
		Effect:       wallet.Effect,
	}, nil
}

func (h *userHandler) GetUserLevel(ctx context.Context, req *pb.GetUserLevelRequest) (*pb.UserLevelResponse, error) {
	if h.helperService == nil {
		return nil, status.Errorf(codes.Unimplemented, "levels service not available")
	}

	// Get user level from levels service via helper service
	level, err := h.helperService.GetUserLevel(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user level: %v", err)
	}

	if level == nil {
		return &pb.UserLevelResponse{
			Level:            nil,
			Score:            0,
			PercentageToNext: 0.0,
		}, nil
	}

	// Get score percentage to next level
	user, err := h.userService.GetUser(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	scorePercentage, err := h.helperService.GetScorePercentageToNextLevel(ctx, req.UserId, user.Score)
	if err != nil {
		// Log error but continue with 0.0
		scorePercentage = 0.0
	}

	return &pb.UserLevelResponse{
		Level: &pb.Level{
			Id:          level.ID,
			Title:       level.Title,
			Description: level.Description,
			Score:       level.Score,
		},
		Score:            user.Score,
		PercentageToNext: scorePercentage,
	}, nil
}

func (h *userHandler) GetProfileLimitations(ctx context.Context, req *pb.GetProfileLimitationsRequest) (*pb.GetProfileLimitationsResponse, error) {
	limitation, err := h.profileLimitationService.GetBetweenUsers(ctx, req.CallerUserId, req.TargetUserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get profile limitations: %v", err)
	}

	// If no limitation exists, return empty response
	if limitation == nil {
		return &pb.GetProfileLimitationsResponse{}, nil
	}

	// Convert to proto - note visibility depends on caller being the limiter
	return &pb.GetProfileLimitationsResponse{
		Data: convertProfileLimitationToProtoForUser(limitation, req.CallerUserId),
	}, nil
}

// ListUsers handles GET /api/users
func (h *userHandler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	page := req.Page
	if page < 1 {
		page = 1
	}

	users, totalCount, limit, err := h.userService.ListUsers(ctx, req.Search, req.OrderBy, page)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list users: %v", err)
	}

	response := &pb.ListUsersResponse{
		Data: make([]*pb.UserListItem, 0, len(users)),
	}

	// Convert service layer users to proto
	for _, user := range users {
		item := &pb.UserListItem{
			Id:    user.ID,
			Name:  user.Name,
			Code:  user.Code,
			Score: user.Score,
		}

		item.Levels = &pb.UserLevelInfo{}
		if user.CurrentLevel != nil {
			item.Levels.Current = userListLevelToProto(user.CurrentLevel)
		}
		for _, lvl := range user.PreviousLevels {
			item.Levels.Previous = append(item.Levels.Previous, userListLevelToProto(lvl))
		}

		// Set profile photo (prepend admin panel URL if needed)
		if user.ProfilePhoto != "" {
			item.ProfilePhoto = user.ProfilePhoto
		}

		response.Data = append(response.Data, item)
	}

	// Laravel simplePaginate: signal next page when this page is full.
	currentPage := int32(page)
	hasMore := int32(len(users)) >= limit

	response.Meta = &pb.PaginationMeta{
		CurrentPage: currentPage,
	}
	if hasMore {
		response.Meta.NextPageUrl = fmt.Sprintf("?page=%d", currentPage+1)
	}

	_ = totalCount // total count is not exposed in Laravel simplePaginate meta

	return response, nil
}

// GetUserLevels handles GET /api/users/{user}/levels
func (h *userHandler) GetUserLevels(ctx context.Context, req *pb.GetUserLevelsRequest) (*pb.GetUserLevelsResponse, error) {
	levelsData, err := h.userService.GetUserLevels(ctx, req.UserId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get user levels: %v", err)
	}

	response := &pb.GetUserLevelsResponse{
		Data: &pb.UserLevelData{
			PreviousLevels:             make([]*pb.Level, 0),
			ScorePercentageToNextLevel: levelsData.ScorePercentageToNextLevel,
		},
	}

	// Convert latest level
	if levelsData.LatestLevel != nil {
		response.Data.LatestLevel = &pb.Level{
			Id:       levelsData.LatestLevel.ID,
			Title:    levelsData.LatestLevel.Name, // Level proto uses Title, not Name
			Score:    levelsData.LatestLevel.Score,
			Slug:     levelsData.LatestLevel.Slug,
			ImageUrl: levelsData.LatestLevel.Image,
		}
	}

	// Convert previous levels
	for _, prevLevel := range levelsData.PreviousLevels {
		level := &pb.Level{
			Id:       prevLevel.ID,
			Title:    prevLevel.Name, // Level proto uses Title, not Name
			Score:    prevLevel.Score,
			Slug:     prevLevel.Slug,
			ImageUrl: prevLevel.Image,
		}
		response.Data.PreviousLevels = append(response.Data.PreviousLevels, level)
	}

	return response, nil
}

// GetUserProfile handles GET /api/users/{user}/profile
func (h *userHandler) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
	var viewerUserID *uint64
	if req.ViewerUserId > 0 {
		viewerUserID = &req.ViewerUserId
	}

	profileData, err := h.userService.GetUserProfile(ctx, req.UserId, viewerUserID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get user profile: %v", err)
	}

	response := &pb.GetUserProfileResponse{
		Data: &pb.UserProfileData{
			Id:            profileData.ID,
			Code:          profileData.Code,
			ProfileImages: profileData.ProfileImages,
		},
	}

	// Set name if privacy allows (empty string if privacy disallows)
	if profileData.Name != nil {
		response.Data.Name = *profileData.Name
	}

	// Set registered_at if privacy allows (empty string if privacy disallows)
	if profileData.RegisteredAt != nil {
		response.Data.RegisteredAt = *profileData.RegisteredAt
	}

	// Set followers_count if privacy allows (0 if privacy disallows)
	if profileData.FollowersCount != nil {
		response.Data.FollowersCount = *profileData.FollowersCount
	}

	// Set following_count if privacy allows (0 if privacy disallows)
	if profileData.FollowingCount != nil {
		response.Data.FollowingCount = *profileData.FollowingCount
	}

	return response, nil
}

// GetUserFeaturesCount handles GET /api/users/{user}/features/count
func (h *userHandler) GetUserFeaturesCount(ctx context.Context, req *pb.GetUserFeaturesCountRequest) (*pb.GetUserFeaturesCountResponse, error) {
	featuresData, err := h.userService.GetUserFeaturesCount(ctx, req.UserId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get feature counts: %v", err)
	}

	response := &pb.GetUserFeaturesCountResponse{
		Data: &pb.UserFeaturesCountData{
			MaskoniFeaturesCount:   featuresData.MaskoniFeaturesCount,
			TejariFeaturesCount:    featuresData.TejariFeaturesCount,
			AmoozeshiFeaturesCount: featuresData.AmoozeshiFeaturesCount,
		},
	}

	return response, nil
}

func userListLevelToProto(lvl *service.LevelSummary) *pb.Level {
	if lvl == nil {
		return nil
	}
	return &pb.Level{
		Id:       lvl.ID,
		Title:    lvl.Name,
		Score:    lvl.Score,
		Slug:     lvl.Slug,
		ImageUrl: lvl.Image,
	}
}

// convertProfileLimitationToProtoForUser converts a ProfileLimitation model to proto for user service
func convertProfileLimitationToProtoForUser(limitation *models.ProfileLimitation, callerUserID uint64) *pb.ProfileLimitation {
	proto := &pb.ProfileLimitation{
		Id:            limitation.ID,
		LimiterUserId: limitation.LimiterUserID,
		LimitedUserId: limitation.LimitedUserID,
		Options: &pb.ProfileLimitationOptions{
			Follow:                limitation.Options.Follow,
			SendMessage:           limitation.Options.SendMessage,
			Share:                 limitation.Options.Share,
			SendTicket:            limitation.Options.SendTicket,
			ViewProfileImages:     limitation.Options.ViewProfileImages,
			ViewFeaturesLocations: limitation.Options.ViewFeaturesLocations,
		},
		CreatedAt: timestamppb.New(limitation.CreatedAt),
		UpdatedAt: timestamppb.New(limitation.UpdatedAt),
	}

	// Only include note if caller is the limiter
	if limitation.Note.Valid && callerUserID == limitation.LimiterUserID {
		proto.Note = limitation.Note.String
	}

	return proto
}
