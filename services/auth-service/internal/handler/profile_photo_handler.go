package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metarang/auth-service/internal/lang"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

// ProfilePhotoHandler handles profile photo gRPC requests.
type ProfilePhotoHandler struct {
	pb.UnimplementedProfilePhotoServiceServer
	ProfilePhotoService service.ProfilePhotoService
}

func NewProfilePhotoHandler(profilePhotoService service.ProfilePhotoService) *ProfilePhotoHandler {
	return &ProfilePhotoHandler{ProfilePhotoService: profilePhotoService}
}

func RegisterProfilePhotoHandler(grpcServer *grpc.Server, profilePhotoService service.ProfilePhotoService) {
	pb.RegisterProfilePhotoServiceServer(grpcServer, NewProfilePhotoHandler(profilePhotoService))
}

func (h *ProfilePhotoHandler) ListProfilePhotos(ctx context.Context, req *pb.ListProfilePhotosRequest) (*pb.ListProfilePhotosResponse, error) {
	locale := getProjectLocale()
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	photos, err := h.ProfilePhotoService.ListProfilePhotos(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", lang.Tf(locale, "failed to list profile photos: %v", err))
	}

	response := &pb.ListProfilePhotosResponse{
		Data: make([]*pb.ProfilePhoto, 0, len(photos)),
	}
	for _, photo := range photos {
		response.Data = append(response.Data, &pb.ProfilePhoto{
			Id:  photo.ID,
			Url: h.ProfilePhotoService.ResolvePhotoURL(photo.URL),
		})
	}
	return response, nil
}

func (h *ProfilePhotoHandler) UploadProfilePhoto(ctx context.Context, req *pb.UploadProfilePhotoRequest) (*pb.ProfilePhotoResponse, error) {
	locale := getProjectLocale()
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	photo, err := h.ProfilePhotoService.UploadProfilePhoto(ctx, userID, req.ImageData, req.Filename, req.ContentType)
	if err != nil {
		return nil, mapProfilePhotoServiceError(err, locale)
	}

	return &pb.ProfilePhotoResponse{
		Id:  photo.ID,
		Url: h.ProfilePhotoService.ResolvePhotoURL(photo.URL),
	}, nil
}

func (h *ProfilePhotoHandler) GetProfilePhoto(ctx context.Context, req *pb.GetProfilePhotoRequest) (*pb.ProfilePhotoResponse, error) {
	locale := getProjectLocale()
	if req.ProfilePhotoId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "profile_photo_id is required"))
	}

	photo, err := h.ProfilePhotoService.GetProfilePhoto(ctx, req.ProfilePhotoId)
	if err != nil {
		return nil, mapProfilePhotoServiceError(err, locale)
	}

	return &pb.ProfilePhotoResponse{
		Id:  photo.ID,
		Url: h.ProfilePhotoService.ResolvePhotoURL(photo.URL),
	}, nil
}

func (h *ProfilePhotoHandler) DeleteProfilePhoto(ctx context.Context, req *pb.DeleteProfilePhotoRequest) (*emptypb.Empty, error) {
	locale := getProjectLocale()
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}

	if req.ProfilePhotoId == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "profile_photo_id is required"))
	}

	if err := h.ProfilePhotoService.DeleteProfilePhoto(ctx, userID, req.ProfilePhotoId); err != nil {
		return nil, mapProfilePhotoServiceError(err, locale)
	}
	return &emptypb.Empty{}, nil
}

func mapProfilePhotoServiceError(err error, locale string) error {
	switch {
	case errors.Is(err, service.ErrImageRequired):
		return status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "image_data is required"))
	case errors.Is(err, service.ErrInvalidImage):
		return status.Errorf(codes.InvalidArgument, "%s", lang.T(locale, "invalid image: must be PNG or JPEG, ≤2 MB"))
	case errors.Is(err, service.ErrStorageUnavailable):
		return status.Errorf(codes.Internal, "%s", lang.T(locale, "storage service not available"))
	case errors.Is(err, service.ErrProfilePhotoNotFound):
		return status.Errorf(codes.NotFound, "%s", lang.T(locale, "profile photo not found"))
	case errors.Is(err, service.ErrPhotoUnauthorized):
		return status.Errorf(codes.PermissionDenied, "%s", lang.T(locale, "unauthorized: profile photo does not belong to user"))
	default:
		return status.Errorf(codes.Internal, "%s", lang.Tf(locale, "operation failed: %v", err))
	}
}
