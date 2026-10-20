package auth

import (
	"context"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRequiresServiceAuth(t *testing.T) {
	if !RequiresServiceAuth("/commercial.AssetRateService/GetAssetRate") {
		t.Fatal("expected GetAssetRate to require service auth")
	}
	if !RequiresServiceAuth("/commercial.WalletService/AddBalance") {
		t.Fatal("expected AddBalance to require service auth")
	}
	if RequiresServiceAuth("/commercial.WalletService/GetWallet") {
		t.Fatal("expected GetWallet to remain public")
	}
}

func TestValidateServiceToken(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")

	if !ValidateServiceToken("test-secret") {
		t.Fatal("expected valid service token")
	}
	if ValidateServiceToken("wrong-secret") {
		t.Fatal("expected invalid service token")
	}
}

func TestUnaryServerInterceptor_ServiceAuthRequired(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")

	interceptor := UnaryServerInterceptor(&stubValidator{})
	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}
	info := &grpc.UnaryServerInfo{FullMethod: "/commercial.WalletService/AddBalance"}

	_, err := interceptor(context.Background(), nil, info, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without token, got %v", err)
	}

	md := metadata.Pairs(ServiceTokenMetadataKey, "test-secret")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	resp, err := interceptor(ctx, nil, info, handler)
	if err != nil {
		t.Fatalf("unexpected error with valid service token: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called with valid service token")
	}
	if resp != "ok" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestAttachOutgoingServiceAuth(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")

	outCtx := AttachOutgoingServiceAuth(context.Background())
	md, ok := metadata.FromOutgoingContext(outCtx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	vals := md.Get(ServiceTokenMetadataKey)
	if len(vals) != 1 || vals[0] != "test-secret" {
		t.Fatalf("service token = %v, want test-secret", vals)
	}
}

func TestAttachOutgoingServiceAuth_NoSecret(t *testing.T) {
	os.Unsetenv("INTERNAL_SERVICE_SECRET")

	outCtx := AttachOutgoingServiceAuth(context.Background())
	if _, ok := metadata.FromOutgoingContext(outCtx); ok {
		t.Fatal("expected no outgoing metadata without configured secret")
	}
}
