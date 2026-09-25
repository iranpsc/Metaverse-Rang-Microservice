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
