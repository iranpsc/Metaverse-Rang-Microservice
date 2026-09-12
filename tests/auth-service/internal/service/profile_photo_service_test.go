package service_test

import (
	"context"
	"errors"
	"fmt"
	"metarang/auth-service/internal/service"
	"testing"
	"time"

	"metarang/auth-service/internal/models"
)

type fakeProfilePhotoRepository struct {
	photos       map[uint64]*models.Image
	userPhotos   map[uint64][]*models.Image
	createCount  int
	deleteCount  int
	findByIDFunc func(ctx context.Context, id uint64) (*models.Image, error)
	checkOwnFunc func(ctx context.Context, id uint64, userID uint64) (bool, error)
}

func newFakeProfilePhotoRepository() *fakeProfilePhotoRepository {
	return &fakeProfilePhotoRepository{
		photos:     make(map[uint64]*models.Image),
		userPhotos: make(map[uint64][]*models.Image),
	}
}

func (r *fakeProfilePhotoRepository) Create(ctx context.Context, userID uint64, url string) (*models.Image, error) {
	r.createCount++
	id := uint64(len(r.photos) + 1)
	image := &models.Image{
		ID:            id,
		ImageableType: "App\\Models\\User",
		ImageableID:   userID,
		URL:           url,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	r.photos[id] = image
	r.userPhotos[userID] = append(r.userPhotos[userID], image)
	return image, nil
}

func (r *fakeProfilePhotoRepository) FindByUserID(ctx context.Context, userID uint64) ([]*models.Image, error) {
	return r.userPhotos[userID], nil
}

func (r *fakeProfilePhotoRepository) FindByID(ctx context.Context, id uint64) (*models.Image, error) {
	if r.findByIDFunc != nil {
		return r.findByIDFunc(ctx, id)
	}
	return r.photos[id], nil
}

func (r *fakeProfilePhotoRepository) Delete(ctx context.Context, id uint64) error {
	r.deleteCount++
	if _, exists := r.photos[id]; !exists {
		return errors.New("profile photo not found")
	}
	delete(r.photos, id)
	for userID, photos := range r.userPhotos {
		for i, photo := range photos {
			if photo.ID == id {
				r.userPhotos[userID] = append(photos[:i], photos[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (r *fakeProfilePhotoRepository) CheckOwnership(ctx context.Context, id uint64, userID uint64) (bool, error) {
	if r.checkOwnFunc != nil {
		return r.checkOwnFunc(ctx, id, userID)
	}
	photo, exists := r.photos[id]
	if !exists {
		return false, nil
	}
	return photo.ImageableID == userID, nil
}

type fakeFileStorage struct {
	uploadCount int
}

func (f *fakeFileStorage) UploadChunk(ctx context.Context, uploadID, uploadPath, filename, contentType string, data []byte) (string, error) {
	f.uploadCount++
	return fmt.Sprintf("/uploads/profile/%s", filename), nil
}

func (f *fakeFileStorage) ReadFile(ctx context.Context, filePath string) ([]byte, string, error) {
	return nil, "", errors.New("not implemented")
}

func TestProfilePhotoService_ListProfilePhotos(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProfilePhotoRepository()
	storageClient := &fakeFileStorage{}
	svc := service.NewProfilePhotoService(repo, storageClient, "http://localhost:8080")

	t.Run("successful list", func(t *testing.T) {
		userID := uint64(1)
		_, _ = repo.Create(ctx, userID, "https://example.com/photo1.jpg")
		_, _ = repo.Create(ctx, userID, "https://example.com/photo2.jpg")

		photos, err := svc.ListProfilePhotos(ctx, userID)
		if err != nil {
			t.Fatalf("ListProfilePhotos failed: %v", err)
		}
		if len(photos) != 2 {
			t.Errorf("Expected 2 photos, got %d", len(photos))
		}
	})

	t.Run("empty list for user with no photos", func(t *testing.T) {
		photos, err := svc.ListProfilePhotos(ctx, 999)
		if err != nil {
			t.Fatalf("ListProfilePhotos failed: %v", err)
		}
		if len(photos) != 0 {
			t.Errorf("Expected 0 photos, got %d", len(photos))
		}
	})
}

func TestProfilePhotoService_UploadProfilePhoto(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProfilePhotoRepository()
	storageClient := &fakeFileStorage{}
	svc := service.NewProfilePhotoService(repo, storageClient, "http://localhost:8080")

	t.Run("successful upload", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "test.jpg", "image/jpeg")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
		if photo.URL == "" {
			t.Error("Expected photo URL to be set")
		}
		if storageClient.uploadCount != 1 {
			t.Errorf("Expected 1 upload, got %d", storageClient.uploadCount)
		}
	})

	t.Run("empty image data", func(t *testing.T) {
		_, err := svc.UploadProfilePhoto(ctx, 1, []byte{}, "test.jpg", "image/jpeg")
		if err != service.ErrImageRequired {
			t.Errorf("Expected service.ErrImageRequired, got %v", err)
		}
	})

	t.Run("file at max size", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 2*1024*1024), "test.jpg", "image/jpeg")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
	})

	t.Run("file too large", func(t *testing.T) {
		_, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 2*1024*1024+1), "test.jpg", "image/jpeg")
		if err != service.ErrInvalidImage {
			t.Errorf("Expected service.ErrInvalidImage, got %v", err)
		}
	})

	t.Run("invalid content type", func(t *testing.T) {
		_, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "test.gif", "image/gif")
		if err != service.ErrInvalidImage {
			t.Errorf("Expected service.ErrInvalidImage, got %v", err)
		}
	})

	t.Run("invalid file extension", func(t *testing.T) {
		_, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "test.gif", "image/jpeg")
		if err != service.ErrInvalidImage {
			t.Errorf("Expected service.ErrInvalidImage, got %v", err)
		}
	})

	t.Run("PNG file upload", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "test.png", "image/png")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
	})

	t.Run("PNG with empty content type", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "avatar.png", "")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
	})

	t.Run("JPEG with octet-stream content type", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "avatar.jpeg", "application/octet-stream")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
	})

	t.Run("PNG with content type parameters", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "avatar.png", "image/png; charset=binary")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
	})

	t.Run("JPG mime type alias", func(t *testing.T) {
		photo, err := svc.UploadProfilePhoto(ctx, 1, make([]byte, 100), "avatar.jpg", "image/jpg")
		if err != nil {
			t.Fatalf("UploadProfilePhoto failed: %v", err)
		}
		if photo.ID == 0 {
			t.Error("Expected photo ID to be set")
		}
	})
}

func TestProfilePhotoService_GetProfilePhoto(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProfilePhotoRepository()
	svc := service.NewProfilePhotoService(repo, &fakeFileStorage{}, "http://localhost:8080")

	t.Run("successful get", func(t *testing.T) {
		photo, _ := repo.Create(ctx, 1, "https://example.com/photo.jpg")
		result, err := svc.GetProfilePhoto(ctx, photo.ID)
		if err != nil {
			t.Fatalf("GetProfilePhoto failed: %v", err)
		}
		if result.ID != photo.ID {
			t.Errorf("Expected ID %d, got %d", photo.ID, result.ID)
		}
	})

	t.Run("photo not found", func(t *testing.T) {
		_, err := svc.GetProfilePhoto(ctx, 999)
		if err != service.ErrProfilePhotoNotFound {
			t.Errorf("Expected service.ErrProfilePhotoNotFound, got %v", err)
		}
	})

	t.Run("repository returns nil", func(t *testing.T) {
		repo.findByIDFunc = func(ctx context.Context, id uint64) (*models.Image, error) {
			return nil, nil
		}
		_, err := svc.GetProfilePhoto(ctx, 1)
		if err != service.ErrProfilePhotoNotFound {
			t.Errorf("Expected service.ErrProfilePhotoNotFound, got %v", err)
		}
	})
}

func TestProfilePhotoService_DeleteProfilePhoto(t *testing.T) {
	ctx := context.Background()
	repo := newFakeProfilePhotoRepository()
	svc := service.NewProfilePhotoService(repo, &fakeFileStorage{}, "http://localhost:8080")

	t.Run("successful delete", func(t *testing.T) {
		photo, _ := repo.Create(ctx, 1, "https://example.com/photo.jpg")
		if err := svc.DeleteProfilePhoto(ctx, 1, photo.ID); err != nil {
			t.Fatalf("DeleteProfilePhoto failed: %v", err)
		}
		if result, _ := repo.FindByID(ctx, photo.ID); result != nil {
			t.Error("Expected photo to be deleted")
		}
	})

	t.Run("unauthorized - photo belongs to different user", func(t *testing.T) {
		photo, _ := repo.Create(ctx, 1, "https://example.com/photo.jpg")
		err := svc.DeleteProfilePhoto(ctx, 2, photo.ID)
		if err != service.ErrPhotoUnauthorized {
			t.Errorf("Expected service.ErrPhotoUnauthorized, got %v", err)
		}
	})

	t.Run("photo not found", func(t *testing.T) {
		repo.checkOwnFunc = func(ctx context.Context, id uint64, userID uint64) (bool, error) {
			return false, nil
		}
		err := svc.DeleteProfilePhoto(ctx, 1, 999)
		if err != service.ErrPhotoUnauthorized {
			t.Errorf("Expected service.ErrPhotoUnauthorized, got %v", err)
		}
	})
}
