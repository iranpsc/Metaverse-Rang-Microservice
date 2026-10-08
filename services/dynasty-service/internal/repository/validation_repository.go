package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type ValidationRepository struct {
	db *sql.DB
}

func NewValidationRepository(db *sql.DB) *ValidationRepository {
	return &ValidationRepository{db: db}
}

// CheckPendingRequest checks if there's a pending join request between two users
func (r *ValidationRepository) CheckPendingRequest(ctx context.Context, fromUser, toUser uint64) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM join_requests 
		WHERE from_user = ? AND to_user = ? AND status = 0
	)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, fromUser, toUser).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check pending request: %w", err)
	}

	return exists, nil
}

// CheckRejectedRequest checks if there's a rejected join request between two users
func (r *ValidationRepository) CheckRejectedRequest(ctx context.Context, fromUser, toUser uint64) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM join_requests 
		WHERE from_user = ? AND to_user = ? AND status = -1
	)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, fromUser, toUser).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check rejected request: %w", err)
	}

	return exists, nil
}

// CheckUserInFamily checks if a user is already in a family (as non-owner)
func (r *ValidationRepository) CheckUserInFamily(ctx context.Context, userID uint64) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM family_members 
		WHERE user_id = ? AND relationship != 'owner'
	)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check user in family: %w", err)
	}

	return exists, nil
}

// CountFamilyMembers counts every member of a family, including the owner.
func (r *ValidationRepository) CountFamilyMembers(ctx context.Context, familyID uint64) (int, error) {
	query := `SELECT COUNT(*) FROM family_members WHERE family_id = ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, familyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count family members: %w", err)
	}
	return count, nil
}

// CountSiblingMembers counts brothers and sisters together.
func (r *ValidationRepository) CountSiblingMembers(ctx context.Context, familyID uint64) (int, error) {
	query := `SELECT COUNT(*) FROM family_members WHERE family_id = ? AND relationship IN ('brother', 'sister')`

	var count int
	err := r.db.QueryRowContext(ctx, query, familyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sibling members: %w", err)
	}
	return count, nil
}

// CheckReceiverVerified reports KYC approval and the user code used in denial messages.
func (r *ValidationRepository) CheckReceiverVerified(ctx context.Context, userID uint64) (bool, string, error) {
	query := `
		SELECT u.code, COALESCE(k.status, 0) = 1
		FROM users u
		LEFT JOIN kycs k ON k.user_id = u.id
		WHERE u.id = ?
		LIMIT 1
	`
	var code string
	var verified bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&code, &verified)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to check receiver verification: %w", err)
	}
	return verified, code, nil
}

// CountFamilyMembersByRelationship counts family members with specific relationship
func (r *ValidationRepository) CountFamilyMembersByRelationship(ctx context.Context, familyID uint64, relationship string) (int, error) {
	query := `SELECT COUNT(*) FROM family_members 
	          WHERE family_id = ? AND relationship = ?`

	var count int
	err := r.db.QueryRowContext(ctx, query, familyID, relationship).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count family members: %w", err)
	}

	return count, nil
}

// GetFamilyByDynastyID gets family for a dynasty
func (r *ValidationRepository) GetFamilyByDynastyID(ctx context.Context, dynastyID uint64) (uint64, error) {
	query := `SELECT id FROM families WHERE dynasty_id = ?`

	var familyID uint64
	err := r.db.QueryRowContext(ctx, query, dynastyID).Scan(&familyID)
	if err != nil {
		return 0, fmt.Errorf("failed to get family: %w", err)
	}

	return familyID, nil
}

// CheckUserHasDynasty checks if user has a dynasty
func (r *ValidationRepository) CheckUserHasDynasty(ctx context.Context, userID uint64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM dynasties WHERE user_id = ?)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check dynasty: %w", err)
	}

	return exists, nil
}

// CheckUserDMPermission matches UserPolicy::addFamilyMember for under-18 senders.
// No children_permissions row is allowed. A row is allowed when verified OR DM is set.
func (r *ValidationRepository) CheckUserDMPermission(ctx context.Context, userID uint64) (bool, error) {
	query := `SELECT verified, DM FROM children_permissions WHERE user_id = ?`

	var verified, dm bool
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&verified, &dm)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check DM permission: %w", err)
	}

	return verified || dm, nil
}
