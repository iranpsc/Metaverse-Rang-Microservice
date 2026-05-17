package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metargb/auth-service/internal/lang"
	"metargb/auth-service/internal/repository"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
	"metargb/shared/pkg/helpers"
)

type authHandler struct {
	pb.UnimplementedAuthServiceServer
	authService         service.AuthService
	tokenRepo           repository.TokenRepository
	profilePhotoHandler *ProfilePhotoHandler
	locale              string
}

func RegisterAuthHandler(grpcServer *grpc.Server, authService service.AuthService, tokenRepo repository.TokenRepository, profilePhotoHandler *ProfilePhotoHandler, locale string) {
	pb.RegisterAuthServiceServer(grpcServer, &authHandler{
		authService:         authService,
		tokenRepo:           tokenRepo,
		profilePhotoHandler: profilePhotoHandler,
		locale:              lang.NormalizeLocale(locale),
	})
}

func (h *authHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// Validate back_url is required (matching Laravel: 'back_url' => 'required|url')
	validationErrors := make(map[string]string)

	if req.BackUrl == "" {
		t := helpers.GetLocaleTranslations(h.locale)
		validationErrors["back_url"] = fmt.Sprintf(t.Required, "back_url")
	}

	// If validation errors exist, return them
	if len(validationErrors) > 0 {
		encodedError := helpers.EncodeValidationError(validationErrors)
		return nil, status.Error(codes.InvalidArgument, encodedError)
	}

	url, err := h.authService.Register(ctx, req.BackUrl, req.Referral)
	if err != nil {
		// Check if it's a referral validation error
		if strings.Contains(err.Error(), "referral code does not exist") {
			t := helpers.GetLocaleTranslations(h.locale)
			validationErrors["referral"] = fmt.Sprintf(t.Invalid, "referral")
			encodedError := helpers.EncodeValidationError(validationErrors)
			return nil, status.Error(codes.InvalidArgument, encodedError)
		}
		return nil, status.Errorf(codes.Internal, lang.Tf(h.locale, "registration failed: %v", err))
	}

	return &pb.RegisterResponse{
		Url: url,
	}, nil
}

func (h *authHandler) Redirect(ctx context.Context, req *pb.RedirectRequest) (*pb.RedirectResponse, error) {
	url, _, err := h.authService.Redirect(ctx, req.RedirectTo, req.BackUrl)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(h.locale, "redirect failed: %v", err))
	}

	return &pb.RedirectResponse{
		Url: url,
	}, nil
}

func (h *authHandler) Callback(ctx context.Context, req *pb.CallbackRequest) (*pb.CallbackResponse, error) {
	// Extract IP from gRPC metadata if available
	ip := extractIPFromContext(ctx)
	
	result, err := h.authService.Callback(ctx, req.State, req.Code, ip)
	if err != nil {
		// Map InvalidArgumentException to InvalidArgument status code
		if strings.Contains(err.Error(), "invalid state value") {
			return nil, status.Errorf(codes.InvalidArgument, lang.Tf(h.locale, "invalid state value: %v", err))
		}
		return nil, status.Errorf(codes.Internal, lang.Tf(h.locale, "callback failed: %v", err))
	}

	return &pb.CallbackResponse{
		Token:       result.Token,
		ExpiresAt:   result.ExpiresAt,
		RedirectUrl: result.RedirectURL,
	}, nil
}

func (h *authHandler) GetMe(ctx context.Context, req *pb.GetMeRequest) (*pb.UserResponse, error) {
	userDetails, err := h.authService.GetMe(ctx, req.Token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, lang.Tf(h.locale, "authentication failed: %v", err))
	}

	// Default automatic_logout to 55 if 0 (matching Laravel: settings->automatic_logout ?: 55)
	automaticLogout := userDetails.AutomaticLogout
	if automaticLogout == 0 {
		automaticLogout = 55
	}

	response := &pb.UserResponse{
		Id:                         userDetails.ID,
		Name:                       userDetails.Name,
		Code:                       userDetails.Code,
		AutomaticLogout:            automaticLogout,
		Level:                      nil, // Set below if available
		Image:                      h.profilePhotoHandler.PrependGatewayURL(userDetails.Image),
		Notifications:              userDetails.Notifications,
		SocrePercentageToNextLevel: userDetails.ScorePercentageToNextLevel, // TYPO PRESERVED!
		UnasnweredQuestionsCount:   userDetails.UnansweredQuestionsCount,   // TYPO PRESERVED!
		HourlyProfitTimePercentage: userDetails.HourlyProfitTimePercentage,
		VerifiedKyc:                userDetails.VerifiedKYC,
		Birthdate:                  userDetails.Birthdate,
		// Token and AccessToken are omitted to match Laravel AuthenticatedUserResource structure
	}

	if userDetails.Level != nil {
		response.Level = &pb.Level{
			Id:    userDetails.Level.ID,
			Title: userDetails.Level.Title,
			Score: userDetails.Level.Score,
		}
	}

	return response, nil
}

func (h *authHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*emptypb.Empty, error) {
	// Validate token and get user
	user, err := h.tokenRepo.ValidateToken(ctx, req.Token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, lang.Tf(h.locale, "invalid token: %v", err))
	}

	// Extract IP and UserAgent from request context (if available)
	// TODO: Extract from gRPC metadata
	ip := ""
	userAgent := ""

	if err := h.authService.Logout(ctx, user.ID, ip, userAgent); err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(h.locale, "logout failed: %v", err))
	}

	return &emptypb.Empty{}, nil
}

func (h *authHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	user, err := h.authService.ValidateToken(ctx, req.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{
			Valid: false,
		}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:  true,
		UserId: user.ID,
		Email:  user.Email,
	}, nil
}

func (h *authHandler) RequestAccountSecurity(ctx context.Context, req *pb.RequestAccountSecurityRequest) (*emptypb.Empty, error) {
	// Validate time parameter
	validationErrors := make(map[string]string)

	t := helpers.GetLocaleTranslations(h.locale)
	switch {
	case req.TimeMinutes == 0:
		validationErrors["time"] = fmt.Sprintf(t.Required, "time")
	case req.TimeMinutes < 5 || req.TimeMinutes > 60:
		validationErrors["time"] = fmt.Sprintf(t.Invalid, "time")
	}

	// If validation errors exist, return them with field information
	if len(validationErrors) > 0 {
		encodedError := helpers.EncodeValidationError(validationErrors)
		return nil, status.Error(codes.InvalidArgument, encodedError)
	}

	if err := h.authService.RequestAccountSecurity(ctx, req.UserId, req.TimeMinutes, req.Phone); err != nil {
		return nil, mapAccountSecurityErrorWithFields(err, h.locale)
	}
	return &emptypb.Empty{}, nil
}

func (h *authHandler) VerifyAccountSecurity(ctx context.Context, req *pb.VerifyAccountSecurityRequest) (*emptypb.Empty, error) {
	// Validate code parameter
	validationErrors := make(map[string]string)

	if req.Code == "" {
		t := helpers.GetLocaleTranslations(h.locale)
		validationErrors["code"] = fmt.Sprintf(t.Required, "code")
	} else if len(req.Code) != 6 {
		t := helpers.GetLocaleTranslations(h.locale)
		validationErrors["code"] = fmt.Sprintf(t.Len, "code", "6")
	} else {
		// Validate that code contains only digits
		allDigits := true
		for _, char := range req.Code {
			if char < '0' || char > '9' {
				allDigits = false
				break
			}
		}
		if !allDigits {
			t := helpers.GetLocaleTranslations(h.locale)
			validationErrors["code"] = fmt.Sprintf(t.Invalid, "code")
		}
	}

	// If validation errors exist, return them with field information
	if len(validationErrors) > 0 {
		encodedError := helpers.EncodeValidationError(validationErrors)
		return nil, status.Error(codes.InvalidArgument, encodedError)
	}

	if err := h.authService.VerifyAccountSecurity(ctx, req.UserId, req.Code, req.Ip, req.UserAgent); err != nil {
		return nil, mapAccountSecurityErrorWithFields(err, h.locale)
	}
	return &emptypb.Empty{}, nil
}

func mapAccountSecurityError(err error, locale string) error {
	return mapAccountSecurityErrorWithFields(err, locale)
}

func mapAccountSecurityErrorWithFields(err error, locale string) error {
	validationErrors := make(map[string]string)

	switch {
	case errors.Is(err, service.ErrInvalidOTPCode):
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["code"] = fmt.Sprintf(t.Invalid, "code")
		encodedError := helpers.EncodeValidationError(validationErrors)
		return status.Error(codes.InvalidArgument, encodedError)
	case errors.Is(err, service.ErrPhoneRequired):
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["phone"] = fmt.Sprintf(t.Required, "phone")
		encodedError := helpers.EncodeValidationError(validationErrors)
		return status.Error(codes.InvalidArgument, encodedError)
	case errors.Is(err, service.ErrInvalidPhoneFormat):
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["phone"] = fmt.Sprintf(t.IranianMobile, "phone")
		encodedError := helpers.EncodeValidationError(validationErrors)
		return status.Error(codes.InvalidArgument, encodedError)
	case errors.Is(err, service.ErrPhoneAlreadyTaken):
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["phone"] = fmt.Sprintf(t.Unique, "phone")
		encodedError := helpers.EncodeValidationError(validationErrors)
		return status.Error(codes.InvalidArgument, encodedError)
	case errors.Is(err, service.ErrInvalidUnlockDuration):
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["time"] = fmt.Sprintf(t.Invalid, "time")
		encodedError := helpers.EncodeValidationError(validationErrors)
		return status.Error(codes.InvalidArgument, encodedError)
	case errors.Is(err, service.ErrAccountSecurityNotFound):
		return status.Errorf(codes.InvalidArgument, "%v", err)
	case errors.Is(err, service.ErrUserNotFound):
		return status.Errorf(codes.NotFound, "%v", err)
	case errors.Is(err, service.ErrAccountSecurityAlreadyUnlocked):
		return status.Errorf(codes.FailedPrecondition, "%v", err)
	case errors.Is(err, service.ErrVerificationRequestRateLimited):
		return status.Errorf(codes.ResourceExhausted, "%v", err)
	default:
		return status.Errorf(codes.Internal, lang.Tf(locale, "account security operation failed: %v", err))
	}
}

// extractIPFromContext extracts the IP address from gRPC metadata
// Looks for x-forwarded-for, x-real-ip, or peer address
func extractIPFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Check for X-Forwarded-For header
	if values := md.Get("x-forwarded-for"); len(values) > 0 {
		// Take the first IP if multiple are present
		ips := strings.Split(values[0], ",")
		return strings.TrimSpace(ips[0])
	}

	// Check for X-Real-IP header
	if values := md.Get("x-real-ip"); len(values) > 0 {
		return values[0]
	}

	// Could also extract from peer.Peer if needed
	return ""
}
