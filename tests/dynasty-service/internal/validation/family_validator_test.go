package validation_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/validation"
)

func TestFamilyValidator_ValidateRelationship(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	validationRepo := repository.NewValidationRepository(db)
	validator := validation.NewFamilyValidator(validationRepo)

	validRelationships := []string{"father", "mother", "offspring", "husband", "wife", "brother", "sister"}
	for _, rel := range validRelationships {
		t.Run("Valid_"+rel, func(t *testing.T) {
			err := validator.ValidateRelationship(rel)
			assert.NoError(t, err)
		})
	}

	t.Run("Invalid", func(t *testing.T) {
		err := validator.ValidateRelationship("invalid")
		assert.Error(t, err)
	})
}

func TestFamilyValidator_ValidateRelationshipLimits(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	validationRepo := repository.NewValidationRepository(db)
	validator := validation.NewFamilyValidator(validationRepo)

	ctx := context.Background()
	familyID := uint64(1)

	t.Run("SingleParent_Father", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id = \?$`).
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(familyID, "father").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		err := validator.ValidateRelationshipLimits(ctx, familyID, "father")
		assert.NoError(t, err)
	})

	t.Run("SingleParent_Mother", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id = \?$`).
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(familyID, "mother").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		err := validator.ValidateRelationshipLimits(ctx, familyID, "mother")
		assert.NoError(t, err)
	})

	t.Run("SingleSpouse_Husband", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id = \?$`).
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(familyID, "husband").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0))

		err := validator.ValidateRelationshipLimits(ctx, familyID, "husband")
		assert.NoError(t, err)
	})

	t.Run("MaxSpouse_Wife", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id = \?$`).
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(2))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(familyID, "wife").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))

		err := validator.ValidateRelationshipLimits(ctx, familyID, "wife")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "همسر")
	})

	t.Run("BrotherSister_CombinedLimit", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id = \?$`).
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(5))
		mock.ExpectQuery("relationship IN").
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(4))
		err := validator.ValidateRelationshipLimits(ctx, familyID, "brother")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "4")
	})

	t.Run("MaxOffspring", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id = \?$`).
			WithArgs(familyID).
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(5))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(familyID, "offspring").
			WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(4))

		err := validator.ValidateRelationshipLimits(ctx, familyID, "offspring")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "4")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFamilyValidator_ValidateAddFamilyMember(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	validationRepo := repository.NewValidationRepository(db)
	validator := validation.NewFamilyValidator(validationRepo)

	ctx := context.Background()
	fromUserID := uint64(1)
	toUserID := uint64(2)

	t.Run("NoPendingRequest", func(t *testing.T) {
		// Order matches FamilyValidator.ValidateAddFamilyMember
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(fromUserID, toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(fromUserID, toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery("SELECT u.code").
			WithArgs(toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"code", "verified"}).AddRow("U2", true))

		err := validator.ValidateAddFamilyMember(ctx, fromUserID, toUserID, "offspring", false)
		assert.NoError(t, err)
	})

	t.Run("HasPendingRequest", func(t *testing.T) {
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(fromUserID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(fromUserID, toUserID).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		err := validator.ValidateAddFamilyMember(ctx, fromUserID, toUserID, "offspring", false)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ارسال")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
