package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"
	"metarang/dynasty-service/internal/service"
)

func newDynastyService(t *testing.T) (sqlmock.Sqlmock, *service.DynastyService) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	svc := service.NewDynastyService(
		repository.NewDynastyRepository(db),
		repository.NewFamilyRepository(db),
		repository.NewPrizeRepository(db),
		"localhost:50054",
	)
	return mock, svc
}

func TestDynastyService_CreateDynasty_ErrorPaths(t *testing.T) {
	ctx := context.Background()
	userID, featureID := uint64(1), uint64(100)

	t.Run("CheckExistingError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(errors.New("db down"))
		dynasty, family, err := svc.CreateDynasty(ctx, userID, featureID)
		assert.Error(t, err)
		assert.Nil(t, dynasty)
		assert.Nil(t, family)
		assert.Contains(t, err.Error(), "failed to check existing dynasty")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateDynastyRepoError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)
		expectCreateFeatureGate(mock, userID, featureID)
		mock.ExpectExec("INSERT INTO dynasties").
			WithArgs(userID, featureID).
			WillReturnError(errors.New("insert failed"))
		_, _, err := svc.CreateDynasty(ctx, userID, featureID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create dynasty")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateFamilyError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)
		expectCreateFeatureGate(mock, userID, featureID)
		mock.ExpectExec("INSERT INTO dynasties").
			WithArgs(userID, featureID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO families").
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(errors.New("family insert failed"))
		_, _, err := svc.CreateDynasty(ctx, userID, featureID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create family")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("FeatureNotOwned", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery("FROM kycs").
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"verified"}).AddRow(true))
		mock.ExpectQuery("SELECT f.owner_id, fp.karbari").
			WithArgs(featureID).
			WillReturnRows(sqlmock.NewRows([]string{"owner_id", "karbari"}).AddRow(uint64(99), "m"))
		_, _, err := svc.CreateDynasty(ctx, userID, featureID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "مالک این ملک نیستید")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AddOwnerError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)
		expectCreateFeatureGate(mock, userID, featureID)
		mock.ExpectExec("INSERT INTO dynasties").
			WithArgs(userID, featureID).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO families").
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("INSERT INTO family_members").
			WithArgs(sqlmock.AnyArg(), userID, "owner").
			WillReturnError(errors.New("member insert failed"))
		_, _, err := svc.CreateDynasty(ctx, userID, featureID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add owner to family")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDynastyService_GetDynastyByID_RepoError(t *testing.T) {
	mock, svc := newDynastyService(t)
	mock.ExpectQuery("SELECT id, user_id, feature_id").
		WithArgs(uint64(1)).
		WillReturnError(errors.New("timeout"))
	dynasty, err := svc.GetDynastyByID(context.Background(), 1)
	assert.Error(t, err)
	assert.Nil(t, dynasty)
	assert.Contains(t, err.Error(), "failed to get dynasty")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyService_GetDynastyByUserID(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("Found", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(uint64(7)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(3, 7, 100, now, now))
		dynasty, err := svc.GetDynastyByUserID(ctx, 7)
		require.NoError(t, err)
		require.NotNil(t, dynasty)
		assert.Equal(t, uint64(3), dynasty.ID)
		assert.Equal(t, uint64(7), dynasty.UserID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("None", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(uint64(7)).
			WillReturnError(sql.ErrNoRows)
		dynasty, err := svc.GetDynastyByUserID(ctx, 7)
		require.NoError(t, err)
		assert.Nil(t, dynasty)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("RepoError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(uint64(7)).
			WillReturnError(errors.New("db"))
		_, err := svc.GetDynastyByUserID(ctx, 7)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get dynasty")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDynastyService_UpdateDynastyFeature_ValidationAndPenalty(t *testing.T) {
	ctx := context.Background()
	dynastyID, userID, newFeatureID := uint64(1), uint64(10), uint64(200)
	now := time.Now()

	t.Run("GetError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnError(errors.New("db"))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get dynasty")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("NotFound", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnError(sql.ErrNoRows)
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dynasty not found")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("AlreadySameFeature", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, newFeatureID, now, now.AddDate(0, 0, -60)))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already dynasty feature")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("UpdateRepoErrorOutsidePenaltyWindow", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now.AddDate(0, 0, -60), now.AddDate(0, 0, -31)))
		expectUpdateFeatureGate(mock, userID, newFeatureID)
		mock.ExpectExec("UPDATE dynasties SET feature_id").
			WithArgs(newFeatureID, dynastyID).
			WillReturnError(errors.New("update failed"))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update dynasty feature")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	for _, tc := range []struct {
		name    string
		karbari string
	}{
		{name: "KarbariM_Yellow", karbari: "m"},
		{name: "KarbariT_Red", karbari: "t"},
		{name: "KarbariA_Blue", karbari: "a"},
		{name: "KarbariDefault_Yellow", karbari: "x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock, svc := newDynastyService(t)
			mock.ExpectQuery("SELECT id, user_id, feature_id").
				WithArgs(dynastyID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
					AddRow(dynastyID, userID, 100, now, now.Add(-time.Hour)))
			expectUpdateFeatureGate(mock, userID, newFeatureID)
			mock.ExpectQuery("SELECT fp.karbari, fp.stability").
				WithArgs(uint64(100)).
				WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow(tc.karbari, 10000.0))
			mock.ExpectExec("INSERT INTO debts").
				WithArgs(userID, 100.0, "update-dynasty-feature").
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec("INSERT INTO locked_features").
				WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec("UPDATE feature_properties SET label").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE dynasties SET feature_id").
				WithArgs(newFeatureID, dynastyID).
				WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("ZeroDebtSkipsCreateDebt", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now, now.Add(-time.Hour)))
		expectUpdateFeatureGate(mock, userID, newFeatureID)
		mock.ExpectQuery("SELECT fp.karbari, fp.stability").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow("m", 0.0))
		mock.ExpectExec("INSERT INTO locked_features").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE feature_properties SET label").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE dynasties SET feature_id").
			WithArgs(newFeatureID, dynastyID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("PenaltyDataError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now, now.Add(-time.Hour)))
		expectUpdateFeatureGate(mock, userID, newFeatureID)
		mock.ExpectQuery("SELECT fp.karbari, fp.stability").
			WithArgs(uint64(100)).
			WillReturnError(errors.New("penalty query failed"))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get feature penalty data")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("CreateDebtError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now, now.Add(-time.Hour)))
		expectUpdateFeatureGate(mock, userID, newFeatureID)
		mock.ExpectQuery("SELECT fp.karbari, fp.stability").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow("m", 10000.0))
		mock.ExpectExec("INSERT INTO debts").
			WillReturnError(errors.New("debt insert failed"))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create debt")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("LockFeatureError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now, now.Add(-time.Hour)))
		expectUpdateFeatureGate(mock, userID, newFeatureID)
		mock.ExpectQuery("SELECT fp.karbari, fp.stability").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow("m", 0.0))
		mock.ExpectExec("INSERT INTO locked_features").
			WillReturnError(errors.New("lock failed"))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to lock feature")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("SetFeatureLabelError", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT id, user_id, feature_id").
			WithArgs(dynastyID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).
				AddRow(dynastyID, userID, 100, now, now.Add(-time.Hour)))
		expectUpdateFeatureGate(mock, userID, newFeatureID)
		mock.ExpectQuery("SELECT fp.karbari, fp.stability").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow("m", 0.0))
		mock.ExpectExec("INSERT INTO locked_features").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec("UPDATE feature_properties SET label").
			WillReturnError(errors.New("label failed"))
		err := svc.UpdateDynastyFeature(ctx, dynastyID, newFeatureID, userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set feature label")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDynastyService_PassthroughLookups(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	t.Run("GetFeatureDetails", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT f.id, fp.id as properties_id").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "area", "density", "stability"}).
				AddRow(100, "p1", "a", "d", "15000"))
		mock.ExpectQuery("FROM coordinates c").
			WithArgs(uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"feature_id", "id", "x", "y"}).
				AddRow(uint64(100), uint64(9), "51.389000", "35.689200"))
		details, err := svc.GetFeatureDetails(ctx, 100)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), details["id"])
		assert.Equal(t, "15000", details["stability"])
		assert.Equal(t, []map[string]interface{}{
			{"id": uint64(9), "x": "51.389000", "y": "35.689200"},
		}, details["coordinates"])
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetUserFeatures", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT f.id, fp.id as properties_id").
			WithArgs(uint64(1), uint64(100)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "properties_id", "area", "density", "stability", "karbari"}).
				AddRow(uint64(101), "p2", "a", "d", "s", "m"))
		features, err := svc.GetUserFeatures(ctx, 1, 100)
		require.NoError(t, err)
		require.Len(t, features, 1)
		assert.Equal(t, uint64(101), features[0]["id"])
		_, hasCoordinates := features[0]["coordinates"]
		assert.False(t, hasCoordinates)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetUserProfilePhoto", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT url FROM images").
			WithArgs(uint64(5)).
			WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://photo"))
		photo, err := svc.GetUserProfilePhoto(ctx, 5)
		require.NoError(t, err)
		require.NotNil(t, photo)
		assert.Equal(t, "https://photo", *photo)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetFamilyByDynastyID", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("FROM families WHERE dynasty_id").
			WithArgs(uint64(3)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "dynasty_id", "created_at", "updated_at"}).
				AddRow(9, 3, now, now))
		fam, err := svc.GetFamilyByDynastyID(ctx, 3)
		require.NoError(t, err)
		require.NotNil(t, fam)
		assert.Equal(t, uint64(9), fam.ID)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetFamilyMemberCount", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM family_members WHERE family_id`).
			WithArgs(uint64(9)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
		count, err := svc.GetFamilyMemberCount(ctx, 9)
		require.NoError(t, err)
		assert.Equal(t, int32(4), count)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetIntroductionPrizes", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("FROM dynasty_prizes").
			WillReturnRows(sqlmock.NewRows([]string{"id", "member", "satisfaction", "introduction_profit_increase", "accumulated_capital_reserve", "data_storage", "psc", "created_at", "updated_at"}).
				AddRow(1, "offspring", 0.1, 0.2, 0.3, 0.4, 1000, now, now))
		prizes, err := svc.GetIntroductionPrizes(ctx)
		require.NoError(t, err)
		require.Len(t, prizes, 1)
		assert.Equal(t, "offspring", prizes[0].Member)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GetVariableRate", func(t *testing.T) {
		mock, svc := newDynastyService(t)
		mock.ExpectQuery("SELECT price FROM variables WHERE asset").
			WithArgs("psc").
			WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(250.5))
		rate, err := svc.GetVariableRate(ctx, "psc")
		require.NoError(t, err)
		assert.Equal(t, 250.5, rate)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
