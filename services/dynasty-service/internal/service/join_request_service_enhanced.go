package service

import (
	"context"
	"fmt"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/validation"
	"metarang/shared/pkg/helpers"
)

// JoinRequestServiceEnhanced provides enhanced join request operations with full validation
type JoinRequestServiceEnhanced struct {
	joinRequestRepo *repository.JoinRequestRepository
	dynastyRepo     *repository.DynastyRepository
	familyRepo      *repository.FamilyRepository
	validationRepo  *repository.ValidationRepository
	messageRepo     *repository.MessageRepository
	prizeRepo       *repository.PrizeRepository
	validator       *validation.FamilyValidator

	// gRPC clients (to be injected)
	// authClient         auth.AuthServiceClient
	// notificationClient notification.NotificationServiceClient
}

func NewJoinRequestServiceEnhanced(
	joinRequestRepo *repository.JoinRequestRepository,
	dynastyRepo *repository.DynastyRepository,
	familyRepo *repository.FamilyRepository,
	validationRepo *repository.ValidationRepository,
	messageRepo *repository.MessageRepository,
	prizeRepo *repository.PrizeRepository,
) *JoinRequestServiceEnhanced {
	validator := validation.NewFamilyValidator(validationRepo)

	return &JoinRequestServiceEnhanced{
		joinRequestRepo: joinRequestRepo,
		dynastyRepo:     dynastyRepo,
		familyRepo:      familyRepo,
		validationRepo:  validationRepo,
		messageRepo:     messageRepo,
		prizeRepo:       prizeRepo,
		validator:       validator,
	}
}

// SendJoinRequest creates and sends a join request with full validation
func (s *JoinRequestServiceEnhanced) SendJoinRequest(
	ctx context.Context,
	fromUserID, toUserID uint64,
	relationship string,
	permissions *models.ChildPermission,
) (*models.JoinRequest, string, string, error) {

	// Validate relationship type
	if err := s.validator.ValidateRelationship(relationship); err != nil {
		return nil, "", "", err
	}

	// Check if sender is under 18 (would call Auth service)
	isUnder18, err := s.joinRequestRepo.CheckUserAge(ctx, fromUserID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to check user age: %w", err)
	}

	// Validate permissions for offspring relationship
	if relationship == "offspring" && permissions != nil {
		toUserUnder18, err := s.joinRequestRepo.CheckUserAge(ctx, toUserID)
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to check receiver age: %w", err)
		}
		if !toUserUnder18 {
			return nil, "", "", fmt.Errorf("شما مجاز به تعریف دسترسی برای فرزند بالای 18 سال نیستید")
		}
	}

	// Run all validation rules
	if err := s.validator.ValidateAddFamilyMember(ctx, fromUserID, toUserID, relationship, isUnder18); err != nil {
		return nil, "", "", err
	}

	// Get sender's dynasty and family to check relationship limits
	dynasty, err := s.dynastyRepo.GetDynastyByUserID(ctx, fromUserID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get dynasty: %w", err)
	}

	familyID, err := s.validationRepo.GetFamilyByDynastyID(ctx, dynasty.ID)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get family: %w", err)
	}

	// Validate relationship limits
	if err := s.validator.ValidateRelationshipLimits(ctx, familyID, relationship); err != nil {
		return nil, "", "", err
	}

	// Get user details (would call Auth service in real implementation)
	// For now, we'll use placeholder data
	senderCode := fmt.Sprintf("USER-%d", fromUserID)
	receiverCode := fmt.Sprintf("USER-%d", toUserID)
	senderName := "نام کاربر"
	receiverName := "نام گیرنده"

	// Prepare messages with templates
	jalaliDate := helpers.NowJalali()
	senderMsg, receiverMsg, err := s.messageRepo.PrepareJoinRequestMessages(
		ctx,
		senderCode,
		receiverCode,
		senderName,
		receiverName,
		relationship,
		jalaliDate,
	)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to prepare messages: %w", err)
	}

	// Create join request with CORRECT status value (0 = pending)
	joinRequest := &models.JoinRequest{
		FromUser:     fromUserID,
		ToUser:       toUserID,
		Status:       0, // CORRECT: 0 = pending (not 4!)
		Relationship: relationship,
		Message:      &receiverMsg,
	}

	if err := s.joinRequestRepo.CreateJoinRequest(ctx, joinRequest); err != nil {
		return nil, "", "", fmt.Errorf("failed to create join request: %w", err)
	}

	// If permissions provided for offspring, store them
	if relationship == "offspring" && permissions != nil {
		permissions.UserID = toUserID
		permissions.Verified = false // Not verified until accepted
		if err := s.joinRequestRepo.CreateChildPermission(ctx, permissions); err != nil {
			return nil, "", "", fmt.Errorf("failed to create child permissions: %w", err)
		}
	}

	// TODO: Send notifications via gRPC to notification service
	// s.notificationClient.SendNotification(ctx, &notification.SendRequest{
	//     UserId: fromUserID,
	//     Type: "JoinDynastyNotification",
	//     Data: map[string]string{
	//         "type": "requester_confirmation_message",
	//         "message": senderMsg,
	//     },
	// })
	// Similar for receiver...

	return joinRequest, senderMsg, receiverMsg, nil
}

// AcceptJoinRequest accepts a join request with full business logic
func (s *JoinRequestServiceEnhanced) AcceptJoinRequest(
	ctx context.Context,
	requestID, userID uint64,
) (*models.JoinRequest, error) {

	// Get join request
	request, err := s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get join request: %w", err)
	}
	if request == nil {
		return nil, fmt.Errorf("join request not found")
	}

	// Authorize: only receiver can accept, status must be pending
	if request.ToUser != userID {
		return nil, fmt.Errorf("unauthorized to accept this request")
	}
	if request.Status != 0 { // Must be pending
		return nil, fmt.Errorf("request is not pending")
	}

	// Update request status to accepted (1 = accepted, NOT 2!)
	if err := s.joinRequestRepo.UpdateJoinRequestStatus(ctx, requestID, 1); err != nil {
		return nil, fmt.Errorf("failed to update request status: %w", err)
	}

	// Get requester's dynasty
	requestedUser := request.FromUser
	dynasty, err := s.dynastyRepo.GetDynastyByUserID(ctx, requestedUser)
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty: %w", err)
	}
	if dynasty == nil {
		return nil, fmt.Errorf("requester does not have a dynasty")
	}

	// Get family
	family, err := s.familyRepo.GetFamilyByDynastyID(ctx, dynasty.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get family: %w", err)
	}

	// Add user to family
	member := &models.FamilyMember{
		FamilyID:     family.ID,
		UserID:       userID,
		Relationship: request.Relationship,
	}
	if err := s.familyRepo.CreateFamilyMember(ctx, member); err != nil {
		return nil, fmt.Errorf("failed to add family member: %w", err)
	}

	// COMPLEX PERMISSION LOGIC
	requestedUserUnder18, _ := s.joinRequestRepo.CheckUserAge(ctx, requestedUser)
	receiverUserUnder18, _ := s.joinRequestRepo.CheckUserAge(ctx, userID)

	// If requested user is under 18 and relationship is 'father'
	// Give the user full default dynasty permissions
	if requestedUserUnder18 && request.Relationship == "father" {
		defaultPerms, err := s.joinRequestRepo.GetDynastyPermission(ctx)
		if err == nil && defaultPerms != nil {
			childPerm := &models.ChildPermission{
				UserID:   requestedUser,
				Verified: true,
				BFR:      defaultPerms.BFR,
				SF:       defaultPerms.SF,
				W:        defaultPerms.W,
				JU:       defaultPerms.JU,
				DM:       defaultPerms.DM,
				PIUP:     defaultPerms.PIUP,
				PITC:     defaultPerms.PITC,
				PIC:      defaultPerms.PIC,
				ESOO:     defaultPerms.ESOO,
				COTB:     defaultPerms.COTB,
			}
			_ = s.joinRequestRepo.CreateChildPermission(ctx, childPerm)
		}
	} else if receiverUserUnder18 && request.Relationship == "offspring" {
		// If receiver is under 18 and relationship is 'offspring'
		// Verify existing permissions
		existingPerm, _ := s.joinRequestRepo.GetChildPermission(ctx, userID)
		if existingPerm != nil {
			existingPerm.Verified = true
			_ = s.joinRequestRepo.UpdateChildPermission(ctx, userID, existingPerm)
		}
	}

	// AWARD PRIZE based on relationship
	// Get prize for this relationship
	prize, err := s.prizeRepo.GetPrizeByRelationship(ctx, request.Relationship)
	if err == nil && prize != nil {
		// Prepare accept messages
		jalaliDate := helpers.FormatJalaliDate(request.CreatedAt)
		requesterMsg, _, err := s.messageRepo.PrepareAcceptMessages(
			ctx,
			fmt.Sprintf("USER-%d", requestedUser),
			fmt.Sprintf("USER-%d", userID),
			"نام درخواست کننده",
			"نام پذیرنده",
			request.Relationship,
			jalaliDate,
		)
		if err == nil {
			// Award prize to requester
			_ = s.prizeRepo.AwardPrize(ctx, requestedUser, prize.ID, requesterMsg)
		}
	}

	// TODO: Send notifications via gRPC
	// TODO: Prepare and send accept messages to both parties

	// Refresh and return request
	request, _ = s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	return request, nil
}

// RejectJoinRequest rejects a join request
func (s *JoinRequestServiceEnhanced) RejectJoinRequest(
	ctx context.Context,
	requestID, userID uint64,
) error {

	// Get join request
	request, err := s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get join request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("join request not found")
	}

	// Authorize: only receiver can reject, status must be pending
	if request.ToUser != userID {
		return fmt.Errorf("unauthorized to reject this request")
	}
	if request.Status != 0 { // Must be pending
		return fmt.Errorf("request is not pending")
	}

	// Update request status to rejected (-1 = rejected, NOT 2!)
	if err := s.joinRequestRepo.UpdateJoinRequestStatus(ctx, requestID, -1); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	// Prepare reject messages
	requesterCode := fmt.Sprintf("USER-%d", request.FromUser)
	receiverCode := fmt.Sprintf("USER-%d", userID)
	requesterMsg, receiverMsg, _ := s.messageRepo.PrepareRejectMessages(ctx, requesterCode, receiverCode)

	// TODO: Send notifications via gRPC
	_ = requesterMsg
	_ = receiverMsg

	return nil
}
