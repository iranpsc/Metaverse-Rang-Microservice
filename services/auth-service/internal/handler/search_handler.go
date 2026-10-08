package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
	sharedauth "metarang/shared/pkg/auth"
)

type searchHandler struct {
	pb.UnimplementedSearchServiceServer
	searchService service.SearchService
}

func RegisterSearchHandler(grpcServer *grpc.Server, searchService service.SearchService) pb.SearchServiceServer {
	h := NewSearchHandler(searchService)
	pb.RegisterSearchServiceServer(grpcServer, h)
	return h
}

// SearchUsers handles user search requests
func (h *searchHandler) SearchUsers(ctx context.Context, req *pb.SearchUsersRequest) (*pb.SearchUsersResponse, error) {
	// Validate request
	if req.SearchTerm == "" {
		return &pb.SearchUsersResponse{
			Data: []*pb.SearchUserResult{},
		}, nil
	}

	var viewerUserID uint64
	if userCtx, err := sharedauth.GetUserFromContext(ctx); err == nil && userCtx != nil && userCtx.UserID > 0 {
		viewerUserID = userCtx.UserID
	}

	// Call service
	results, err := h.searchService.SearchUsers(ctx, req.SearchTerm, viewerUserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Convert service results to protobuf
	pbResults := make([]*pb.SearchUserResult, 0, len(results))
	for _, result := range results {
		pbResult := &pb.SearchUserResult{
			Id:          result.ID,
			Code:        result.Code,
			Name:        result.Name,
			Followers:   result.Followers,
			IsFollowing: result.IsFollowing,
		}

		if result.Level != nil {
			pbResult.Level = *result.Level
		}
		if result.Photo != nil {
			pbResult.Photo = *result.Photo
		}

		pbResults = append(pbResults, pbResult)
	}

	return &pb.SearchUsersResponse{
		Data: pbResults,
	}, nil
}

// SearchIsicCodes handles ISIC code search requests
func (h *searchHandler) SearchIsicCodes(ctx context.Context, req *pb.SearchIsicCodesRequest) (*pb.SearchIsicCodesResponse, error) {
	// Validate request
	if req.SearchTerm == "" {
		return &pb.SearchIsicCodesResponse{
			Data: []*pb.IsicCodeResult{},
		}, nil
	}

	// Call service
	results, err := h.searchService.SearchIsicCodes(ctx, req.SearchTerm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Convert service results to protobuf
	pbResults := make([]*pb.IsicCodeResult, 0, len(results))
	for _, result := range results {
		pbResults = append(pbResults, &pb.IsicCodeResult{
			Id:   result.ID,
			Name: result.Name,
			Code: result.Code,
		})
	}

	return &pb.SearchIsicCodesResponse{
		Data: pbResults,
	}, nil
}
