package handler

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestGetLocaleFromContext(t *testing.T) {
	t.Run("defaults to en", func(t *testing.T) {
		if got := GetLocaleFromContext(context.Background()); got != "en" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("fa from grpcgateway prefix", func(t *testing.T) {
		md := metadata.Pairs("grpcgateway-accept-language", "fa-IR,en;q=0.9")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		if got := GetLocaleFromContext(ctx); got != "fa" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("fa from accept-language", func(t *testing.T) {
		md := metadata.Pairs("accept-language", "fa")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		if got := GetLocaleFromContext(ctx); got != "fa" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("en from primary tag", func(t *testing.T) {
		md := metadata.Pairs("accept-language", "en-US")
		ctx := metadata.NewIncomingContext(context.Background(), md)
		if got := GetLocaleFromContext(ctx); got != "en" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestValidateMinLength(t *testing.T) {
	errs := validateMinLength("codes", "a", 2, "en")
	if errs["codes"] == "" {
		t.Fatal("expected error")
	}
	if len(validateMinLength("codes", "ab", 2, "en")) > 0 {
		t.Fatal("unexpected error")
	}
}
