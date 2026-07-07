package handler

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metargb/auth-service/internal/lang"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
	"metargb/shared/pkg/helpers"
)

type walletConnectionHandler struct {
	pb.UnimplementedWalletConnectionServiceServer
	walletService service.WalletConnectionService
	locale        string
}

func RegisterWalletConnectionHandler(grpcServer *grpc.Server, walletService service.WalletConnectionService, locale string) {
	pb.RegisterWalletConnectionServiceServer(grpcServer, &walletConnectionHandler{
		walletService: walletService,
		locale:        lang.NormalizeLocale(locale),
	})
}

func (h *walletConnectionHandler) GetLinkNonce(ctx context.Context, req *pb.GetWalletLinkNonceRequest) (*pb.GetWalletNonceResponse, error) {
	if err := validateWalletAddressField(req.Address, h.locale); err != nil {
		return nil, err
	}

	nonce, err := h.walletService.GetLinkNonce(ctx, req.UserId, req.Address)
	if err != nil {
		return nil, mapWalletConnectionError(err, h.locale)
	}

	return &pb.GetWalletNonceResponse{Nonce: nonce}, nil
}

func (h *walletConnectionHandler) LinkWallet(ctx context.Context, req *pb.LinkWalletRequest) (*pb.LinkWalletResponse, error) {
	if err := validateWalletLinkRequest(req, h.locale); err != nil {
		return nil, err
	}

	address, err := h.walletService.LinkWallet(ctx, req.UserId, req.Address, req.Signature, req.Ip)
	if err != nil {
		return nil, mapWalletConnectionError(err, h.locale)
	}

	return &pb.LinkWalletResponse{
		Message:       "Wallet connected successfully",
		WalletAddress: address,
	}, nil
}

func (h *walletConnectionHandler) GetSecurityNonce(ctx context.Context, req *pb.GetWalletSecurityNonceRequest) (*pb.GetWalletNonceResponse, error) {
	if err := validateWalletAddressField(req.Address, h.locale); err != nil {
		return nil, err
	}

	nonce, err := h.walletService.GetSecurityNonce(ctx, req.UserId, req.Address)
	if err != nil {
		return nil, mapWalletConnectionError(err, h.locale)
	}

	return &pb.GetWalletNonceResponse{Nonce: nonce}, nil
}

func (h *walletConnectionHandler) VerifySecuritySignature(ctx context.Context, req *pb.VerifyWalletSecuritySignatureRequest) (*pb.VerifyWalletSecuritySignatureResponse, error) {
	if err := validateWalletSecurityVerifyRequest(req, h.locale); err != nil {
		return nil, err
	}

	until, err := h.walletService.VerifySecuritySignature(
		ctx,
		req.UserId,
		req.Address,
		req.Signature,
		req.Duration,
		req.Ip,
		req.UserAgent,
	)
	if err != nil {
		return nil, mapWalletConnectionError(err, h.locale)
	}

	return &pb.VerifyWalletSecuritySignatureResponse{
		Message: "Account security unlocked successfully",
		Until:   until,
	}, nil
}

func validateWalletAddressField(address, locale string) error {
	if address == "" {
		t := helpers.GetLocaleTranslations(locale)
		encodedError := helpers.EncodeValidationError(map[string]string{
			"address": fmt.Sprintf(t.Required, "address"),
		})
		return status.Error(codes.InvalidArgument, encodedError)
	}
	return nil
}

func validateWalletLinkRequest(req *pb.LinkWalletRequest, locale string) error {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	if req.Address == "" {
		validationErrors["address"] = fmt.Sprintf(t.Required, "address")
	}
	if req.Signature == "" {
		validationErrors["signature"] = fmt.Sprintf(t.Required, "signature")
	}

	if len(validationErrors) > 0 {
		return status.Error(codes.InvalidArgument, helpers.EncodeValidationError(validationErrors))
	}
	return nil
}

func validateWalletSecurityVerifyRequest(req *pb.VerifyWalletSecuritySignatureRequest, locale string) error {
	validationErrors := make(map[string]string)
	t := helpers.GetLocaleTranslations(locale)

	if req.Address == "" {
		validationErrors["address"] = fmt.Sprintf(t.Required, "address")
	}
	if req.Signature == "" {
		validationErrors["signature"] = fmt.Sprintf(t.Required, "signature")
	}
	if req.Duration == 0 {
		validationErrors["duration"] = fmt.Sprintf(t.Required, "duration")
	} else if req.Duration < 5 || req.Duration > 120 {
		validationErrors["duration"] = fmt.Sprintf(t.Invalid, "duration")
	}

	if len(validationErrors) > 0 {
		return status.Error(codes.InvalidArgument, helpers.EncodeValidationError(validationErrors))
	}
	return nil
}

func mapWalletConnectionError(err error, locale string) error {
	switch {
	case errors.Is(err, service.ErrInvalidWalletAddress):
		t := helpers.GetLocaleTranslations(locale)
		return status.Error(codes.InvalidArgument, helpers.EncodeValidationError(map[string]string{
			"address": fmt.Sprintf(t.Invalid, "address"),
		}))
	case errors.Is(err, service.ErrInvalidWalletSignature):
		t := helpers.GetLocaleTranslations(locale)
		return status.Error(codes.InvalidArgument, helpers.EncodeValidationError(map[string]string{
			"signature": fmt.Sprintf(t.Invalid, "signature"),
		}))
	case errors.Is(err, service.ErrInvalidWalletSecurityDuration):
		t := helpers.GetLocaleTranslations(locale)
		return status.Error(codes.InvalidArgument, helpers.EncodeValidationError(map[string]string{
			"duration": fmt.Sprintf(t.Invalid, "duration"),
		}))
	case errors.Is(err, service.ErrWalletAlreadyConnected),
		errors.Is(err, service.ErrWalletAlreadyLinked),
		errors.Is(err, service.ErrWalletNonceExpired):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, service.ErrWalletSignatureFailed):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, service.ErrWalletNotConnectedToAccount):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Errorf(codes.Internal, "%s", lang.Tf(locale, "wallet connection operation failed: %v", err))
	}
}
