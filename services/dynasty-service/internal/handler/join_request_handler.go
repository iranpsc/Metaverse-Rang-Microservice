package handler

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/service"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	"metarang/shared/pkg/helpers"
)

// JoinRequestHandler handles JoinRequestService gRPC methods
type JoinRequestHandler struct {
	dynastypb.UnimplementedJoinRequestServiceServer
	joinRequestService *service.JoinRequestService
	permissionService  *service.PermissionService
	userSearchService  *service.UserSearchService
}

// NewJoinRequestHandler creates a new join request handler
func NewJoinRequestHandler(
	joinRequestService *service.JoinRequestService,
	permissionService *service.PermissionService,
	userSearchService *service.UserSearchService,
) *JoinRequestHandler {
	return &JoinRequestHandler{
		joinRequestService: joinRequestService,
		permissionService:  permissionService,
		userSearchService:  userSearchService,
	}
}

// SendJoinRequest sends a join request to add a family member
func (h *JoinRequestHandler) SendJoinRequest(ctx context.Context, req *dynastypb.SendJoinRequestRequest) (*dynastypb.JoinRequestResponse, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	var permissions *models.ChildPermission
	if req.Permissions != nil && req.Relationship == "offspring" {
		permissions = &models.ChildPermission{
			Verified: req.Permissions.Verified,
			BFR:      req.Permissions.BFR,
			SF:       req.Permissions.SF,
			W:        req.Permissions.W,
			JU:       req.Permissions.JU,
			DM:       req.Permissions.DM,
			PIUP:     req.Permissions.PIUP,
			PITC:     req.Permissions.PITC,
			PIC:      req.Permissions.PIC,
			ESOO:     req.Permissions.ESOO,
			COTB:     req.Permissions.COTB,
		}
	}

	joinRequest, err := h.joinRequestService.SendJoinRequest(ctx, req.FromUserId, req.ToUserId, req.Relationship, nil, permissions)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Get user info
	toUserInfo, _ := h.joinRequestService.GetUserBasicInfo(ctx, req.ToUserId)

	// Get prize for relationship
	prize, _ := h.joinRequestService.GetPrizeByRelationship(ctx, req.Relationship)

	return buildJoinRequestResponse(joinRequest, toUserInfo, prize), nil
}

// GetSentRequests retrieves all sent join requests for a user
func (h *JoinRequestHandler) GetSentRequests(ctx context.Context, req *dynastypb.GetSentRequestsRequest) (*dynastypb.JoinRequestsResponse, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	page := int32(1)
	perPage := int32(10)
	if req.Pagination != nil {
		page = req.Pagination.Page
		perPage = req.Pagination.PerPage
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	requests, total, err := h.joinRequestService.GetSentRequests(ctx, req.UserId, page, perPage)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var responses []*dynastypb.JoinRequestResponse
	for _, req := range requests {
		toUserInfo, _ := h.joinRequestService.GetUserBasicInfo(ctx, req.ToUser)
		prize, _ := h.joinRequestService.GetPrizeByRelationship(ctx, req.Relationship)
		responses = append(responses, buildJoinRequestResponse(req, toUserInfo, prize))
	}

	return &dynastypb.JoinRequestsResponse{
		Requests: responses,
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}, nil
}

// GetReceivedRequests retrieves all received join requests for a user
func (h *JoinRequestHandler) GetReceivedRequests(ctx context.Context, req *dynastypb.GetReceivedRequestsRequest) (*dynastypb.JoinRequestsResponse, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	page := int32(1)
	perPage := int32(10)
	if req.Pagination != nil {
		page = req.Pagination.Page
		perPage = req.Pagination.PerPage
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}

	requests, total, err := h.joinRequestService.GetReceivedRequests(ctx, req.UserId, page, perPage)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var responses []*dynastypb.JoinRequestResponse
	for _, req := range requests {
		fromUserInfo, _ := h.joinRequestService.GetUserBasicInfo(ctx, req.FromUser)
		prize, _ := h.joinRequestService.GetPrizeByRelationship(ctx, req.Relationship)
		responses = append(responses, buildJoinRequestResponse(req, fromUserInfo, prize))
	}

	return &dynastypb.JoinRequestsResponse{
		Requests: responses,
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}, nil
}

// GetJoinRequest retrieves a specific join request by ID
func (h *JoinRequestHandler) GetJoinRequest(ctx context.Context, req *dynastypb.GetJoinRequestRequest) (*dynastypb.JoinRequestResponse, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	joinRequest, err := h.joinRequestService.GetJoinRequest(ctx, req.RequestId, req.UserId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	// Get user info (from or to depending on who is viewing)
	var userInfo *models.UserBasic
	if joinRequest.FromUser == req.UserId {
		userInfo, _ = h.joinRequestService.GetUserBasicInfo(ctx, joinRequest.ToUser)
	} else {
		userInfo, _ = h.joinRequestService.GetUserBasicInfo(ctx, joinRequest.FromUser)
	}

	prize, _ := h.joinRequestService.GetPrizeByRelationship(ctx, joinRequest.Relationship)

	return buildJoinRequestResponse(joinRequest, userInfo, prize), nil
}

// AcceptJoinRequest accepts a join request
func (h *JoinRequestHandler) AcceptJoinRequest(ctx context.Context, req *dynastypb.AcceptJoinRequestRequest) (*commonpb.Empty, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	err := h.joinRequestService.AcceptJoinRequest(ctx, req.RequestId, req.UserId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}

// RejectJoinRequest rejects a join request
func (h *JoinRequestHandler) RejectJoinRequest(ctx context.Context, req *dynastypb.RejectJoinRequestRequest) (*commonpb.Empty, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	err := h.joinRequestService.RejectJoinRequest(ctx, req.RequestId, req.UserId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}

// DeleteJoinRequest deletes a sent join request
func (h *JoinRequestHandler) DeleteJoinRequest(ctx context.Context, req *dynastypb.DeleteJoinRequestRequest) (*commonpb.Empty, error) {
	if h.joinRequestService == nil {
		return nil, status.Errorf(codes.Internal, "join request service not initialized")
	}

	err := h.joinRequestService.DeleteJoinRequest(ctx, req.RequestId, req.UserId)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}

// GetDefaultPermissions retrieves default permissions for offspring relationship
// Implements POST /api/dynasty/add/member/get/permissions
func (h *JoinRequestHandler) GetDefaultPermissions(ctx context.Context, req *dynastypb.GetDefaultPermissionsRequest) (*dynastypb.DefaultPermissionsResponse, error) {
	if h.permissionService == nil {
		return nil, status.Errorf(codes.Internal, "permission service not initialized")
	}

	// Validate relationship - must be "offspring"
	locale := getLocale(ctx)
	if req.Relationship != "offspring" {
		validationErrors := make(map[string]string)
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["relationship"] = fmt.Sprintf(t.Invalid, "relationship")
		return nil, returnValidationError(validationErrors)
	}

	perms, err := h.permissionService.GetDefaultPermissions(ctx)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &dynastypb.DefaultPermissionsResponse{
		Permissions: &dynastypb.ChildPermissions{
			BFR:  perms.BFR,
			SF:   perms.SF,
			W:    perms.W,
			JU:   perms.JU,
			DM:   perms.DM,
			PIUP: perms.PIUP,
			PITC: perms.PITC,
			PIC:  perms.PIC,
			ESOO: perms.ESOO,
			COTB: perms.COTB,
		},
	}, nil
}

// SearchUsers searches for users by code or name
// Implements POST /api/dynasty/search
func (h *JoinRequestHandler) SearchUsers(ctx context.Context, req *dynastypb.SearchUsersRequest) (*dynastypb.SearchUsersResponse, error) {
	if h.userSearchService == nil {
		return nil, status.Errorf(codes.Internal, "user search service not initialized")
	}

	// Validate search term
	locale := getLocale(ctx)
	if req.SearchTerm == "" {
		validationErrors := validateRequired("search_term", req.SearchTerm, locale)
		return nil, returnValidationError(validationErrors)
	}

	results, err := h.userSearchService.SearchUsers(ctx, req.SearchTerm, 20)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var protoResults []*dynastypb.UserSearchResult
	for _, r := range results {
		protoResults = append(protoResults, &dynastypb.UserSearchResult{
			Id:       r.ID,
			Code:     r.Code,
			Name:     r.Name,
			Image:    stringOrEmpty(r.Image),
			Level:    r.Level,
			Verified: r.Verified,
			Age:      r.Age,
			Levels:   toProtoUserSearchLevels(r.Levels),
		})
	}

	// Note: API returns { "data": [...] } but proto uses repeated field
	// Proto field name is "data" which becomes "Data" in Go
	return &dynastypb.SearchUsersResponse{
		Data: protoResults,
	}, nil
}
