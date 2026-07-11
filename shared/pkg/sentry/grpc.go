package sentry

import (
	"context"

	"github.com/getsentry/sentry-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor captures panics and server-side gRPC errors in Sentry.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		if !enabled {
			return handler(ctx, req)
		}

		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub().Clone()
			ctx = sentry.SetHubOnContext(ctx, hub)
		}

		hub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("grpc.method", info.FullMethod)
			scope.SetContext("grpc", map[string]interface{}{
				"method": info.FullMethod,
			})
		})

		defer func() {
			if recovered := recover(); recovered != nil {
				hub.RecoverWithContext(ctx, recovered)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		resp, err = handler(ctx, req)
		if err != nil && shouldReportGRPCError(err) {
			hub.CaptureException(err)
		}

		return resp, err
	}
}

func shouldReportGRPCError(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return true
	}

	switch st.Code() {
	case codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange,
		codes.Canceled,
		codes.DeadlineExceeded:
		return false
	default:
		return true
	}
}
