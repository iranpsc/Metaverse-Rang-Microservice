package auth

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// AttachOutgoingAuth copies the bearer token from the current context into outgoing
// gRPC metadata so downstream service calls inherit the caller's authentication.
func AttachOutgoingAuth(ctx context.Context) context.Context {
	ctx = AttachInternalServiceAuth(ctx)

	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vals := md.Get("authorization"); len(vals) > 0 && vals[0] != "" {
			return ctx
		}
	}

	if userCtx, ok := ctx.Value(UserContextKey{}).(*UserContext); ok && userCtx != nil && userCtx.Token != "" {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+userCtx.Token)
	}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("authorization"); len(vals) > 0 && vals[0] != "" {
			return metadata.AppendToOutgoingContext(ctx, "authorization", vals[0])
		}
	}

	return ctx
}
