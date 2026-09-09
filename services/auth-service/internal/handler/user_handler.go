package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
	sharedauth "metarang/shared/pkg/auth"
	"metarang/shared/pkg/helpers"
)

type userHandler struct {
	pb.UnimplementedUserServiceServer
	userService              service.UserService
	profileLimitationService service.ProfileLimitationService
	helperService            service.HelperService
}

func RegisterUserHandler(grpcServer *grpc.Server, userService service.UserService, profileLimitationService service.ProfileLimitationService, helperService service.HelperService) pb.UserServiceServer {
	h := NewUserHandler(userService, profileLimitationService, helperService)
	pb.RegisterUserServiceServer(grpcServer, h)
	return h
}

func (h *userHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	if err := authorizeSelfOrService(ctx, req.UserId); err != nil {
		return nil, err
	}

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

	return response, nil
}

func (h *userHandler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.User, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.userService.UpdateProfile(ctx, userID, req.Name, req.Email, req.Phone)
	if err != nil {
		if errors.Is(err, service.ErrInvalidEmailFormat) {
			return nil, status.Error(codes.InvalidArgument, helpers.EncodeValidationError(map[string]string{
				"email": "invalid email format",
			}))
		}
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
	callerUserID, err := resolveProfileLimitationCaller(ctx, req.GetCallerUserId())
	if err != nil {
		return nil, err
	}

	limitation, err := h.profileLimitationService.GetBetweenUsers(ctx, callerUserID, req.TargetUserId)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to get profile limitations: %v", err)
	}

	// If no limitation exists, return empty response ({ "data": [] } at the gateway)
	if limitation == nil {
		return &pb.GetProfileLimitationsResponse{}, nil
	}

	return &pb.GetProfileLimitationsResponse{
		Data: convertProfileLimitationToProto(limitation, callerUserID),
	}, nil
}

// resolveProfileLimitationCaller returns the authenticated user for user-facing calls.
// Trusted inter-service callers (valid service token) may supply caller_user_id explicitly
// so social-service can check both pair and self profile limitations without a user bearer.
func resolveProfileLimitationCaller(ctx context.Context, requestedCallerID uint64) (uint64, error) {
	if sharedauth.HasValidServiceToken(ctx) {
		if requestedCallerID == 0 {
			return 0, status.Error(codes.InvalidArgument, "caller_user_id is required")
		}
		return requestedCallerID, nil
	}
	return authenticatedUserID(ctx)
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

	currentPage := int32(page)
	hasMore := int32(len(users)) >= limit

	response.Meta = &pb.PaginationMeta{
		CurrentPage: currentPage,
	}
	if hasMore {
		response.Meta.NextPageUrl = fmt.Sprintf("?page=%d", currentPage+1)
	}

	_ = totalCount

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
			Gem:      levelGemProto(levelsData.LatestLevel.GemPngFile),
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
			Gem:      levelGemProto(prevLevel.GemPngFile),
		}
		response.Data.PreviousLevels = append(response.Data.PreviousLevels, level)
	}

	return response, nil
}

func levelGemProto(pngFile string) *pb.LevelGem {
	if pngFile == "" {
		return nil
	}
	return &pb.LevelGem{PngFile: pngFile}
}

// GetUserProfile handles GET /api/users/{user}/profile
func (h *userHandler) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
	var viewerUserID *uint64
	if userCtx, err := sharedauth.GetUserFromContext(ctx); err == nil && userCtx != nil && userCtx.UserID > 0 {
		viewerUserID = &userCtx.UserID
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
