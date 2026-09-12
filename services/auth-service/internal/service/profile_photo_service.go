package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/repository"
)

var (
	ErrProfilePhotoNotFound = errors.New("profile photo not found")
	ErrPhotoUnauthorized    = errors.New("unauthorized: profile photo does not belong to user")
	ErrInvalidImage         = errors.New("invalid image: must be PNG or JPEG, ≤2 MB")
	ErrImageRequired        = errors.New("image is required")
)

const (
	profilePhotoMaxSize    = 2 * 1024 * 1024
	profilePhotoUploadPath = "/uploads/profile"
)

type ProfilePhotoService interface {
	ListProfilePhotos(ctx context.Context, userID uint64) ([]*models.Image, error)
	UploadProfilePhoto(ctx context.Context, userID uint64, imageData []byte, filename, contentType string) (*models.Image, error)
	GetProfilePhoto(ctx context.Context, id uint64) (*models.Image, error)
	DeleteProfilePhoto(ctx context.Context, userID uint64, id uint64) error
	ResolvePhotoURL(url string) string
}

type profilePhotoService struct {
	repo          repository.ProfilePhotoRepository
	fileStorage   FileStorage
	apiGatewayURL string
}

func NewProfilePhotoService(repo repository.ProfilePhotoRepository, fileStorage FileStorage, apiGatewayURL string) ProfilePhotoService {
	return &profilePhotoService{
		repo:          repo,
		fileStorage:   fileStorage,
		apiGatewayURL: apiGatewayURL,
	}
}

func (s *profilePhotoService) ResolvePhotoURL(url string) string {
	return ResolvePublicURL(s.apiGatewayURL, url)
}

func (s *profilePhotoService) ListProfilePhotos(ctx context.Context, userID uint64) ([]*models.Image, error) {
	photos, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list profile photos: %w", err)
	}
	return photos, nil
}

func (s *profilePhotoService) UploadProfilePhoto(ctx context.Context, userID uint64, imageData []byte, filename, contentType string) (*models.Image, error) {
	normalizedType, err := validateProfilePhotoFile(imageData, filename, contentType)
	if err != nil {
		return nil, err
	}
	if s.fileStorage == nil {
		return nil, ErrStorageUnavailable
	}

	relativePath, err := s.fileStorage.UploadChunk(
		ctx,
		NewUploadID("profile_photo", userID),
		profilePhotoUploadPath,
		filename,
		normalizedType,
		imageData,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upload profile photo: %w", err)
	}

	fullURL := PrependGatewayURL(s.apiGatewayURL, relativePath)
	image, err := s.repo.Create(ctx, userID, fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create profile photo record: %w", err)
	}
	return image, nil
}

func validateProfilePhotoFile(imageData []byte, filename, contentType string) (string, error) {
	if len(imageData) == 0 {
		return "", ErrImageRequired
	}
	if len(imageData) > profilePhotoMaxSize {
		return "", ErrInvalidImage
	}
	if strings.TrimSpace(filename) == "" {
		return "", ErrInvalidImage
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return "", ErrInvalidImage
	}

	normalizedType := normalizeProfilePhotoContentType(contentType, filename, imageData)
	if normalizedType != "image/png" && normalizedType != "image/jpeg" {
		return "", ErrInvalidImage
	}
	return normalizedType, nil
}

// normalizeProfilePhotoContentType accepts image/png, image/jpeg, and image/jpg.
// When multipart clients omit Content-Type or send application/octet-stream,
// the type is inferred from the filename extension and/or file magic bytes.
func normalizeProfilePhotoContentType(contentType, filename string, data []byte) string {
	contentType = strings.TrimSpace(strings.ToLower(contentType))
	if contentType != "" {
		if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
			contentType = mediaType
		}
	}
	if contentType == "image/jpg" {
		contentType = "image/jpeg"
	}
	if contentType == "image/png" || contentType == "image/jpeg" {
		return contentType
	}

	if contentType == "" || contentType == "application/octet-stream" {
		switch strings.ToLower(filepath.Ext(filename)) {
		case ".png":
			return "image/png"
		case ".jpg", ".jpeg":
			return "image/jpeg"
		}
	}

	detected := http.DetectContentType(data)
	if mediaType, _, err := mime.ParseMediaType(detected); err == nil {
		detected = mediaType
	}
	if detected == "image/png" || detected == "image/jpeg" {
		return detected
	}
	return contentType
}

func (s *profilePhotoService) GetProfilePhoto(ctx context.Context, id uint64) (*models.Image, error) {
	photo, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile photo: %w", err)
	}
	if photo == nil {
		return nil, ErrProfilePhotoNotFound
	}
	return photo, nil
}

func (s *profilePhotoService) DeleteProfilePhoto(ctx context.Context, userID uint64, id uint64) error {
	owns, err := s.repo.CheckOwnership(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("failed to check ownership: %w", err)
	}
	if !owns {
		return ErrPhotoUnauthorized
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete profile photo: %w", err)
	}
	return nil
}
