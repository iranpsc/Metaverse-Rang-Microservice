package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "metarang/shared/pb/notifications"
	"metarang/shared/pkg/helpers"

	"metarang/notifications-service/internal/errs"
	"metarang/notifications-service/internal/models"
	"metarang/notifications-service/internal/service"
)

// EmailHandler implements the gRPC EmailService.
type EmailHandler struct {
	pb.UnimplementedEmailServiceServer
	service service.EmailService
}

// RegisterEmailHandler registers the email handler with the gRPC server.
func RegisterEmailHandler(grpcServer *grpc.Server, svc service.EmailService) {
	handler := &EmailHandler{service: svc}
	pb.RegisterEmailServiceServer(grpcServer, handler)
}

func (h *EmailHandler) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.EmailResponse, error) {
	if req.To == "" {
		return nil, status.Error(codes.InvalidArgument, "to is required")
	}
	if req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "subject is required")
	}
	if req.Body == "" && req.HtmlBody == "" {
		return nil, status.Error(codes.InvalidArgument, "either body or html_body is required")
	}

	to, err := helpers.ParseEmailAddress(req.To)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid to address")
	}
	cc, err := helpers.ParseEmailAddresses(req.Cc)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid cc address")
	}
	bcc, err := helpers.ParseEmailAddresses(req.Bcc)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid bcc address")
	}

	payload := models.EmailPayload{
		To:       to,
		Subject:  req.Subject,
		Body:     req.Body,
		HTMLBody: req.HtmlBody,
		CC:       cc,
		BCC:      bcc,
	}

	messageID, err := h.service.SendEmail(ctx, payload)
	if err != nil {
		return nil, handleEmailError(err)
	}

	return &pb.EmailResponse{
		Sent:      true,
		MessageId: messageID,
	}, nil
}

func handleEmailError(err error) error {
	if errors.Is(err, errs.ErrNotImplemented) {
		return status.Error(codes.Unimplemented, err.Error())
	}
	if errors.Is(err, helpers.ErrInvalidEmailAddress) {
		return status.Error(codes.InvalidArgument, "invalid email address")
	}
	return status.Errorf(codes.Internal, "email service error: %v", err)
}
