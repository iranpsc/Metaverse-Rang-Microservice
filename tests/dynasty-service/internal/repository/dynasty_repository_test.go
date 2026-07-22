package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"metarang/dynasty-service/internal/repository"

	"metarang/dynasty-service/internal/models"
)

func TestDynastyRepository_CRUDAndMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewDynastyRepository(db)
	ctx := context.Background()
	now := time.Now()

	d := &models.Dynasty{UserID: 1, FeatureID: 100}
	mock.ExpectExec("INSERT INTO dynasties").WithArgs(d.UserID, d.FeatureID).WillReturnResult(sqlmock.NewResult(10, 1))
	require.NoError(t, r.CreateDynasty(ctx, d))
	assert.Equal(t, uint64(10), d.ID)

	mock.ExpectQuery("SELECT id, user_id, feature_id").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "feature_id", "created_at", "updated_at"}).AddRow(1, 1, 100, now, now))
	got, err := r.GetDynastyByUserID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, got)

	mock.ExpectExec("UPDATE dynasties SET feature_id").WithArgs(uint64(200), uint64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.UpdateDynastyFeature(ctx, 1, 200))

	mock.ExpectQuery("SELECT message FROM dynasty_messages").WithArgs("receiver_message").WillReturnRows(sqlmock.NewRows([]string{"message"}).AddRow("hello"))
	msg, err := r.GetDynastyMessage(ctx, "receiver_message")
	require.NoError(t, err)
	assert.Equal(t, "hello", msg)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDynastyRepository_PenaltyMethods(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := repository.NewDynastyRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT fp.karbari, fp.stability").WithArgs(uint64(100)).WillReturnRows(sqlmock.NewRows([]string{"karbari", "stability"}).AddRow("m", 25000.0))
	k, s, err := r.GetFeaturePenaltyData(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, "m", k)
	assert.Equal(t, 25000.0, s)

	mock.ExpectExec("INSERT INTO debts").WithArgs(uint64(1), 250.0, "update-dynasty-feature").WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, r.CreateDebt(ctx, 1, "yellow", 250.0, "update-dynasty-feature"))

	mock.ExpectExec("INSERT INTO locked_features").WithArgs(uint64(100), "dynasty-feature-change", sqlmock.AnyArg(), 0).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, r.LockFeature(ctx, 100, "dynasty-feature-change", time.Now().AddDate(0, 1, 0), 0))

	mock.ExpectExec("UPDATE feature_properties SET label").WithArgs("locked", uint64(100)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, r.SetFeatureLabel(ctx, 100, "locked"))

	require.NoError(t, mock.ExpectationsWereMet())
}
