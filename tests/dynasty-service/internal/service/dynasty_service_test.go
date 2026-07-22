package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
)

func TestDynastyService_CreateDynasty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")

	ctx := context.Background()
	userID := uint64(1)
	featureID := uint64(100)

	t.Run("Success", func(t *testing.T) {
		// Check existing dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		// Create dynasty
		mock.ExpectExec("INSERT INTO dynasties").
			WithArgs(userID, featureID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Create family
		mock.ExpectExec("INSERT INTO families").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// Add owner member
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), userID, "owner").
			WillReturnResult(sqlmock.NewResult(1, 1))

		dynasty, family, err := service.CreateDynasty(ctx, userID, featureID)
		require.NoError(t, err)
		assert.NotNil(t, dynasty)
		assert.NotNil(t, family)
		assert.Equal(t, userID, dynasty.UserID)
		assert.Equal(t, featureID, dynasty.FeatureID)
	})

	t.Run("UserAlreadyHasDynasty", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, userID, featureID, time.Now(), time.Now()))

		dynasty, family, err := service.CreateDynasty(ctx, userID, featureID)
		assert.Error(t, err)
		assert.Nil(t, dynasty)
		assert.Nil(t, family)
		assert.Contains(t, err.Error(), "already has a dynasty")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyService_GetDynastyByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")

	ctx := context.Background()
	dynastyID := uint64(1)

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, 1, 100, time.Now(), time.Now()))

		dynasty, err := service.GetDynastyByID(ctx, dynastyID)
		require.NoError(t, err)
		assert.NotNil(t, dynasty)
		assert.Equal(t, dynastyID, dynasty.ID)
	})

	t.Run("NotFound", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnError(sql.ErrNoRows)

		dynasty, err := service.GetDynastyByID(ctx, dynastyID)
		assert.Error(t, err)
		assert.Nil(t, dynasty)
		assert.Contains(t, err.Error(), "not found")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyService_UpdateDynastyFeature(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dynastyRepo := repository.NewDynastyRepository(db)
	familyRepo := repository.NewFamilyRepository(db)
	prizeRepo := repository.NewPrizeRepository(db)
	service := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")

	ctx := context.Background()
	dynastyID := uint64(1)
	userID := uint64(1)
	newFeatureID := uint64(200)

	t.Run("Success", func(t *testing.T) {
		// Get dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -31)))

		// Update feature
		mock.ExpectExec("UPDATE dynasties SET feature_id").
			WithArgs(newFeatureID, dynastyID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := service.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		require.NoError(t, err)
	})

	t.Run("Unauthorized", func(t *testing.T) {
		otherUserID := uint64(2)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, time.Now().AddDate(0, 0, -60), time.Now().AddDate(0, 0, -31)))

		err := service.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, otherUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
