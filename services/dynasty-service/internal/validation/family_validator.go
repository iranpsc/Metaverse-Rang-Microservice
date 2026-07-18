// Package validation provides request validation for the dynasty service.
package validation

import (
	"context"
	"fmt"

	"metarang/dynasty-service/internal/repository"
)

type FamilyValidator struct {
	validationRepo *repository.ValidationRepository
}

func NewFamilyValidator(validationRepo *repository.ValidationRepository) *FamilyValidator {
	return &FamilyValidator{validationRepo: validationRepo}
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
	Code    int
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateAddFamilyMember validates all rules for adding a family member
func (v *FamilyValidator) ValidateAddFamilyMember(ctx context.Context, senderID, receiverID uint64, relationship string, isUnder18 bool) error {
	// RULE 1: Under-18 sender needs verified DM permission
	if isUnder18 {
		hasPermission, err := v.validationRepo.CheckUserDMPermission(ctx, senderID)
		if err != nil {
			return fmt.Errorf("failed to check permission: %w", err)
		}
		if !hasPermission {
			return &ValidationError{
				Message: "شما مجاز به مدیریت سلسله نیستید",
				Code:    403,
			}
		}
	}

	// RULE 2: Cannot add self
	if senderID == receiverID {
		return &ValidationError{
			Message: "شما نمی توانید خودتان را درخواست داشته باشید.",
			Code:    403,
		}
	}

	// RULE 3: Sender must have dynasty
	hasDynasty, err := v.validationRepo.CheckUserHasDynasty(ctx, senderID)
	if err != nil {
		return fmt.Errorf("failed to check dynasty: %w", err)
	}
	if !hasDynasty {
		return &ValidationError{
			Message: "شما هیچ سلسله ای تاسیس نکرده اید.",
			Code:    403,
		}
	}

	// RULE 4: No duplicate pending requests
	hasPending, err := v.validationRepo.CheckPendingRequest(ctx, senderID, receiverID)
	if err != nil {
		return fmt.Errorf("failed to check pending request: %w", err)
	}
	if hasPending {
		return &ValidationError{
			Message: "شما قبلا درخواست خود را به این کاربر ارسال کرده اید.",
			Code:    403,
		}
	}

	// RULE 5: No previously rejected requests
	wasRejected, err := v.validationRepo.CheckRejectedRequest(ctx, senderID, receiverID)
	if err != nil {
		return fmt.Errorf("failed to check rejected request: %w", err)
	}
	if wasRejected {
		return &ValidationError{
			Message: "درخواست شما قبلا توسط این کاربر رد شده است.",
			Code:    403,
		}
	}

	// RULE 6: Receiver not already in another family
	inFamily, err := v.validationRepo.CheckUserInFamily(ctx, receiverID)
	if err != nil {
		return fmt.Errorf("failed to check user in family: %w", err)
	}
	if inFamily {
		return &ValidationError{
			Message: "این کاربر قبلا به سلسله شما اضافه شده است.",
			Code:    403,
		}
	}

	// RULE 7 & 8: Relationship-specific limits
	// Get sender's dynasty and family
	// This will be checked in the service layer with actual family ID

	return nil
}

// ValidateRelationshipLimits validates relationship-specific member limits
func (v *FamilyValidator) ValidateRelationshipLimits(ctx context.Context, familyID uint64, relationship string) error {
	// Define limits for each relationship type
	limits := map[string]struct {
		Max     int
		Message string
	}{
		"father": {
			Max:     1,
			Message: "شما فقط می توانید یک پدر داشته باشید.",
		},
		"mother": {
			Max:     1,
			Message: "شما فقط می توانید یک مادر داشته باشید.",
		},
		"husband": {
			Max:     1,
			Message: "شما فقط می توانید یک همسر داشته باشید.",
		},
		"wife": {
			Max:     4,
			Message: "شما فقط می توانید چهار همسر داشته باشید.",
		},
		"offspring": {
			Max:     4,
			Message: "شما قبلا بیش از 4 عضو در سلسله خود دارید.",
		},
	}

	// Check if relationship has a limit
	limit, hasLimit := limits[relationship]
	if !hasLimit {
		// No limit for this relationship (e.g., brother, sister)
		return nil
	}

	// Count existing members with this relationship
	count, err := v.validationRepo.CountFamilyMembersByRelationship(ctx, familyID, relationship)
	if err != nil {
		return fmt.Errorf("failed to count family members: %w", err)
	}

	// Check if limit exceeded
	if count >= limit.Max {
		return &ValidationError{
			Message: limit.Message,
			Code:    403,
		}
	}

	return nil
}

// ValidateRelationship validates if relationship is valid
func (v *FamilyValidator) ValidateRelationship(relationship string) error {
	validRelationships := map[string]bool{
		"brother":   true,
		"sister":    true,
		"father":    true,
		"mother":    true,
		"husband":   true,
		"wife":      true,
		"offspring": true,
	}

	if !validRelationships[relationship] {
		return &ValidationError{
			Message: "نوع رابطه نامعتبر است.",
			Code:    400,
		}
	}

	return nil
}
