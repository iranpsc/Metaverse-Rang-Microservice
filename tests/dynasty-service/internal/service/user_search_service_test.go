package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"metarang/dynasty-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSearchService_SearchUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewUserSearchService(db)
	ctx := context.Background()
	now := time.Now()
	_ = now

	mock.ExpectQuery("FROM users u").WithArgs("%ali%", "%ali%", "%ali%", 20).WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "display_name"}).AddRow(1, "U100", "Ali", "Ali Test"))
	mock.ExpectQuery("SELECT url FROM images").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"url"}).AddRow("https://img"))
	mock.ExpectQuery("SELECT l.title").WithArgs(uint64(1)).WillReturnRows(sqlmock.NewRows([]string{"title"}).AddRow("Gold"))

	results, err := svc.SearchUsers(ctx, "ali", 20)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "Ali Test", results[0].Name)
	assert.Equal(t, "Gold", results[0].Level)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserSearchService_SearchUsers_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	svc := service.NewUserSearchService(db)
	ctx := context.Background()

	mock.ExpectQuery("FROM users u").WithArgs("%bad%", "%bad%", "%bad%", 10).WillReturnError(assert.AnError)
	_, err = svc.SearchUsers(ctx, "bad", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to search users")
}
