package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"testing"
	"time"

	"metarang/buildings-service/internal/repository"
	"metarang/buildings-service/tests/internal/testutil"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func expectFeatureLock(mock sqlmock.Sqlmock, featureID uint64, result interface{}, err error) {
	expectation := mock.ExpectQuery("SELECT GET_LOCK").WithArgs("buildings:feature:" + strconv.FormatUint(featureID, 10))
	if err != nil {
		expectation.WillReturnError(err)
		return
	}
	expectation.WillReturnRows(sqlmock.NewRows([]string{"lock"}).AddRow(result))
}

func expectFeatureUnlock(mock sqlmock.Sqlmock, featureID uint64) {
	mock.ExpectQuery("SELECT RELEASE_LOCK").
		WithArgs("buildings:feature:" + strconv.FormatUint(featureID, 10)).
		WillReturnRows(sqlmock.NewRows([]string{"released"}).AddRow(1))
}

func TestBuildingRepository_CreateBuilding_ErrorPaths(t *testing.T) {
	const featureID uint64 = 7
	now := time.Now()

	tests := []struct {
		name  string
		setup func(sqlmock.Sqlmock)
		call  func(*repository.BuildingRepository) error
	}{
		{
			name:  "invalid model id",
			setup: func(sqlmock.Sqlmock) {},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "not-numeric", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "lock query failure",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, nil, errors.New("database unavailable"))
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "lock timeout",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 0, nil)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "transaction begin failure",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "existence query failure",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").WillReturnError(errors.New("count failed"))
				mock.ExpectRollback()
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "feature already has building",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectRollback()
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "model row missing",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT id FROM building_models").WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "model lookup failure",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT id FROM building_models").WillReturnError(errors.New("model lookup failed"))
				mock.ExpectRollback()
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "insert failure",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT id FROM building_models").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
				mock.ExpectExec("INSERT INTO buildings").WillReturnError(errors.New("insert failed"))
				mock.ExpectRollback()
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
		{
			name: "commit failure",
			setup: func(mock sqlmock.Sqlmock) {
				expectFeatureLock(mock, featureID, 1, nil)
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM buildings").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery("SELECT id FROM building_models").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(9))
				mock.ExpectExec("INSERT INTO buildings").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
				expectFeatureUnlock(mock, featureID)
			},
			call: func(repo *repository.BuildingRepository) error {
				return repo.CreateBuilding(context.Background(), featureID, 42, "1001", "20", "0", "1,2", "", now, now, 1)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, mock := testutil.NewSQLMock(t)
			tc.setup(mock)
			err := tc.call(repository.NewBuildingRepository(db))
			require.Error(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
