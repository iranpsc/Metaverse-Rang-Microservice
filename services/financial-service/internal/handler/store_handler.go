package handler

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/service"
	pb "metarang/shared/pb/financial"
)

type StoreHandler struct {
	pb.UnimplementedStoreServiceServer
	storeService service.StoreService
}

func NewStoreHandler(storeService service.StoreService) *StoreHandler {
	return &StoreHandler{
		storeService: storeService,
	}
}

func RegisterStoreHandler(grpcServer *grpc.Server, storeService service.StoreService) {
	handler := NewStoreHandler(storeService)
	pb.RegisterStoreServiceServer(grpcServer, handler)
}

func (h *StoreHandler) GetStorePackages(ctx context.Context, req *pb.GetStorePackagesRequest) (*pb.GetStorePackagesResponse, error) {
	locale := GetLocaleFromContext(ctx)
	validationErrors := mergeValidationErrors(
		validateMin("codes", int64(len(req.Codes)), constants.MinStoreCodes, locale),
	)

	for i, code := range req.Codes {
		validationErrors = mergeValidationErrors(
			validationErrors,
			validateMin(fmt.Sprintf("codes.%d", i), int64(len(code)), constants.MinStoreCodeLength, locale),
		)
	}

	if len(validationErrors) > 0 {
		return nil, returnValidationError(validationErrors)
	}

	packages, err := h.storeService.GetStorePackages(ctx, req.Codes)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCodes) || errors.Is(err, service.ErrInvalidCodeLength) {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to get store packages: %v", err)
	}

	pbPackages := make([]*pb.Package, 0, len(packages))
	for _, pkg := range packages {
		pbPackage := &pb.Package{
			Id:        pkg.ID,
			Code:      pkg.Code,
			Asset:     pkg.Asset,
			Amount:    pkg.Amount,
			UnitPrice: pkg.UnitPrice,
		}
		if pkg.Image != nil && *pkg.Image != "" {
			pbPackage.Image = pkg.Image
		}
		pbPackages = append(pbPackages, pbPackage)
	}

	return &pb.GetStorePackagesResponse{
		Packages: pbPackages,
	}, nil
}
