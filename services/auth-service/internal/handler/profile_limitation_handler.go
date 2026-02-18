package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"metargb/auth-service/internal/lang"
	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
)

type profileLimitationHandler struct {
	pb.UnimplementedProfileLimitationServiceServer
	limitationService service.ProfileLimitationService
}

func RegisterProfileLimitationHandler(grpcServer *grpc.Server, limitationService service.ProfileLimitationService) {
	pb.RegisterProfileLimitationServiceServer(grpcServer, &profileLimitationHandler{
		limitationService: limitationService,
	})
}

func (h *profileLimitationHandler) CreateProfileLimitation(ctx context.Context, req *pb.CreateProfileLimitationRequest) (*pb.ProfileLimitationResponse, error) {
	// Convert proto options to model options
	options := models.ProfileLimitationOptions{
		Follow:                req.Options.Follow,
		SendMessage:           req.Options.SendMessage,
		Share:                 req.Options.Share,
		SendTicket:            req.Options.SendTicket,
		ViewProfileImages:     req.Options.ViewProfileImages,
		ViewFeaturesLocations: req.Options.ViewFeaturesLocations,
	}

	// Validate options
	locale := getProjectLocale()
	if err := h.limitationService.ValidateOptions(options); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, lang.Tf(locale, "invalid options: %v", err))
	}

	limitation, err := h.limitationService.Create(
		ctx,
		req.LimiterUserId,
		req.LimitedUserId,
		options,
		req.Note,
	)
	if err != nil {
		return nil, mapProfileLimitationError(err, locale)
	}

	return &pb.ProfileLimitationResponse{
		Data: convertProfileLimitationToProto(limitation, req.LimiterUserId),
	}, nil
}

func (h *profileLimitationHandler) UpdateProfileLimitation(ctx context.Context, req *pb.UpdateProfileLimitationRequest) (*pb.ProfileLimitationResponse, error) {
	// Convert proto options to model options
	options := models.ProfileLimitationOptions{
		Follow:                req.Options.Follow,
		SendMessage:           req.Options.SendMessage,
		Share:                 req.Options.Share,
		SendTicket:            req.Options.SendTicket,
		ViewProfileImages:     req.Options.ViewProfileImages,
		ViewFeaturesLocations: req.Options.ViewFeaturesLocations,
	}

	// Validate options
	locale := getProjectLocale()
	if err := h.limitationService.ValidateOptions(options); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, lang.Tf(locale, "invalid options: %v", err))
	}

	limitation, err := h.limitationService.Update(
		ctx,
		req.LimitationId,
		req.LimiterUserId,
		options,
		req.Note,
	)
	if err != nil {
		return nil, mapProfileLimitationError(err, locale)
	}

	return &pb.ProfileLimitationResponse{
		Data: convertProfileLimitationToProto(limitation, req.LimiterUserId),
	}, nil
}

func (h *profileLimitationHandler) DeleteProfileLimitation(ctx context.Context, req *pb.DeleteProfileLimitationRequest) (*emptypb.Empty, error) {
	if err := h.limitationService.Delete(ctx, req.LimitationId, req.LimiterUserId); err != nil {
		return nil, mapProfileLimitationError(err, getProjectLocale())
	}

	return &emptypb.Empty{}, nil
}

func (h *profileLimitationHandler) GetProfileLimitation(ctx context.Context, req *pb.GetProfileLimitationRequest) (*pb.ProfileLimitationResponse, error) {
	limitation, err := h.limitationService.GetByID(ctx, req.LimitationId)
	if err != nil {
		return nil, mapProfileLimitationError(err, getProjectLocale())
	}

	// Note: We need caller user ID to determine if note should be visible
	// For now, we'll include note if it exists (this should be handled by the caller)
	return &pb.ProfileLimitationResponse{
		Data: convertProfileLimitationToProto(limitation, limitation.LimiterUserID),
	}, nil
}

// mapProfileLimitationError maps service errors to gRPC status codes
func mapProfileLimitationError(err error, locale string) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, service.ErrProfileLimitationNotFound):
		return status.Errorf(codes.NotFound, "%s", err.Error())
	case errors.Is(err, service.ErrProfileLimitationAlreadyExists):
		// Return 403 Forbidden as per API documentation
		return status.Errorf(codes.PermissionDenied, "%s", err.Error())
	case errors.Is(err, service.ErrInvalidOptions):
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	case errors.Is(err, service.ErrNoteTooLong):
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		return status.Errorf(codes.NotFound, "%s", err.Error())
	case errors.Is(err, service.ErrUnauthorized):
		return status.Errorf(codes.PermissionDenied, "%s", err.Error())
	default:
		return status.Errorf(codes.Internal, lang.Tf(locale, "operation failed: %v", err))
	}
}

// convertProfileLimitationToProto converts a ProfileLimitation model to proto
// callerUserID is used to determine if note should be visible (only to limiter)
func convertProfileLimitationToProto(limitation *models.ProfileLimitation, callerUserID uint64) *pb.ProfileLimitation {
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
