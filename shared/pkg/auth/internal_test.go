package auth

import (
	"context"
	"os"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestValidateInternalServiceSecret(t *testing.T) {
	t.Setenv("INTERNAL_SERVICE_SECRET", "test-secret")

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalServiceMetadataKey, "test-secret"))
	if !validateInternalServiceSecret(ctx) {
		t.Fatal("expected valid internal service secret")
	}

	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalServiceMetadataKey, "wrong"))
	if validateInternalServiceSecret(ctx) {
		t.Fatal("expected invalid secret to be rejected")
	}
}

func TestRequiresInternalServiceOnly(t *testing.T) {
	if !requiresInternalServiceOnly("/commercial.WalletService/AddBalance") {
		t.Fatal("wallet mutation should require internal auth")
	}
	if requiresInternalServiceOnly("/commercial.WalletService/GetWallet") {
		t.Fatal("get wallet should not be internal-only")
	}
}

func TestInternalServiceSecretEmpty(t *testing.T) {
	_ = os.Unsetenv("INTERNAL_SERVICE_SECRET")
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalServiceMetadataKey, "anything"))
	if validateInternalServiceSecret(ctx) {
		t.Fatal("expected rejection when secret is unset")
	}
}
