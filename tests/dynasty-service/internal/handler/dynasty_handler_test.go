package handler_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/handler"
	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
	dynastypb "metarang/shared/pb/dynasty"
)

func TestDynastyHandler_CreateDynasty(t *testing.T) {
	ctx := context.Background()
	userID := uint64(1)
	featureID := uint64(100)

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		// Setup repositories and service
		dynastyRepo := repository.NewDynastyRepository(db)
		familyRepo := repository.NewFamilyRepository(db)
		prizeRepo := repository.NewPrizeRepository(db)
		dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")
		handler := handler.NewDynastyHandler(dynastyService)

		// Check existing dynasty - none exists
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

		// Get feature details (called after creation)
		mock.ExpectQuery("SELECT f.id, fp.id as properties_id").
			WithArgs(featureID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "area", "density", "stability"}).
				AddRow(featureID, 1, "1000", "50", "15000"))

		// Get user features
		mock.ExpectQuery("SELECT f.id, fp.id as properties_id").
			WithArgs(userID, featureID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "density", "stability", "area"}))

		// Get profile photo (may return nil)
		mock.ExpectQuery("SELECT url FROM images").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		// Get family member count
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		req := &dynastypb.CreateDynastyRequest{
			UserId:    userID,
			FeatureId: featureID,
		}

		resp, err := handler.CreateDynasty(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.UserHasDynasty)
		assert.Equal(t, uint64(1), resp.Id)
		assert.Equal(t, uint64(1), resp.FamilyId)
		assert.NotEmpty(t, resp.CreatedAt) // Jalali formatted date
		assert.NotNil(t, resp.DynastyFeature)
		assert.Equal(t, featureID, resp.DynastyFeature.Id)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user already has dynasty", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		dynastyRepo := repository.NewDynastyRepository(db)
		familyRepo := repository.NewFamilyRepository(db)
		prizeRepo := repository.NewPrizeRepository(db)
		dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")
		handler := handler.NewDynastyHandler(dynastyService)

		now := time.Now()

		// User already has a dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(1, userID, featureID, now, now))

		req := &dynastypb.CreateDynastyRequest{
			UserId:    userID,
			FeatureId: featureID,
		}

		resp, err := handler.CreateDynasty(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.AlreadyExists, st.Code())

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("nil service", func(t *testing.T) {
		handler := &handler.DynastyHandler{}

		req := &dynastypb.CreateDynastyRequest{
			UserId:    userID,
			FeatureId: featureID,
		}

		resp, err := handler.CreateDynasty(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}

func TestDynastyHandler_UpdateDynastyFeature(t *testing.T) {
	ctx := context.Background()
	dynastyID := uint64(1)
	userID := uint64(1)
	newFeatureID := uint64(200)

	t.Run("success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		dynastyRepo := repository.NewDynastyRepository(db)
		familyRepo := repository.NewFamilyRepository(db)
		prizeRepo := repository.NewPrizeRepository(db)
		dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")
		handler := handler.NewDynastyHandler(dynastyService)

		now := time.Now()

		// UpdateDynastyFeature: Get dynasty to verify ownership
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now.AddDate(0, 0, -60), now.AddDate(0, 0, -31)))

		// UpdateDynastyFeature: Update feature
		mock.ExpectExec("UPDATE dynasties SET feature_id").
			WithArgs(newFeatureID, dynastyID).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// GetDynasty (called after update): Get dynasty
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, newFeatureID, now, now))

		// GetDynasty: Get family
		mock.ExpectQuery("SELECT id, dynasty_id, created_at, updated_at FROM families WHERE dynasty_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(1, dynastyID, now, now))

		// GetDynasty: Get feature details
		mock.ExpectQuery("SELECT").
			WithArgs(newFeatureID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "area", "density", "stability"}).
				AddRow(newFeatureID, "2", "2000", "60", "20000"))

		// GetDynasty: Get user features
		mock.ExpectQuery("SELECT").
			WithArgs(userID, newFeatureID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "area", "density", "stability", "karbari"}))

		// GetDynasty: Get profile photo
		mock.ExpectQuery("SELECT url FROM images").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		// GetDynasty: Get family member count
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM family_members").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		req := &dynastypb.UpdateDynastyFeatureRequest{
			DynastyId: dynastyID,
			FeatureId: newFeatureID,
			UserId:    userID,
		}

		resp, err := handler.UpdateDynastyFeature(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.UserHasDynasty)
		assert.Equal(t, dynastyID, resp.Id)
		assert.NotNil(t, resp.DynastyFeature)
		assert.Equal(t, newFeatureID, resp.DynastyFeature.Id)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("unauthorized - user does not own dynasty", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		dynastyRepo := repository.NewDynastyRepository(db)
		familyRepo := repository.NewFamilyRepository(db)
		prizeRepo := repository.NewPrizeRepository(db)
		dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")
		handler := handler.NewDynastyHandler(dynastyService)

		now := time.Now()
		otherUserID := uint64(2)

		// Get dynasty - user doesn't own it
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now, now))

		req := &dynastypb.UpdateDynastyFeatureRequest{
			DynastyId: dynastyID,
			FeatureId: newFeatureID,
			UserId:    otherUserID, // Different user
		}

		resp, err := handler.UpdateDynastyFeature(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code()) // mapServiceError maps "unauthorized" to PermissionDenied

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("dynasty not found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		dynastyRepo := repository.NewDynastyRepository(db)
		familyRepo := repository.NewFamilyRepository(db)
		prizeRepo := repository.NewPrizeRepository(db)
		dynastyService := service.NewDynastyService(dynastyRepo, familyRepo, prizeRepo, "localhost:50054")
		handler := handler.NewDynastyHandler(dynastyService)

		// Dynasty doesn't exist
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnError(sql.ErrNoRows)

		req := &dynastypb.UpdateDynastyFeatureRequest{
			DynastyId: dynastyID,
			FeatureId: newFeatureID,
			UserId:    userID,
		}

		resp, err := handler.UpdateDynastyFeature(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code()) // mapServiceError maps "not found" to NotFound

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("nil service", func(t *testing.T) {
		handler := &handler.DynastyHandler{}

		req := &dynastypb.UpdateDynastyFeatureRequest{
			DynastyId: dynastyID,
			FeatureId: newFeatureID,
			UserId:    userID,
		}

		resp, err := handler.UpdateDynastyFeature(ctx, req)
		require.Error(t, err)
		assert.Nil(t, resp)

		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
	})
}
