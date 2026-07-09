package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const permissionDeniedMessage = "forbidden"

// ErrInvalidToken is returned when a token validation fails
var ErrInvalidToken = errors.New("invalid token")

// UserContextKey is the key for user data in context
type UserContextKey struct{}

// UserContext holds authenticated user information
type UserContext struct {
	UserID uint64
	Email  string
	Token  string
}

// TokenValidator interface for validating tokens
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*UserContext, error)
}

// UnaryServerInterceptor returns a new unary server interceptor for authentication
func UnaryServerInterceptor(validator TokenValidator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Skip authentication for certain methods (e.g., health checks, public endpoints)
		if shouldSkipAuth(info.FullMethod) {
			return handler(ctx, req)
		}

		// Internal service-only RPCs (wallet mutations, etc.)
		if requiresInternalServiceOnly(info.FullMethod) {
			if !validateInternalServiceSecret(ctx) {
				return nil, status.Error(codes.PermissionDenied, permissionDeniedMessage)
			}
			ctx = contextWithInternalService(ctx)
			return handler(ctx, req)
		}

		// Service-to-service calls authenticated with the internal secret
		if validateInternalServiceSecret(ctx) {
			ctx = contextWithInternalService(ctx)
			return handler(ctx, req)
		}

		// Optional auth: proceed without a token; attach user context when a valid token is sent
		if shouldUseOptionalAuth(info.FullMethod) {
			ctx = contextWithOptionalAuth(ctx, validator)
			return handler(ctx, req)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := extractToken(authHeader[0])
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		// Validate token
		userCtx, err := validator.ValidateToken(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("invalid token: %v", err))
		}

		// Add user context
		ctx = context.WithValue(ctx, UserContextKey{}, userCtx)

		return handler(ctx, req)
	}
}

// StreamServerInterceptor returns a new stream server interceptor for authentication
func StreamServerInterceptor(validator TokenValidator) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Skip authentication for certain methods
		if shouldSkipAuth(info.FullMethod) {
			return handler(srv, stream)
		}

		ctx := stream.Context()

		if requiresInternalServiceOnly(info.FullMethod) {
			if !validateInternalServiceSecret(ctx) {
				return status.Error(codes.PermissionDenied, permissionDeniedMessage)
			}
			ctx = contextWithInternalService(ctx)
			wrappedStream := &wrappedServerStream{ServerStream: stream, ctx: ctx}
			return handler(srv, wrappedStream)
		}

		if validateInternalServiceSecret(ctx) {
			ctx = contextWithInternalService(ctx)
			wrappedStream := &wrappedServerStream{ServerStream: stream, ctx: ctx}
			return handler(srv, wrappedStream)
		}

		// Optional auth: proceed without a token; attach user context when a valid token is sent
		if shouldUseOptionalAuth(info.FullMethod) {
			ctx = contextWithOptionalAuth(ctx, validator)
			wrappedStream := &wrappedServerStream{
				ServerStream: stream,
				ctx:          ctx,
			}
			return handler(srv, wrappedStream)
		}

		// Extract token from metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := extractToken(authHeader[0])
		if token == "" {
			return status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		// Validate token
		userCtx, err := validator.ValidateToken(ctx, token)
		if err != nil {
			return status.Error(codes.Unauthenticated, fmt.Sprintf("invalid token: %v", err))
		}

		// Add user context
		ctx = context.WithValue(ctx, UserContextKey{}, userCtx)

		// Wrap stream with new context
		wrappedStream := &wrappedServerStream{
			ServerStream: stream,
			ctx:          ctx,
		}

		return handler(srv, wrappedStream)
	}
}

// extractToken extracts the token from "Bearer <token>" format
func extractToken(authHeader string) string {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return parts[1]
}

// shouldUseOptionalAuth checks if authentication is optional for a method.
// Matches Laravel routes that work without auth but enrich the response when a bearer token is present.
func shouldUseOptionalAuth(fullMethod string) bool {
	optionalMethods := []string{
		"/features.FeatureService/ListFeatures",
		"/features.FeatureService/GetFeature",
		"/features.MapsService/ListMaps",
		"/features.MapsService/GetMap",
		"/features.MapsService/GetMapBorder",
		"/financial.StoreService/GetStorePackages",
	}

	for _, method := range optionalMethods {
		if fullMethod == method {
			return true
		}
	}
	return false
}

// contextWithOptionalAuth validates the token when present and ignores missing/invalid tokens.
func contextWithOptionalAuth(ctx context.Context, validator TokenValidator) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		return ctx
	}

	token := extractToken(authHeader[0])
	if token == "" {
		return ctx
	}

	userCtx, err := validator.ValidateToken(ctx, token)
	if err != nil {
		return ctx
	}

	return context.WithValue(ctx, UserContextKey{}, userCtx)
}

// shouldSkipAuth checks if authentication should be skipped for a method
func shouldSkipAuth(fullMethod string) bool {
	// List of methods that don't require authentication
	publicMethods := []string{
		// Health checks
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		// Auth service public endpoints (OAuth flow)
		"/auth.AuthService/Register",
		"/auth.AuthService/Login",
		"/auth.AuthService/Redirect",
		"/auth.AuthService/Callback",
		"/auth.AuthService/ValidateToken", // Other services call this to validate tokens
		// Financial service payment gateway callback (verified with Sadad before crediting)
		"/financial.OrderService/HandleCallback",
	}

	for _, method := range publicMethods {
		if fullMethod == method {
			return true
		}
	}
	return false
}

// GetUserFromContext retrieves user context from the context
func GetUserFromContext(ctx context.Context) (*UserContext, error) {
	userCtx, ok := ctx.Value(UserContextKey{}).(*UserContext)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user context not found")
	}
	return userCtx, nil
}

// wrappedServerStream wraps grpc.ServerStream to override context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
