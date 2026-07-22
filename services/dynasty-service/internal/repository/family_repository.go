package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/dynasty-service/internal/models"
)

type FamilyRepository struct {
	db *sql.DB
}

func NewFamilyRepository(db *sql.DB) *FamilyRepository {
	return &FamilyRepository{db: db}
}

// CreateFamily creates a new family
func (r *FamilyRepository) CreateFamily(ctx context.Context, dynastyID uint64) (*models.Family, error) {
	query := `INSERT INTO families (dynasty_id, created_at, updated_at) 
	          VALUES (?, NOW(), NOW())`

	result, err := r.db.ExecContext(ctx, query, dynastyID)
	if err != nil {
		return nil, fmt.Errorf("failed to create family: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get family ID: %w", err)
	}

	return &models.Family{
		ID:        uint64(id),
		DynastyID: dynastyID,
	}, nil
}

func scanFamily(scanner interface {
	Scan(dest ...any) error
}) (*models.Family, error) {
	var family models.Family
	var createdAt, updatedAt sql.NullTime

	err := scanner.Scan(
		&family.ID,
		&family.DynastyID,
		&createdAt,
		&updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get family: %w", err)
	}

	if createdAt.Valid {
		family.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		family.UpdatedAt = updatedAt.Time
	}

	return &family, nil
}

// GetFamilyByID retrieves a family by ID
func (r *FamilyRepository) GetFamilyByID(ctx context.Context, id uint64) (*models.Family, error) {
	query := `SELECT id, dynasty_id, created_at, updated_at 
	          FROM families WHERE id = ?`

	return scanFamily(r.db.QueryRowContext(ctx, query, id))
}

// GetFamilyByDynastyID retrieves a family by dynasty ID
func (r *FamilyRepository) GetFamilyByDynastyID(ctx context.Context, dynastyID uint64) (*models.Family, error) {
	query := `SELECT id, dynasty_id, created_at, updated_at 
	          FROM families WHERE dynasty_id = ?`

	return scanFamily(r.db.QueryRowContext(ctx, query, dynastyID))
}

// CreateFamilyMember creates a new family member
func (r *FamilyRepository) CreateFamilyMember(ctx context.Context, member *models.FamilyMember) error {
	query := `INSERT INTO family_members (family_id, user_id, relationship, created_at, updated_at) 
	          VALUES (?, ?, ?, NOW(), NOW())`

	result, err := r.db.ExecContext(ctx, query, member.FamilyID, member.UserID, member.Relationship)
	if err != nil {
		return fmt.Errorf("failed to create family member: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get family member ID: %w", err)
	}

	member.ID = uint64(id)
	return nil
}

// GetFamilyMembers retrieves all members of a family
func (r *FamilyRepository) GetFamilyMembers(ctx context.Context, familyID uint64, page, perPage int32) ([]*models.FamilyMember, int32, error) {
	offset := (page - 1) * perPage

	// Get total count
	countQuery := `SELECT COUNT(*) FROM family_members WHERE family_id = ?`
	var total int32
	err := r.db.QueryRowContext(ctx, countQuery, familyID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count family members: %w", err)
	}

	// Get members
	query := `SELECT id, family_id, user_id, relationship, created_at, updated_at 
	          FROM family_members 
	          WHERE family_id = ? 
	          ORDER BY created_at ASC 
	          LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, familyID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get family members: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var members []*models.FamilyMember
	for rows.Next() {
		var member models.FamilyMember
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&member.ID,
			&member.FamilyID,
			&member.UserID,
			&member.Relationship,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan family member: %w", err)
		}
		if createdAt.Valid {
			member.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			member.UpdatedAt = updatedAt.Time
		}
		members = append(members, &member)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("failed to iterate family members: %w", err)
	}

	return members, total, nil
}

// GetFamilyMemberCount retrieves the count of family members
func (r *FamilyRepository) GetFamilyMemberCount(ctx context.Context, familyID uint64) (int32, error) {
	query := `SELECT COUNT(*) FROM family_members WHERE family_id = ?`

	var count int32
	err := r.db.QueryRowContext(ctx, query, familyID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count family members: %w", err)
	}

	return count, nil
}

// GetUserBasicInfo retrieves basic user information for family members
func (r *FamilyRepository) GetUserBasicInfo(ctx context.Context, userID uint64) (*models.UserBasic, error) {
	query := `SELECT id, code, name FROM users WHERE id = ?`

	var user models.UserBasic
	var code, name sql.NullString
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&user.ID, &code, &name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	if code.Valid {
		user.Code = code.String
	}
	if name.Valid {
		user.Name = name.String
	}

	// Get profile photo
	photoQuery := `
		SELECT url FROM images 
		WHERE imageable_type = 'App\\Models\\User' 
		AND imageable_id = ? 
		ORDER BY id DESC LIMIT 1
	`
	var photoURL string
	err = r.db.QueryRowContext(ctx, photoQuery, userID).Scan(&photoURL)
	if err == nil {
		user.ProfilePhoto = &photoURL
	}

	return &user, nil
}

// FindMemberByUserAndFamily finds a family member by user ID and family ID
func (r *FamilyRepository) FindMemberByUserAndFamily(ctx context.Context, userID, familyID uint64) (*models.FamilyMember, error) {
	query := `SELECT id, family_id, user_id, relationship, created_at, updated_at 
	          FROM family_members 
	          WHERE user_id = ? AND family_id = ?`

	var member models.FamilyMember
	var createdAt, updatedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, userID, familyID).Scan(
		&member.ID,
		&member.FamilyID,
		&member.UserID,
		&member.Relationship,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find family member: %w", err)
	}
	if createdAt.Valid {
		member.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		member.UpdatedAt = updatedAt.Time
	}

	return &member, nil
}
