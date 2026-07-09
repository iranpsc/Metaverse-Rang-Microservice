package auth

import (
	"context"
	"crypto/subtle"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const internalServiceMetadataKey = "x-internal-service-secret"

type internalServiceContextKey struct{}

// InternalServiceSecret returns the shared secret used for service-to-service gRPC calls.
func InternalServiceSecret() string {
	return os.Getenv("INTERNAL_SERVICE_SECRET")
}

// AttachInternalServiceAuth adds the internal service secret to outgoing gRPC metadata.
func AttachInternalServiceAuth(ctx context.Context) context.Context {
	secret := InternalServiceSecret()
	if secret == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, internalServiceMetadataKey, secret)
}

// UnaryClientInterceptor attaches internal service credentials to outbound gRPC calls.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = AttachInternalServiceAuth(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// IsInternalServiceCall reports whether the request was authenticated as an internal service.
func IsInternalServiceCall(ctx context.Context) bool {
	if v := ctx.Value(internalServiceContextKey{}); v != nil {
		if ok, _ := v.(bool); ok {
			return ok
		}
	}
	return validateInternalServiceSecret(ctx)
}

func validateInternalServiceSecret(ctx context.Context) bool {
	expected := InternalServiceSecret()
	if expected == "" {
		return false
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	vals := md.Get(internalServiceMetadataKey)
	if len(vals) == 0 || vals[0] == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(vals[0]), []byte(expected)) == 1
}

func contextWithInternalService(ctx context.Context) context.Context {
	return context.WithValue(ctx, internalServiceContextKey{}, true)
}

func requiresInternalServiceOnly(fullMethod string) bool {
	internalOnly := []string{
		"/commercial.WalletService/AddBalance",
		"/commercial.WalletService/DeductBalance",
		"/commercial.WalletService/LockBalance",
		"/commercial.WalletService/UnlockBalance",
		"/commercial.TransactionService/CreateTransaction",
		"/financial.WalletService/AddBalance",
		"/financial.WalletService/DeductBalance",
		"/financial.WalletService/LockBalance",
		"/financial.WalletService/UnlockBalance",
	}

	for _, method := range internalOnly {
		if fullMethod == method {
			return true
		}
	}
	return false
}
