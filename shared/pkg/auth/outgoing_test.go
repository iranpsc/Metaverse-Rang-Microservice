package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestAttachOutgoingAuthFromUserContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserContextKey{}, &UserContext{
		UserID: 1,
		Token:  "test-token",
	})

	outCtx := AttachOutgoingAuth(ctx)
	md, ok := metadata.FromOutgoingContext(outCtx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	vals := md.Get("authorization")
	if len(vals) != 1 || vals[0] != "Bearer test-token" {
		t.Fatalf("authorization = %v, want Bearer test-token", vals)
	}
}

func TestAttachOutgoingAuthFromIncomingMetadata(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer incoming-token"))

	outCtx := AttachOutgoingAuth(ctx)
	md, ok := metadata.FromOutgoingContext(outCtx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	vals := md.Get("authorization")
	if len(vals) != 1 || vals[0] != "Bearer incoming-token" {
		t.Fatalf("authorization = %v, want Bearer incoming-token", vals)
	}
}
