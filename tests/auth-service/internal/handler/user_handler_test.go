package handler

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
)

// mockUserService implements service.UserService for testing
type mockUserService struct {
	listUsersFunc            func(ctx context.Context, search string, orderBy string, page int32) ([]*service.UserListItem, int32, int32, error)
	getUserLevelsFunc        func(ctx context.Context, userID uint64) (*service.UserLevelsData, error)
	getUserProfileFunc       func(ctx context.Context, userID uint64, viewerUserID *uint64) (*service.UserProfileData, error)
	getUserFeaturesCountFunc func(ctx context.Context, userID uint64) (*service.UserFeaturesCountData, error)
	getUserFunc              func(ctx context.Context, userID uint64) (*models.User, error)
	updateProfileFunc        func(ctx context.Context, userID uint64, name, email, phone string) (*models.User, error)
}

func (m *mockUserService) ListUsers(ctx context.Context, search string, orderBy string, page int32) ([]*service.UserListItem, int32, int32, error) {
	if m.listUsersFunc != nil {
		return m.listUsersFunc(ctx, search, orderBy, page)
	}
	return nil, 0, 0, errors.New("not implemented")
}

func (m *mockUserService) GetUserLevels(ctx context.Context, userID uint64) (*service.UserLevelsData, error) {
	if m.getUserLevelsFunc != nil {
		return m.getUserLevelsFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) GetUserProfile(ctx context.Context, userID uint64, viewerUserID *uint64) (*service.UserProfileData, error) {
	if m.getUserProfileFunc != nil {
		return m.getUserProfileFunc(ctx, userID, viewerUserID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) GetUserFeaturesCount(ctx context.Context, userID uint64) (*service.UserFeaturesCountData, error) {
	if m.getUserFeaturesCountFunc != nil {
		return m.getUserFeaturesCountFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserService) GetUser(ctx context.Context, userID uint64) (*models.User, error) {
	if m.getUserFunc != nil {
		return m.getUserFunc(ctx, userID)
	}
	// This method is not used in tests, but required by interface
	return nil, errors.New("not implemented")
}

func (m *mockUserService) UpdateProfile(ctx context.Context, userID uint64, name, email, phone string) (*models.User, error) {
	if m.updateProfileFunc != nil {
		return m.updateProfileFunc(ctx, userID, name, email, phone)
	}
	return nil, errors.New("not implemented")
}

func TestUserHandler_ListUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("successful list users", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.listUsersFunc = func(ctx context.Context, search string, orderBy string, page int32) ([]*service.UserListItem, int32, int32, error) {
			return []*service.UserListItem{
				{
					ID:    1,
					Name:  "Test User",
					Code:  "hm-1234567",
					Score: 100,
				},
			}, 1, 20, nil
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.ListUsersRequest{
			Page: 1,
		}

		resp, err := handler.ListUsers(ctx, req)
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Errorf("Expected 1 user, got %d", len(resp.Data))
		}

		if resp.Data[0].Id != 1 {
			t.Errorf("Expected user ID 1, got %d", resp.Data[0].Id)
		}

		if resp.Meta == nil {
			t.Error("Expected pagination meta")
		}
	})

	t.Run("list users with search", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.listUsersFunc = func(ctx context.Context, search string, orderBy string, page int32) ([]*service.UserListItem, int32, int32, error) {
			if search != "test" {
				t.Errorf("Expected search 'test', got '%s'", search)
			}
			return []*service.UserListItem{}, 0, 20, nil
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.ListUsersRequest{
			Search: "test",
			Page:   1,
		}

		_, err := handler.ListUsers(ctx, req)
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}
	})

	t.Run("list users service error", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.listUsersFunc = func(ctx context.Context, search string, orderBy string, page int32) ([]*service.UserListItem, int32, int32, error) {
			return nil, 0, 0, errors.New("service error")
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.ListUsersRequest{
			Page: 1,
		}

		_, err := handler.ListUsers(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.Internal {
			t.Errorf("Expected Internal error code, got %v", st.Code())
		}
	})
}

func TestUserHandler_GetUserLevels(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get user levels", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.getUserLevelsFunc = func(ctx context.Context, userID uint64) (*service.UserLevelsData, error) {
			return &service.UserLevelsData{
				LatestLevel: &service.LevelDetail{
					ID:    1,
					Name:  "Beginner",
					Score: 100,
					Slug:  "beginner",
					Image: "https://admin.example.com/uploads/level1.png",
				},
				ScorePercentageToNextLevel: 42.5,
			}, nil
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.GetUserLevelsRequest{
			UserId: 1,
		}

		resp, err := handler.GetUserLevels(ctx, req)
		if err != nil {
			t.Fatalf("GetUserLevels failed: %v", err)
		}

		if resp.Data == nil {
			t.Fatal("Expected level data")
		}

		if resp.Data.LatestLevel == nil {
			t.Fatal("Expected latest level")
		}

		if resp.Data.LatestLevel.Id != 1 {
			t.Errorf("Expected level ID 1, got %d", resp.Data.LatestLevel.Id)
		}

		if resp.Data.LatestLevel.ImageUrl != "https://admin.example.com/uploads/level1.png" {
			t.Errorf("Expected level image URL, got %q", resp.Data.LatestLevel.ImageUrl)
		}

		if resp.Data.ScorePercentageToNextLevel != 42.5 {
			t.Errorf("Expected score percentage 42.5, got %f", resp.Data.ScorePercentageToNextLevel)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.getUserLevelsFunc = func(ctx context.Context, userID uint64) (*service.UserLevelsData, error) {
			return nil, errors.New("user not found")
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.GetUserLevelsRequest{
			UserId: 999,
		}

		_, err := handler.GetUserLevels(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})
}

func TestUserHandler_GetUserProfile(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get user profile", func(t *testing.T) {
		mockService := &mockUserService{}
		name := "Test User"
		registeredAt := "1403/08/09"
		mockService.getUserProfileFunc = func(ctx context.Context, userID uint64, viewerUserID *uint64) (*service.UserProfileData, error) {
			return &service.UserProfileData{
				ID:             1,
				Name:           &name,
				Code:           "hm-1234567",
				RegisteredAt:   &registeredAt,
				ProfileImages:  []string{"https://example.com/image1.jpg"},
				FollowersCount: func() *int32 { c := int32(10); return &c }(),
				FollowingCount: func() *int32 { c := int32(5); return &c }(),
			}, nil
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.GetUserProfileRequest{
			UserId:       1,
			ViewerUserId: 2,
		}

		resp, err := handler.GetUserProfile(ctx, req)
		if err != nil {
			t.Fatalf("GetUserProfile failed: %v", err)
		}

		if resp.Data == nil {
			t.Fatal("Expected profile data")
		}

		if resp.Data.Id != 1 {
			t.Errorf("Expected user ID 1, got %d", resp.Data.Id)
		}

		if resp.Data.Name != "Test User" {
			t.Errorf("Expected name 'Test User', got '%s'", resp.Data.Name)
		}

		if len(resp.Data.ProfileImages) != 1 {
			t.Errorf("Expected 1 profile image, got %d", len(resp.Data.ProfileImages))
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.getUserProfileFunc = func(ctx context.Context, userID uint64, viewerUserID *uint64) (*service.UserProfileData, error) {
			return nil, errors.New("user not found")
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.GetUserProfileRequest{
			UserId: 999,
		}

		_, err := handler.GetUserProfile(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})
}

func TestUserHandler_GetUserFeaturesCount(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get user features count", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.getUserFeaturesCountFunc = func(ctx context.Context, userID uint64) (*service.UserFeaturesCountData, error) {
			return &service.UserFeaturesCountData{
				MaskoniFeaturesCount:   5,
				TejariFeaturesCount:    2,
				AmoozeshiFeaturesCount: 0,
			}, nil
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.GetUserFeaturesCountRequest{
			UserId: 1,
		}

		resp, err := handler.GetUserFeaturesCount(ctx, req)
		if err != nil {
			t.Fatalf("GetUserFeaturesCount failed: %v", err)
		}

		if resp.Data == nil {
			t.Fatal("Expected features count data")
		}

		if resp.Data.MaskoniFeaturesCount != 5 {
			t.Errorf("Expected maskoni count 5, got %d", resp.Data.MaskoniFeaturesCount)
		}

		if resp.Data.TejariFeaturesCount != 2 {
			t.Errorf("Expected tejari count 2, got %d", resp.Data.TejariFeaturesCount)
		}

		if resp.Data.AmoozeshiFeaturesCount != 0 {
			t.Errorf("Expected amoozeshi count 0, got %d", resp.Data.AmoozeshiFeaturesCount)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		mockService := &mockUserService{}
		mockService.getUserFeaturesCountFunc = func(ctx context.Context, userID uint64) (*service.UserFeaturesCountData, error) {
			return nil, errors.New("user not found")
		}

		handler := &userHandler{
			userService: mockService,
		}

		req := &pb.GetUserFeaturesCountRequest{
			UserId: 999,
		}

		_, err := handler.GetUserFeaturesCount(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Expected gRPC status error")
		}
		if st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound error code, got %v", st.Code())
		}
	})
}
