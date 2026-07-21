package handler_test

import (
	"context"
	"errors"
	"metarang/auth-service/internal/handler"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
	storagepb "metarang/shared/pb/storage"
)

// mockProfilePhotoService is a mock implementation for testing
type mockProfilePhotoService struct {
	listPhotosFunc   func(ctx context.Context, userID uint64) ([]*models.Image, error)
	uploadPhotoFunc  func(ctx context.Context, userID uint64, imageData []byte, filename, contentType string) (*models.Image, error)
	createRecordFunc func(ctx context.Context, userID uint64, url string) (*models.Image, error)
	getPhotoFunc     func(ctx context.Context, id uint64) (*models.Image, error)
	deletePhotoFunc  func(ctx context.Context, userID uint64, id uint64) error
}

func (m *mockProfilePhotoService) ListProfilePhotos(ctx context.Context, userID uint64) ([]*models.Image, error) {
	if m.listPhotosFunc != nil {
		return m.listPhotosFunc(ctx, userID)
	}
	return nil, nil
}

func (m *mockProfilePhotoService) UploadProfilePhoto(ctx context.Context, userID uint64, imageData []byte, filename, contentType string) (*models.Image, error) {
	if m.uploadPhotoFunc != nil {
		return m.uploadPhotoFunc(ctx, userID, imageData, filename, contentType)
	}
	return nil, nil
}

func (m *mockProfilePhotoService) GetProfilePhoto(ctx context.Context, id uint64) (*models.Image, error) {
	if m.getPhotoFunc != nil {
		return m.getPhotoFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockProfilePhotoService) CreateProfilePhotoRecord(ctx context.Context, userID uint64, url string) (*models.Image, error) {
	if m.createRecordFunc != nil {
		return m.createRecordFunc(ctx, userID, url)
	}
	return &models.Image{ImageableID: userID, URL: url}, nil
}

type mockStorageClient struct {
	storagepb.FileStorageServiceClient
}

func (m *mockStorageClient) ChunkUpload(context.Context, *storagepb.ChunkUploadRequest, ...grpc.CallOption) (*storagepb.ChunkUploadResponse, error) {
	return &storagepb.ChunkUploadResponse{
		Success:       true,
		IsFinished:    true,
		FileUrl:       "/uploads/profile",
		FilePath:      "test.jpg",
		FinalFilename: "test.jpg",
	}, nil
}

func (m *mockProfilePhotoService) DeleteProfilePhoto(ctx context.Context, userID uint64, id uint64) error {
	if m.deletePhotoFunc != nil {
		return m.deletePhotoFunc(ctx, userID, id)
	}
	return nil
}

func TestProfilePhotoHandler_ListProfilePhotos(t *testing.T) {
	ctx := context.Background()

	t.Run("successful list with full URLs - database records from auth-service, files from storage-service", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		// Simulate database records stored by auth-service with relative URLs from storage-service
		mockService.listPhotosFunc = func(ctx context.Context, userID uint64) ([]*models.Image, error) {
			return []*models.Image{
				{ID: 1, URL: "/uploads/profile/photo1.jpg"},            // Relative URL from storage-service
				{ID: 2, URL: "/uploads/profile/photo2.jpg"},            // Relative URL from storage-service
				{ID: 3, URL: "https://storage.example.com/photo3.jpg"}, // Already full URL
			}, nil
		}

		// API gateway URL for prepending
		apiGatewayURL := "https://api.example.com"
		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			APIGatewayURL:       apiGatewayURL,
		}

		req := &pb.ListProfilePhotosRequest{UserId: 1}
		resp, err := h.ListProfilePhotos(ctx, req)
		if err != nil {
			t.Fatalf("ListProfilePhotos failed: %v", err)
		}

		// Verify response structure: data array with id and full URL
		if len(resp.Data) != 3 {
			t.Errorf("Expected 3 photos, got %d", len(resp.Data))
		}

		// Verify first photo: relative URL should be prepended with gateway URL
		if resp.Data[0].Id != 1 {
			t.Errorf("Expected first photo ID 1, got %d", resp.Data[0].Id)
		}
		expectedURL1 := "https://api.example.com/uploads/profile/photo1.jpg"
		if resp.Data[0].Url != expectedURL1 {
			t.Errorf("Expected first photo URL %s, got %s", expectedURL1, resp.Data[0].Url)
		}

		// Verify second photo: relative URL should be prepended
		if resp.Data[1].Id != 2 {
			t.Errorf("Expected second photo ID 2, got %d", resp.Data[1].Id)
		}
		expectedURL2 := "https://api.example.com/uploads/profile/photo2.jpg"
		if resp.Data[1].Url != expectedURL2 {
			t.Errorf("Expected second photo URL %s, got %s", expectedURL2, resp.Data[1].Url)
		}

		// Verify third photo: already full URL should remain unchanged
		if resp.Data[2].Id != 3 {
			t.Errorf("Expected third photo ID 3, got %d", resp.Data[2].Id)
		}
		expectedURL3 := "https://storage.example.com/photo3.jpg"
		if resp.Data[2].Url != expectedURL3 {
			t.Errorf("Expected third photo URL %s, got %s", expectedURL3, resp.Data[2].Url)
		}

		// Verify response format: each item has id and url (full image URL)
		for i, photo := range resp.Data {
			if photo.Id == 0 {
				t.Errorf("Photo at index %d: id is required and must not be zero", i)
			}
			if photo.Url == "" {
				t.Errorf("Photo at index %d: url (full image URL) is required and must not be empty", i)
			}
			// Verify URL is a full URL (starts with http:// or https://)
			if !(strings.HasPrefix(photo.Url, "http://") || strings.HasPrefix(photo.Url, "https://")) {
				t.Errorf("Photo at index %d: url must be a full URL (starts with http:// or https://), got %s", i, photo.Url)
			}
		}
	})

	t.Run("successful list with gateway URL having trailing slash", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.listPhotosFunc = func(ctx context.Context, userID uint64) ([]*models.Image, error) {
			return []*models.Image{
				{ID: 1, URL: "/uploads/profile/photo1.jpg"},
			}, nil
		}

		// Gateway URL with trailing slash should be handled correctly
		apiGatewayURL := "https://api.example.com/"
		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			APIGatewayURL:       apiGatewayURL,
		}

		req := &pb.ListProfilePhotosRequest{UserId: 1}
		resp, err := h.ListProfilePhotos(ctx, req)
		if err != nil {
			t.Fatalf("ListProfilePhotos failed: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Fatalf("Expected 1 photo, got %d", len(resp.Data))
		}

		expectedURL := "https://api.example.com/uploads/profile/photo1.jpg"
		if resp.Data[0].Url != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, resp.Data[0].Url)
		}
	})

	t.Run("successful list with empty gateway URL", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.listPhotosFunc = func(ctx context.Context, userID uint64) ([]*models.Image, error) {
			return []*models.Image{
				{ID: 1, URL: "/uploads/profile/photo1.jpg"},
			}, nil
		}

		// Empty gateway URL should return original URL
		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			APIGatewayURL:       "",
		}

		req := &pb.ListProfilePhotosRequest{UserId: 1}
		resp, err := h.ListProfilePhotos(ctx, req)
		if err != nil {
			t.Fatalf("ListProfilePhotos failed: %v", err)
		}

		if len(resp.Data) != 1 {
			t.Fatalf("Expected 1 photo, got %d", len(resp.Data))
		}

		// Should return original URL when gateway URL is empty
		expectedURL := "/uploads/profile/photo1.jpg"
		if resp.Data[0].Url != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, resp.Data[0].Url)
		}
	})

	t.Run("successful list with empty result", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.listPhotosFunc = func(ctx context.Context, userID uint64) ([]*models.Image, error) {
			return []*models.Image{}, nil
		}

		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			APIGatewayURL:       "https://api.example.com",
		}

		req := &pb.ListProfilePhotosRequest{UserId: 1}
		resp, err := h.ListProfilePhotos(ctx, req)
		if err != nil {
			t.Fatalf("ListProfilePhotos failed: %v", err)
		}

		if resp.Data == nil {
			t.Error("Expected data array to be initialized, got nil")
		}
		if len(resp.Data) != 0 {
			t.Errorf("Expected 0 photos, got %d", len(resp.Data))
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.ListProfilePhotosRequest{UserId: 0}
		_, err := h.ListProfilePhotos(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing user_id")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.listPhotosFunc = func(ctx context.Context, userID uint64) ([]*models.Image, error) {
			return nil, errors.New("database error")
		}

		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.ListProfilePhotosRequest{UserId: 1}
		_, err := h.ListProfilePhotos(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Internal {
			t.Errorf("Expected Internal error, got %v", err)
		}
	})
}

func TestProfilePhotoHandler_UploadProfilePhoto(t *testing.T) {
	ctx := context.Background()

	t.Run("successful upload - database record stored by auth-service, file uploaded by storage-service", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		// Simulate: storage-service uploads file and returns relative URL
		// auth-service stores database record with that URL
		mockService.uploadPhotoFunc = func(ctx context.Context, userID uint64, imageData []byte, filename, contentType string) (*models.Image, error) {
			return &models.Image{
				ID:  1,
				URL: "/uploads/profile/test.jpg", // Relative URL from storage-service
			}, nil
		}
		mockService.createRecordFunc = func(context.Context, uint64, string) (*models.Image, error) {
			return &models.Image{ID: 1, URL: "/uploads/profile/test.jpg"}, nil
		}

		apiGatewayURL := "https://api.example.com"
		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			StorageClient:       &mockStorageClient{},
			APIGatewayURL:       apiGatewayURL,
		}

		req := &pb.UploadProfilePhotoRequest{
			UserId:      1,
			ImageData:   []byte{1, 2, 3},
			Filename:    "test.jpg",
			ContentType: "image/jpeg",
		}

		resp, err := h.UploadProfilePhoto(ctx, req)
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}

		// Verify response contains id
		if resp.Id != 1 {
			t.Errorf("Expected ID 1, got %d", resp.Id)
		}

		// Verify response contains full image URL (prepended with gateway URL)
		if resp.Url == "" {
			t.Error("Expected URL (full image URL) to be set")
		}
		expectedURL := "https://api.example.com/uploads/profile/test.jpg"
		if resp.Url != expectedURL {
			t.Errorf("Expected full URL %s, got %s", expectedURL, resp.Url)
		}

		// Verify URL is a full URL
		if !strings.HasPrefix(resp.Url, "http://") && !strings.HasPrefix(resp.Url, "https://") {
			t.Errorf("Expected full URL (starts with http:// or https://), got %s", resp.Url)
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.UploadProfilePhotoRequest{
			ImageData:   []byte{1, 2, 3},
			Filename:    "test.jpg",
			ContentType: "image/jpeg",
		}

		_, err := h.UploadProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing user_id")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", err)
		}
	})

	t.Run("missing image_data", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.UploadProfilePhotoRequest{
			UserId:      1,
			Filename:    "test.jpg",
			ContentType: "image/jpeg",
		}

		_, err := h.UploadProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing image_data")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", err)
		}
	})

	t.Run("invalid image error", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.createRecordFunc = func(ctx context.Context, userID uint64, url string) (*models.Image, error) {
			return nil, service.ErrInvalidImage
		}

		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			StorageClient:       &mockStorageClient{},
		}

		req := &pb.UploadProfilePhotoRequest{
			UserId:      1,
			ImageData:   []byte{1, 2, 3},
			Filename:    "test.jpg",
			ContentType: "image/jpeg",
		}

		_, err := h.UploadProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Internal {
			t.Errorf("Expected Internal, got %v", err)
		}
	})
}

func TestProfilePhotoHandler_GetProfilePhoto(t *testing.T) {
	ctx := context.Background()

	t.Run("successful get with full URL", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.getPhotoFunc = func(ctx context.Context, id uint64) (*models.Image, error) {
			return &models.Image{
				ID:  id,
				URL: "/uploads/profile/photo.jpg", // Relative URL from storage-service
			}, nil
		}

		apiGatewayURL := "https://api.example.com"
		h := &handler.ProfilePhotoHandler{
			ProfilePhotoService: mockService,
			APIGatewayURL:       apiGatewayURL,
		}

		req := &pb.GetProfilePhotoRequest{ProfilePhotoId: 1}
		resp, err := h.GetProfilePhoto(ctx, req)
		if err != nil {
			t.Fatalf("GetProfilePhoto failed: %v", err)
		}

		// Verify response contains id
		if resp.Id != 1 {
			t.Errorf("Expected ID 1, got %d", resp.Id)
		}

		// Verify response contains full image URL
		expectedURL := "https://api.example.com/uploads/profile/photo.jpg"
		if resp.Url != expectedURL {
			t.Errorf("Expected full URL %s, got %s", expectedURL, resp.Url)
		}

		// Verify URL is a full URL
		if !strings.HasPrefix(resp.Url, "http://") && !strings.HasPrefix(resp.Url, "https://") {
			t.Errorf("Expected full URL (starts with http:// or https://), got %s", resp.Url)
		}
	})

	t.Run("missing profile_photo_id", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.GetProfilePhotoRequest{ProfilePhotoId: 0}
		_, err := h.GetProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing profile_photo_id")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", err)
		}
	})

	t.Run("photo not found", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.getPhotoFunc = func(ctx context.Context, id uint64) (*models.Image, error) {
			return nil, service.ErrProfilePhotoNotFound
		}

		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.GetProfilePhotoRequest{ProfilePhotoId: 999}
		_, err := h.GetProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound, got %v", err)
		}
	})
}

func TestProfilePhotoHandler_DeleteProfilePhoto(t *testing.T) {
	ctx := context.Background()

	t.Run("successful delete", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.deletePhotoFunc = func(ctx context.Context, userID uint64, id uint64) error {
			return nil
		}

		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.DeleteProfilePhotoRequest{
			UserId:         1,
			ProfilePhotoId: 1,
		}

		_, err := h.DeleteProfilePhoto(ctx, req)
		if err != nil {
			t.Fatalf("DeleteProfilePhoto failed: %v", err)
		}
	})

	t.Run("missing user_id", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.DeleteProfilePhotoRequest{
			ProfilePhotoId: 1,
		}

		_, err := h.DeleteProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing user_id")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", err)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.deletePhotoFunc = func(ctx context.Context, userID uint64, id uint64) error {
			return service.ErrUnauthorized
		}

		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.DeleteProfilePhotoRequest{
			UserId:         1,
			ProfilePhotoId: 999,
		}

		_, err := h.DeleteProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.PermissionDenied {
			t.Errorf("Expected PermissionDenied, got %v", err)
		}
	})

	t.Run("photo not found", func(t *testing.T) {
		mockService := &mockProfilePhotoService{}
		mockService.deletePhotoFunc = func(ctx context.Context, userID uint64, id uint64) error {
			return service.ErrProfilePhotoNotFound
		}

		h := &handler.ProfilePhotoHandler{ProfilePhotoService: mockService}

		req := &pb.DeleteProfilePhotoRequest{
			UserId:         1,
			ProfilePhotoId: 999,
		}

		_, err := h.DeleteProfilePhoto(ctx, req)
		if err == nil {
			t.Fatal("Expected error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected NotFound, got %v", err)
		}
	})
}

// TestPrependGatewayURL tests the prependGatewayURL function with various edge cases
func TestPrependGatewayURL(t *testing.T) {
	tests := []struct {
		name        string
		gatewayURL  string
		inputURL    string
		expectedURL string
		description string
	}{
		{
			name:        "relative URL with gateway URL",
			gatewayURL:  "https://api.example.com",
			inputURL:    "/uploads/profile/photo.jpg",
			expectedURL: "https://api.example.com/uploads/profile/photo.jpg",
			description: "Relative URL should be prepended with gateway URL",
		},
		{
			name:        "relative URL without leading slash",
			gatewayURL:  "https://api.example.com",
			inputURL:    "uploads/profile/photo.jpg",
			expectedURL: "https://api.example.com/uploads/profile/photo.jpg",
			description: "Relative URL without leading slash should be prepended",
		},
		{
			name:        "already full HTTP URL",
			gatewayURL:  "https://api.example.com",
			inputURL:    "http://storage.example.com/photo.jpg",
			expectedURL: "http://storage.example.com/photo.jpg",
			description: "Full HTTP URL should remain unchanged",
		},
		{
			name:        "already full HTTPS URL",
			gatewayURL:  "https://api.example.com",
			inputURL:    "https://storage.example.com/photo.jpg",
			expectedURL: "https://storage.example.com/photo.jpg",
			description: "Full HTTPS URL should remain unchanged",
		},
		{
			name:        "empty gateway URL",
			gatewayURL:  "",
			inputURL:    "/uploads/profile/photo.jpg",
			expectedURL: "/uploads/profile/photo.jpg",
			description: "Empty gateway URL should return original URL",
		},
		{
			name:        "gateway URL with trailing slash",
			gatewayURL:  "https://api.example.com/",
			inputURL:    "/uploads/profile/photo.jpg",
			expectedURL: "https://api.example.com/uploads/profile/photo.jpg",
			description: "Trailing slash in gateway URL should be handled correctly",
		},
		{
			name:        "empty input URL",
			gatewayURL:  "https://api.example.com",
			inputURL:    "",
			expectedURL: "",
			description: "Empty input URL should return empty string",
		},
		{
			name:        "relative URL with gateway URL having trailing slash",
			gatewayURL:  "https://api.example.com/",
			inputURL:    "uploads/profile/photo.jpg",
			expectedURL: "https://api.example.com/uploads/profile/photo.jpg",
			description: "Both gateway and input URL edge cases should be handled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &handler.ProfilePhotoHandler{
				APIGatewayURL: tt.gatewayURL,
			}

			result := h.PrependGatewayURL(tt.inputURL)
			if result != tt.expectedURL {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expectedURL, result)
			}
		})
	}
}
