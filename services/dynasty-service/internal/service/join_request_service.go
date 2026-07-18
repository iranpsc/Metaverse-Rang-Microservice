package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"metarang/dynasty-service/internal/models"
	"metarang/dynasty-service/internal/repository"
)

type NotificationPort interface {
	SendNotification(ctx context.Context, userID uint64, notificationType, title, message string, data map[string]string, sendSMS, sendEmail bool) error
}

type JoinRequestService struct {
	joinRequestRepo         *repository.JoinRequestRepository
	dynastyRepo             *repository.DynastyRepository
	familyRepo              *repository.FamilyRepository
	prizeRepo               *repository.PrizeRepository
	notificationClient      NotificationPort
	notificationServiceAddr string
}

func NewJoinRequestService(
	joinRequestRepo *repository.JoinRequestRepository,
	dynastyRepo *repository.DynastyRepository,
	familyRepo *repository.FamilyRepository,
	prizeRepo *repository.PrizeRepository,
	notificationClient NotificationPort,
	notificationServiceAddr string,
) *JoinRequestService {
	return &JoinRequestService{
		joinRequestRepo:         joinRequestRepo,
		dynastyRepo:             dynastyRepo,
		familyRepo:              familyRepo,
		prizeRepo:               prizeRepo,
		notificationClient:      notificationClient,
		notificationServiceAddr: notificationServiceAddr,
	}
}

// SendJoinRequest creates and sends a join request
func (s *JoinRequestService) SendJoinRequest(ctx context.Context, fromUserID, toUserID uint64, relationship string, message *string, permissions *models.ChildPermission) (*models.JoinRequest, error) {
	// Validate relationship is offering (not spring) and user age for permissions
	if relationship == "offspring" && permissions != nil {
		isUnder18, err := s.joinRequestRepo.CheckUserAge(ctx, toUserID)
		if err != nil {
			return nil, fmt.Errorf("failed to check user age: %w", err)
		}
		if !isUnder18 {
			return nil, fmt.Errorf("cannot set permissions for offspring over 18")
		}
	}

	// Get dynasty message for receiver
	messageTemplate, err := s.dynastyRepo.GetDynastyMessage(ctx, "receiver_message")
	if err != nil {
		return nil, fmt.Errorf("failed to get dynasty message: %w", err)
	}

	// Create join request
	joinRequest := &models.JoinRequest{
		FromUser:     fromUserID,
		ToUser:       toUserID,
		Status:       0, // 0=pending (per API spec)
		Relationship: relationship,
		Message:      message,
	}

	if err := s.joinRequestRepo.CreateJoinRequest(ctx, joinRequest); err != nil {
		return nil, fmt.Errorf("failed to create join request: %w", err)
	}

	// If permissions provided for offspring, store them
	if relationship == "offspring" && permissions != nil {
		permissions.UserID = toUserID
		if err := s.joinRequestRepo.CreateChildPermission(ctx, permissions); err != nil {
			return nil, fmt.Errorf("failed to create child permissions: %w", err)
		}
	}

	// Send notifications (best effort, non-blocking for main flow)
	s.notifyJoinRequestCreated(ctx, fromUserID, toUserID, relationship, messageTemplate)

	return joinRequest, nil
}

// GetSentRequests retrieves sent join requests for a user
func (s *JoinRequestService) GetSentRequests(ctx context.Context, userID uint64, page, perPage int32) ([]*models.JoinRequest, int32, error) {
	return s.joinRequestRepo.GetSentRequests(ctx, userID, page, perPage)
}

// GetReceivedRequests retrieves received join requests for a user
func (s *JoinRequestService) GetReceivedRequests(ctx context.Context, userID uint64, page, perPage int32) ([]*models.JoinRequest, int32, error) {
	return s.joinRequestRepo.GetReceivedRequests(ctx, userID, page, perPage)
}

// GetJoinRequest retrieves a specific join request
func (s *JoinRequestService) GetJoinRequest(ctx context.Context, requestID, userID uint64) (*models.JoinRequest, error) {
	request, err := s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get join request: %w", err)
	}
	if request == nil {
		return nil, fmt.Errorf("join request not found")
	}

	// Authorize: user must be sender or receiver
	if request.FromUser != userID && request.ToUser != userID {
		return nil, fmt.Errorf("unauthorized to view this request")
	}

	return request, nil
}

// AcceptJoinRequest accepts a join request and adds member to family
func (s *JoinRequestService) AcceptJoinRequest(ctx context.Context, requestID, userID uint64) error {
	// Get join request
	request, err := s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get join request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("join request not found")
	}

	// Authorize: only receiver can accept
	if request.ToUser != userID {
		return fmt.Errorf("unauthorized to accept this request")
	}

	// Update request status to accepted
	if err := s.joinRequestRepo.UpdateJoinRequestStatus(ctx, requestID, 1); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	// Get requester's dynasty
	requestedUser := request.FromUser
	dynasty, err := s.dynastyRepo.GetDynastyByUserID(ctx, requestedUser)
	if err != nil {
		return fmt.Errorf("failed to get dynasty: %w", err)
	}
	if dynasty == nil {
		return fmt.Errorf("requester does not have a dynasty")
	}

	// Get family
	family, err := s.familyRepo.GetFamilyByDynastyID(ctx, dynasty.ID)
	if err != nil {
		return fmt.Errorf("failed to get family: %w", err)
	}

	// Add user to family
	member := &models.FamilyMember{
		FamilyID:     family.ID,
		UserID:       userID,
		Relationship: request.Relationship,
	}
	if err := s.familyRepo.CreateFamilyMember(ctx, member); err != nil {
		return fmt.Errorf("failed to add family member: %w", err)
	}

	// Handle permissions for under-18 users
	requestedUserUnder18, _ := s.joinRequestRepo.CheckUserAge(ctx, requestedUser)
	receiverUserUnder18, _ := s.joinRequestRepo.CheckUserAge(ctx, userID)

	if requestedUserUnder18 && request.Relationship == "father" {
		// Give default dynasty permissions to requesting user
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
		// Verify existing permissions
		existingPerm, _ := s.joinRequestRepo.GetChildPermission(ctx, userID)
		if existingPerm != nil {
			existingPerm.Verified = true
			_ = s.joinRequestRepo.UpdateChildPermission(ctx, userID, existingPerm)
		}
	}

	// Award prize based on relationship
	if s.prizeRepo != nil {
		prize, err := s.prizeRepo.GetPrizeByRelationship(ctx, request.Relationship)
		if err == nil && prize != nil {
			// Create message for prize
			message := fmt.Sprintf("پاداش اضافه شدن به سلسله به عنوان %s", request.Relationship)
			// Award prize to the user who accepted (the new family member)
			if err := s.prizeRepo.AwardPrize(ctx, userID, prize.ID, message); err != nil {
				// Log error but don't fail the entire operation
				fmt.Printf("Warning: failed to award prize: %v\n", err)
			}
		}
	}

	// Send accept notifications (best effort)
	s.notifyJoinRequestAccepted(ctx, request.FromUser, request.ToUser, request.Relationship)

	return nil
}

// RejectJoinRequest rejects a join request
func (s *JoinRequestService) RejectJoinRequest(ctx context.Context, requestID, userID uint64) error {
	// Get join request
	request, err := s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get join request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("join request not found")
	}

	// Authorize: only receiver can reject
	if request.ToUser != userID {
		return fmt.Errorf("unauthorized to reject this request")
	}

	// Update request status to rejected (-1 per API spec)
	if err := s.joinRequestRepo.UpdateJoinRequestStatus(ctx, requestID, -1); err != nil {
		return fmt.Errorf("failed to update request status: %w", err)
	}

	// Send reject notifications (best effort)
	s.notifyJoinRequestRejected(ctx, request.FromUser, request.ToUser)

	return nil
}

// DeleteJoinRequest deletes a join request
func (s *JoinRequestService) DeleteJoinRequest(ctx context.Context, requestID, userID uint64) error {
	// Get join request
	request, err := s.joinRequestRepo.GetJoinRequestByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("failed to get join request: %w", err)
	}
	if request == nil {
		return fmt.Errorf("join request not found")
	}

	// Authorize: only sender can delete
	if request.FromUser != userID {
		return fmt.Errorf("unauthorized to delete this request")
	}

	// Delete request
	if err := s.joinRequestRepo.DeleteJoinRequest(ctx, requestID); err != nil {
		return fmt.Errorf("failed to delete request: %w", err)
	}

	return nil
}

// GetUserBasicInfo retrieves basic user information
func (s *JoinRequestService) GetUserBasicInfo(ctx context.Context, userID uint64) (*models.UserBasic, error) {
	return s.joinRequestRepo.GetUserBasicInfo(ctx, userID)
}

// FormatRelationshipMessage formats dynasty messages with placeholders
func (s *JoinRequestService) FormatRelationshipMessage(message string, senderName, receiverName, relationship, date string) string {
	message = strings.ReplaceAll(message, "{sender}", senderName)
	message = strings.ReplaceAll(message, "{receiver}", receiverName)
	message = strings.ReplaceAll(message, "{relationship}", relationship)
	message = strings.ReplaceAll(message, "{date}", date)
	return message
}

func (s *JoinRequestService) fillDynastyTemplate(template string, sender, receiver *models.UserBasic, relationship string, when time.Time) string {
	if template == "" {
		return ""
	}
	relationshipTitle := s.getRelationshipTitle(relationship)
	result := template
	result = strings.ReplaceAll(result, "[sender-code]", sender.Code)
	result = strings.ReplaceAll(result, "[reciever-code]", receiver.Code)
	result = strings.ReplaceAll(result, "[sender-name]", sender.Name)
	result = strings.ReplaceAll(result, "[reciever-name]", receiver.Name)
	result = strings.ReplaceAll(result, "[relationship]", relationshipTitle)
	result = strings.ReplaceAll(result, "[created_at]", when.Format("2006/01/02"))
	return result
}

func (s *JoinRequestService) getRelationshipTitle(relationship string) string {
	titles := map[string]string{
		"brother":   "برادر",
		"sister":    "خواهر",
		"offspring": "فرزند",
		"father":    "پدر",
		"mother":    "مادر",
		"husband":   "شوهر",
		"wife":      "زن",
		"owner":     "مالک",
	}
	if t, ok := titles[relationship]; ok {
		return t
	}
	return relationship
}

func (s *JoinRequestService) notifyJoinRequestCreated(ctx context.Context, senderID, receiverID uint64, relationship, receiverTemplate string) {
	if s.notificationClient == nil {
		return
	}
	senderInfo, err := s.joinRequestRepo.GetUserBasicInfo(ctx, senderID)
	if err != nil || senderInfo == nil {
		return
	}
	receiverInfo, err := s.joinRequestRepo.GetUserBasicInfo(ctx, receiverID)
	if err != nil || receiverInfo == nil {
		return
	}
	requesterTemplate, _ := s.dynastyRepo.GetDynastyMessage(ctx, "requester_confirmation_message")
	now := time.Now()
	senderMsg := s.fillDynastyTemplate(requesterTemplate, senderInfo, receiverInfo, relationship, now)
	receiverMsg := s.fillDynastyTemplate(receiverTemplate, senderInfo, receiverInfo, relationship, now)
	if senderMsg != "" {
		_ = s.notificationClient.SendNotification(ctx, senderID, "dynasty_join_request", "Dynasty", senderMsg, map[string]string{"relationship": relationship}, false, false)
	}
	if receiverMsg != "" {
		_ = s.notificationClient.SendNotification(ctx, receiverID, "dynasty_join_request", "Dynasty", receiverMsg, map[string]string{"relationship": relationship}, false, false)
	}
}

func (s *JoinRequestService) notifyJoinRequestAccepted(ctx context.Context, requesterID, receiverID uint64, relationship string) {
	if s.notificationClient == nil {
		return
	}
	requesterInfo, err := s.joinRequestRepo.GetUserBasicInfo(ctx, requesterID)
	if err != nil || requesterInfo == nil {
		return
	}
	receiverInfo, err := s.joinRequestRepo.GetUserBasicInfo(ctx, receiverID)
	if err != nil || receiverInfo == nil {
		return
	}
	requesterTemplate, _ := s.dynastyRepo.GetDynastyMessage(ctx, "requester_accept_message")
	receiverTemplate, _ := s.dynastyRepo.GetDynastyMessage(ctx, "reciever_accept_message")
	now := time.Now()
	requesterMsg := s.fillDynastyTemplate(requesterTemplate, requesterInfo, receiverInfo, relationship, now)
	receiverMsg := s.fillDynastyTemplate(receiverTemplate, requesterInfo, receiverInfo, relationship, now)
	if requesterMsg != "" {
		_ = s.notificationClient.SendNotification(ctx, requesterID, "dynasty_join_request_accept", "Dynasty", requesterMsg, map[string]string{"relationship": relationship}, false, false)
	}
	if receiverMsg != "" {
		_ = s.notificationClient.SendNotification(ctx, receiverID, "dynasty_join_request_accept", "Dynasty", receiverMsg, map[string]string{"relationship": relationship}, false, false)
	}
}

func (s *JoinRequestService) notifyJoinRequestRejected(ctx context.Context, requesterID, receiverID uint64) {
	if s.notificationClient == nil {
		return
	}
	requesterInfo, err := s.joinRequestRepo.GetUserBasicInfo(ctx, requesterID)
	if err != nil || requesterInfo == nil {
		return
	}
	receiverInfo, err := s.joinRequestRepo.GetUserBasicInfo(ctx, receiverID)
	if err != nil || receiverInfo == nil {
		return
	}
	requesterTemplate, _ := s.dynastyRepo.GetDynastyMessage(ctx, "requester_reject_message")
	receiverTemplate, _ := s.dynastyRepo.GetDynastyMessage(ctx, "reciever_reject_message")
	if requesterTemplate == "" {
		requesterTemplate = "درخواست پیوستن به سلسله شما توسط کاربر [reciever-code] رد شد!"
	}
	if receiverTemplate == "" {
		receiverTemplate = "درخواست پیوستن به سلسله از طرف [sender-code] توسط شما رد شد."
	}
	now := time.Now()
	requesterMsg := s.fillDynastyTemplate(requesterTemplate, requesterInfo, receiverInfo, "", now)
	receiverMsg := s.fillDynastyTemplate(receiverTemplate, requesterInfo, receiverInfo, "", now)
	if requesterMsg != "" {
		_ = s.notificationClient.SendNotification(ctx, requesterID, "dynasty_join_request_reject", "Dynasty", requesterMsg, nil, false, false)
	}
	if receiverMsg != "" {
		_ = s.notificationClient.SendNotification(ctx, receiverID, "dynasty_join_request_reject", "Dynasty", receiverMsg, nil, false, false)
	}
}

// GetPrizeByRelationship retrieves dynasty prize by relationship type
func (s *JoinRequestService) GetPrizeByRelationship(ctx context.Context, relationship string) (*models.DynastyPrize, error) {
	if s.prizeRepo == nil {
		return nil, nil
	}
	return s.prizeRepo.GetPrizeByRelationship(ctx, relationship)
}

// GetDefaultPermissions retrieves default dynasty permissions for offspring
func (s *JoinRequestService) GetDefaultPermissions(ctx context.Context) (*models.DynastyPermission, error) {
	return s.joinRequestRepo.GetDynastyPermission(ctx)
}
