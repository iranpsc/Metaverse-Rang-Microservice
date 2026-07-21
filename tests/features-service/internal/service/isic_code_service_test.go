package service_test

import (
	"context"
	"errors"
	"testing"

	"metarang/features-service/internal/models"
	"metarang/features-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockIsicCodeRepo struct {
	rows         []models.IsicCode
	total        int
	listErr      error
	countErr     error
	lastSearch   string
	lastLimit    int
	lastOffset   int
	lastCountQry string
}

func (m *mockIsicCodeRepo) FindPaginated(ctx context.Context, search string, limit, offset int) ([]models.IsicCode, error) {
	m.lastSearch = search
	m.lastLimit = limit
	m.lastOffset = offset
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.rows, nil
}

func (m *mockIsicCodeRepo) Count(ctx context.Context, search string) (int, error) {
	m.lastCountQry = search
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.total, nil
}

func TestIsicCodeService_Paginate_DefaultsPageAndPerPage(t *testing.T) {
	repo := &mockIsicCodeRepo{total: 0}
	svc := service.NewIsicCodeService(repo)

	page, err := svc.Paginate(context.Background(), 0, "")
	require.NoError(t, err)
	assert.Equal(t, 1, page.CurrentPage)
	assert.Equal(t, models.IsicCodePerPage, page.PerPage)
	assert.Equal(t, models.IsicCodePath, page.Path)
	assert.Equal(t, "", page.Search)
	assert.Equal(t, 10, repo.lastLimit)
	assert.Equal(t, 0, repo.lastOffset)
	assert.Equal(t, "", repo.lastSearch)
}

func TestIsicCodeService_Paginate_TrimsSearch(t *testing.T) {
	repo := &mockIsicCodeRepo{total: 0}
	svc := service.NewIsicCodeService(repo)

	_, err := svc.Paginate(context.Background(), 1, "  textile  ")
	require.NoError(t, err)
	assert.Equal(t, "textile", repo.lastSearch)
	assert.Equal(t, "textile", repo.lastCountQry)
}

func TestIsicCodeService_Paginate_EmptySearchListsAll(t *testing.T) {
	repo := &mockIsicCodeRepo{total: 0}
	svc := service.NewIsicCodeService(repo)

	_, err := svc.Paginate(context.Background(), 1, "   ")
	require.NoError(t, err)
	assert.Equal(t, "", repo.lastSearch)
}

func TestIsicCodeService_Paginate_MapsRows(t *testing.T) {
	code := uint64(1311)
	repo := &mockIsicCodeRepo{
		total: 1,
		rows: []models.IsicCode{
			{ID: 5, Name: "Manufacture of textiles", Code: &code, Verified: true},
		},
	}
	svc := service.NewIsicCodeService(repo)

	page, err := svc.Paginate(context.Background(), 1, "textile")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, uint64(5), page.Items[0].ID)
	assert.Equal(t, "Manufacture of textiles", page.Items[0].Name)
	require.NotNil(t, page.Items[0].Code)
	assert.Equal(t, uint64(1311), *page.Items[0].Code)
	assert.True(t, page.Items[0].Verified)
	assert.Equal(t, "textile", page.Search)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, 1, page.LastPage)
	require.NotNil(t, page.From)
	require.NotNil(t, page.To)
	assert.Equal(t, 1, *page.From)
	assert.Equal(t, 1, *page.To)
}

func TestIsicCodeService_Paginate_SecondPageOffset(t *testing.T) {
	repo := &mockIsicCodeRepo{total: 25, rows: nil}
	svc := service.NewIsicCodeService(repo)

	page, err := svc.Paginate(context.Background(), 2, "131")
	require.NoError(t, err)
	assert.Equal(t, 2, page.CurrentPage)
	assert.Equal(t, 3, page.LastPage)
	assert.Equal(t, 25, page.Total)
	assert.Equal(t, 10, repo.lastLimit)
	assert.Equal(t, 10, repo.lastOffset)
	assert.Equal(t, "131", repo.lastSearch)
	assert.Nil(t, page.From)
	assert.Nil(t, page.To)
}

func TestIsicCodeService_Paginate_ListError(t *testing.T) {
	repo := &mockIsicCodeRepo{listErr: errors.New("db down")}
	svc := service.NewIsicCodeService(repo)

	_, err := svc.Paginate(context.Background(), 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list isic codes")
}

func TestIsicCodeService_Paginate_CountError(t *testing.T) {
	repo := &mockIsicCodeRepo{countErr: errors.New("count failed")}
	svc := service.NewIsicCodeService(repo)

	_, err := svc.Paginate(context.Background(), 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count isic codes")
}
