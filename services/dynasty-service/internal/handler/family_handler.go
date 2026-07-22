package handler

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/service"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	"metarang/shared/pkg/helpers"
)

// FamilyHandler handles FamilyService gRPC methods
type FamilyHandler struct {
	dynastypb.UnimplementedFamilyServiceServer
	familyService     *service.FamilyService
	permissionService *service.PermissionService
}

// NewFamilyHandler creates a new family handler
func NewFamilyHandler(
	familyService *service.FamilyService,
	permissionService *service.PermissionService,
) *FamilyHandler {
	return &FamilyHandler{
		familyService:     familyService,
		permissionService: permissionService,
	}
}

// GetFamily retrieves a family by ID and dynasty ID
func (h *FamilyHandler) GetFamily(ctx context.Context, req *dynastypb.GetFamilyRequest) (*dynastypb.FamilyResponse, error) {
	if h.familyService == nil {
		return nil, status.Errorf(codes.Internal, "family service not initialized")
	}

	family, err := h.familyService.GetFamily(ctx, req.FamilyId, req.DynastyId)
	if err != nil {
		return nil, mapServiceError(err)
	}
	if family == nil {
		return nil, status.Errorf(codes.NotFound, "family not found")
	}

	// Get members
	members, _, err := h.familyService.GetFamilyMembers(ctx, family.ID, 1, 1000)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var protoMembers []*dynastypb.FamilyMember
	for _, m := range members {
		userInfo, _ := h.familyService.GetUserBasicInfo(ctx, m.UserID)
		protoMembers = append(protoMembers, &dynastypb.FamilyMember{
			Id:           m.ID,
			UserId:       m.UserID,
			Relationship: m.Relationship,
			UserInfo:     buildUserBasic(userInfo),
			CreatedAt:    formatJalaliDate(m.CreatedAt),
		})
	}

	return &dynastypb.FamilyResponse{
		Id:        family.ID,
		DynastyId: family.DynastyID,
		Members:   protoMembers,
	}, nil
}

// GetFamilyMembers retrieves family members with pagination
func (h *FamilyHandler) GetFamilyMembers(ctx context.Context, req *dynastypb.GetFamilyMembersRequest) (*dynastypb.FamilyMembersResponse, error) {
	if h.familyService == nil {
		return nil, status.Errorf(codes.Internal, "family service not initialized")
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

	members, total, err := h.familyService.GetFamilyMembers(ctx, req.FamilyId, page, perPage)
	if err != nil {
		return nil, mapServiceError(err)
	}

	var protoMembers []*dynastypb.FamilyMember
	for _, m := range members {
		userInfo, _ := h.familyService.GetUserBasicInfo(ctx, m.UserID)
		protoMembers = append(protoMembers, &dynastypb.FamilyMember{
			Id:           m.ID,
			UserId:       m.UserID,
			Relationship: m.Relationship,
			UserInfo:     buildUserBasic(userInfo),
			CreatedAt:    formatJalaliDate(m.CreatedAt),
		})
	}

	return &dynastypb.FamilyMembersResponse{
		Members: protoMembers,
		Pagination: &commonpb.PaginationMeta{
			CurrentPage: page,
			PerPage:     perPage,
			Total:       total,
			LastPage:    (total + perPage - 1) / perPage,
		},
	}, nil
}

// SetChildPermissions updates a single permission for a child
// Implements POST /api/dynasty/children/{user}
func (h *FamilyHandler) SetChildPermissions(ctx context.Context, req *dynastypb.SetChildPermissionsRequest) (*commonpb.Empty, error) {
	if h.permissionService == nil {
		return nil, status.Errorf(codes.Internal, "permission service not initialized")
	}

	// API spec: POST /api/dynasty/children/{user} with {permission: "BFR", status: true}
	// Updates a single permission flag
	// The grpc-gateway sends a ChildPermissions object with only one permission set
	locale := getLocale(ctx)
	if req.Permissions == nil {
		validationErrors := validateRequired("permissions", nil, locale)
		return nil, returnValidationError(validationErrors)
	}

	// Determine which permission is being set and its status
	var permission string
	var statusVal bool
	found := false

	// Check each permission flag to find the one being set
	// The gateway sets only one permission at a time
	if req.Permissions.BFR {
		permission = "BFR"
		statusVal = req.Permissions.BFR
		found = true
	} else if req.Permissions.SF {
		permission = "SF"
		statusVal = req.Permissions.SF
		found = true
	} else if req.Permissions.W {
		permission = "W"
		statusVal = req.Permissions.W
		found = true
	} else if req.Permissions.JU {
		permission = "JU"
		statusVal = req.Permissions.JU
		found = true
	} else if req.Permissions.DM {
		permission = "DM"
		statusVal = req.Permissions.DM
		found = true
	} else if req.Permissions.PIUP {
		permission = "PIUP"
		statusVal = req.Permissions.PIUP
		found = true
	} else if req.Permissions.PITC {
		permission = "PITC"
		statusVal = req.Permissions.PITC
		found = true
	} else if req.Permissions.PIC {
		permission = "PIC"
		statusVal = req.Permissions.PIC
		found = true
	} else if req.Permissions.ESOO {
		permission = "ESOO"
		statusVal = req.Permissions.ESOO
		found = true
	} else if req.Permissions.COTB {
		permission = "COTB"
		statusVal = req.Permissions.COTB
		found = true
	}

	if !found {
		validationErrors := make(map[string]string)
		t := helpers.GetLocaleTranslations(locale)
		validationErrors["permissions"] = fmt.Sprintf(t.Required, "permissions")
		return nil, returnValidationError(validationErrors)
	}

	err := h.permissionService.UpdateChildPermission(ctx, req.ParentUserId, req.ChildUserId, permission, statusVal)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &commonpb.Empty{}, nil
}
