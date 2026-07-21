package handler_test

import (
	"context"
	"errors"
	"testing"

	"metarang/features-service/internal/handler"
	"metarang/features-service/internal/models"
	pb "metarang/shared/pb/features"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockIsicCodePort struct {
	paginate func(ctx context.Context, page int, search string) (*models.IsicCodePage, error)
}

func (m *mockIsicCodePort) Paginate(ctx context.Context, page int, search string) (*models.IsicCodePage, error) {
	if m.paginate != nil {
		return m.paginate(ctx, page, search)
	}
	return nil, errors.New("not implemented")
}

func TestIsicCodeHandler_ListIsicCodes_ServiceError(t *testing.T) {
	m := &mockIsicCodePort{}
	m.paginate = func(ctx context.Context, page int, search string) (*models.IsicCodePage, error) {
		return nil, errors.New("db failure")
	}
	h := handler.NewIsicCodeHandler(m)

	_, err := h.ListIsicCodes(context.Background(), &pb.ListIsicCodesRequest{Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestIsicCodeHandler_ListIsicCodes_SuccessWithSearch(t *testing.T) {
	code := uint64(1311)
	from, to := 1, 1
	m := &mockIsicCodePort{}
	m.paginate = func(ctx context.Context, page int, search string) (*models.IsicCodePage, error) {
		assert.Equal(t, 2, page)
		assert.Equal(t, "textile", search)
		return &models.IsicCodePage{
			Items: []models.IsicCode{
				{ID: 5, Name: "Manufacture of textiles", Code: &code, Verified: true},
			},
			CurrentPage: 2,
			PerPage:     10,
			Total:       11,
			LastPage:    2,
			From:        &from,
			To:          &to,
			Path:        models.IsicCodePath,
			Search:      "textile",
		}, nil
	}

	h := handler.NewIsicCodeHandler(m)
	resp, err := h.ListIsicCodes(context.Background(), &pb.ListIsicCodesRequest{Page: 2, Search: "textile"})
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, uint64(5), resp.Data[0].Id)
	assert.Equal(t, "Manufacture of textiles", resp.Data[0].Name)
	assert.Equal(t, uint64(1311), resp.Data[0].GetCode())
	assert.True(t, resp.Data[0].Verified)
	assert.Equal(t, int32(2), resp.Meta.CurrentPage)
	assert.Equal(t, int32(11), resp.Meta.Total)
	assert.Contains(t, resp.Links.First, "search=textile")
	assert.Contains(t, resp.Links.Prev, "page=1")
	assert.Equal(t, "", resp.Links.Next)
}

func TestIsicCodeHandler_ListIsicCodes_DefaultsPage(t *testing.T) {
	m := &mockIsicCodePort{}
	m.paginate = func(ctx context.Context, page int, search string) (*models.IsicCodePage, error) {
		assert.Equal(t, 1, page)
		assert.Equal(t, "", search)
		return &models.IsicCodePage{
			Items:       []models.IsicCode{},
			CurrentPage: 1,
			PerPage:     10,
			Total:       0,
			LastPage:    1,
			Path:        models.IsicCodePath,
		}, nil
	}

	h := handler.NewIsicCodeHandler(m)
	resp, err := h.ListIsicCodes(context.Background(), &pb.ListIsicCodesRequest{Page: 0})
	require.NoError(t, err)
	assert.Empty(t, resp.Data)
	assert.Equal(t, int32(0), resp.Meta.Total)
	assert.Equal(t, "/api/isic-codes?page=1", resp.Links.First)
}
