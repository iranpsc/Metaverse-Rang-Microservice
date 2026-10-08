package handler

import (
	"metarang/auth-service/internal/lang"
	"metarang/auth-service/internal/repository"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

// NewAuthHandler constructs an AuthServiceServer for handler tests and registration.
func NewAuthHandler(authService service.AuthService, tokenRepo repository.TokenRepository, profilePhotoService service.ProfilePhotoService, locale string, opts ...AuthHandlerOption) pb.AuthServiceServer {
	h := &authHandler{
		authService:         authService,
		tokenRepo:           tokenRepo,
		profilePhotoService: profilePhotoService,
		locale:              lang.NormalizeLocale(locale),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(h)
		}
	}
	return h
}

// NewSearchHandler constructs a SearchServiceServer for handler tests and registration.
func NewSearchHandler(searchService service.SearchService) pb.SearchServiceServer {
	return &searchHandler{searchService: searchService}
}

// NewUserHandler constructs a UserServiceServer for handler tests and registration.
func NewUserHandler(userService service.UserService, profileLimitationService service.ProfileLimitationService, helperService service.HelperService) pb.UserServiceServer {
	return &userHandler{
		userService:              userService,
		profileLimitationService: profileLimitationService,
		helperService:            helperService,
	}
}

// NewProfileLimitationHandler constructs a ProfileLimitationServiceServer for handler tests and registration.
func NewProfileLimitationHandler(limitationService service.ProfileLimitationService) pb.ProfileLimitationServiceServer {
	return &profileLimitationHandler{limitationService: limitationService}
}

// NewKYCHandler constructs a KYCServiceServer for handler tests and registration.
func NewKYCHandler(kycService service.KYCService, apiGatewayURL string) pb.KYCServiceServer {
	return &kycHandler{
		kycService:    kycService,
		apiGatewayURL: apiGatewayURL,
	}
}
