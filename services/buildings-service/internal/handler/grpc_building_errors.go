package handler

import (
	"strings"

	"metarang/buildings-service/internal/lang"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type buildingErrorMap struct {
	mapNotFound     bool
	mapPrecondition bool
}

func mapEntryServiceError(err error, locale string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unauthorized") || strings.Contains(msg, "does not own"):
		return status.Errorf(codes.PermissionDenied, "%s", msg)
	case strings.Contains(msg, "not found"):
		return status.Errorf(codes.NotFound, "%s", msg)
	case strings.Contains(msg, "already exists"):
		return status.Errorf(codes.AlreadyExists, "%s", msg)
	case strings.Contains(msg, "unavailable"):
		return status.Errorf(codes.Unavailable, "%s", msg)
	case strings.Contains(msg, "invalid"):
		return status.Errorf(codes.InvalidArgument, "%s", msg)
	case strings.Contains(msg, "outcome unknown"),
		strings.Contains(msg, "needs reconciliation"):
		return status.Error(codes.FailedPrecondition, "payment outcome unknown")
	case strings.Contains(msg, "insufficient"),
		strings.Contains(msg, "already"),
		strings.Contains(msg, "not active"),
		strings.Contains(msg, "not finished"),
		strings.Contains(msg, "outside"),
		strings.Contains(msg, "expired"),
		strings.Contains(msg, "limit reached"):
		return status.Errorf(codes.FailedPrecondition, "%s", msg)
	default:
		return status.Error(codes.Internal, lang.T(locale, "internal server error"))
	}
}

func mapBuildingServiceError(err error, locale string, opts buildingErrorMap) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "unauthorized") || strings.Contains(msg, "does not own") {
		return status.Errorf(codes.PermissionDenied, "%s", msg)
	}
	if opts.mapNotFound && strings.Contains(msg, "not found") {
		return status.Errorf(codes.NotFound, "%s", msg)
	}
	if opts.mapPrecondition && (strings.Contains(msg, "already has") || strings.Contains(msg, "insufficient")) {
		return status.Errorf(codes.FailedPrecondition, "%s", msg)
	}
	if strings.Contains(msg, "invalid") {
		return status.Errorf(codes.InvalidArgument, "%s", msg)
	}
	return status.Error(codes.Internal, lang.T(locale, "internal server error"))
}
