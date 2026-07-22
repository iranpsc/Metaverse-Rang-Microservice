package policy_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/policy"
	"metarang/dynasty-service/internal/repository"
)

func TestDynastyPolicy_CanCreateDynasty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dynastyRepo := repository.NewDynastyRepository(db)
	p := policy.NewDynastyPolicy(dynastyRepo)

	ctx := context.Background()
	userID := uint64(1)
	featureID := uint64(100)

	t.Run("Success", func(t *testing.T) {
		// Check user doesn't have dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		// Check feature has no pending requests
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(featureID).
			WillReturnRows(sqlmock.NewRows([]string{"EXISTS("}).AddRow(false))

		canCreate, msg, err := p.CanCreateDynasty(ctx, userID, featureID)
		require.NoError(t, err)
		assert.True(t, canCreate)
		assert.Empty(t, msg)
	})

	t.Run("UserAlreadyHasDynasty", func(t *testing.T) {
		// User already has a dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, userID, featureID, time.Now(), time.Now()))

		canCreate, msg, err := p.CanCreateDynasty(ctx, userID, featureID)
		require.NoError(t, err)
		assert.False(t, canCreate)
		assert.Contains(t, msg, "قبلا سلسله تاسیس کرده")
	})

	t.Run("FeatureHasPendingRequest", func(t *testing.T) {
		// Check user doesn't have dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		// Feature has pending requests
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs(featureID).
			WillReturnRows(sqlmock.NewRows([]string{"EXISTS("}).AddRow(true))

		canCreate, msg, err := p.CanCreateDynasty(ctx, userID, featureID)
		require.NoError(t, err)
		assert.False(t, canCreate)
		assert.Contains(t, msg, "درخواست در انتظار")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyPolicy_CanUpdateDynastyFeature(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	dynastyRepo := repository.NewDynastyRepository(db)
	p := policy.NewDynastyPolicy(dynastyRepo)

	ctx := context.Background()
	userID := uint64(1)
	dynastyID := uint64(1)
	currentFeatureID := uint64(100)
	newFeatureID := uint64(200)

	t.Run("Success", func(t *testing.T) {
		// Get dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, currentFeatureID, time.Now(), time.Now()))

		canUpdate, msg, err := p.CanUpdateDynastyFeature(ctx, userID, dynastyID, newFeatureID)
		require.NoError(t, err)
		assert.True(t, canUpdate)
		assert.Empty(t, msg)
	})

	t.Run("UserDoesNotOwnDynasty", func(t *testing.T) {
		otherUserID := uint64(2)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, currentFeatureID, time.Now(), time.Now()))

		canUpdate, msg, err := p.CanUpdateDynastyFeature(ctx, otherUserID, dynastyID, newFeatureID)
		require.NoError(t, err)
		assert.False(t, canUpdate)
		assert.Contains(t, msg, "مالک این سلسله نیستید")
	})

	t.Run("SameFeature", func(t *testing.T) {
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, currentFeatureID, time.Now(), time.Now()))

		canUpdate, msg, err := p.CanUpdateDynastyFeature(ctx, userID, dynastyID, currentFeatureID)
		require.NoError(t, err)
		assert.False(t, canUpdate)
		assert.Contains(t, msg, "هم اکنون ملک سلسله")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
