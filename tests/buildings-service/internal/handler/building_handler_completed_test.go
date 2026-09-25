package handler_test

import (
	"context"
	"errors"
	"testing"

	"metarang/buildings-service/internal/handler"
	"metarang/buildings-service/internal/models"
	pb "metarang/shared/pb/features"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockCompletedBuildingPort struct {
	paginate func(ctx context.Context, page int) (*models.CompletedBuildingPage, error)
}

func (m *mockCompletedBuildingPort) Paginate(ctx context.Context, page int) (*models.CompletedBuildingPage, error) {
	if m.paginate != nil {
		return m.paginate(ctx, page)
	}
	return nil, errors.New("not implemented")
}

func TestBuildingHandler_ListCompletedBuildings_ServiceUnavailable(t *testing.T) {
	h := handler.NewBuildingHandler(&mockBuildingPort{}, nil)
	_, err := h.ListCompletedBuildings(context.Background(), &pb.ListCompletedBuildingsRequest{Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestBuildingHandler_ListCompletedBuildings_ServiceError(t *testing.T) {
	m := &mockCompletedBuildingPort{}
	m.paginate = func(ctx context.Context, page int) (*models.CompletedBuildingPage, error) {
		return nil, errors.New("db failure")
	}
	h := handler.NewBuildingHandler(&mockBuildingPort{}, m)
	_, err := h.ListCompletedBuildings(context.Background(), &pb.ListCompletedBuildingsRequest{Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestBuildingHandler_ListCompletedBuildings_Success(t *testing.T) {
	length := "30"
	width := "50"
	density := "3"
	from, to := 1, 1
	m := &mockCompletedBuildingPort{}
	m.paginate = func(ctx context.Context, page int) (*models.CompletedBuildingPage, error) {
		assert.Equal(t, 2, page)
		return &models.CompletedBuildingPage{
			Items: []models.CompletedBuilding{
				{
					ID:                  42,
					FeatureID:           7,
					FeaturePropertiesID: "ABC-123",
					Length:              &length,
					Width:               &width,
					Density:             &density,
					Karbari:             "m",
				},
			},
			CurrentPage: 2,
			PerPage:     10,
			Total:       11,
			LastPage:    2,
			From:        &from,
			To:          &to,
			Path:        models.CompletedBuildingPath,
		}, nil
	}

	h := handler.NewBuildingHandler(&mockBuildingPort{}, m)
	resp, err := h.ListCompletedBuildings(context.Background(), &pb.ListCompletedBuildingsRequest{Page: 2})
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, uint64(42), resp.Data[0].Id)
	assert.Equal(t, uint64(7), resp.Data[0].FeatureId)
	assert.Equal(t, "ABC-123", resp.Data[0].FeaturePropertiesId)
	assert.Equal(t, "30", resp.Data[0].GetLength())
	assert.Equal(t, "50", resp.Data[0].GetWidth())
	assert.Equal(t, "3", resp.Data[0].GetDensity())
	assert.Equal(t, "m", resp.Data[0].GetKarbari())
	assert.Equal(t, int32(2), resp.Meta.CurrentPage)
	assert.Equal(t, int32(11), resp.Meta.Total)
	assert.Equal(t, "/api/features/buildings/completed?page=1", resp.Links.First)
	assert.Contains(t, resp.Links.Prev, "page=1")
	assert.Equal(t, "", resp.Links.Next)
}

func TestBuildingHandler_ListCompletedBuildings_DefaultsPage(t *testing.T) {
	m := &mockCompletedBuildingPort{}
	m.paginate = func(ctx context.Context, page int) (*models.CompletedBuildingPage, error) {
		assert.Equal(t, 1, page)
		return &models.CompletedBuildingPage{
			Items:       []models.CompletedBuilding{},
			CurrentPage: 1,
			PerPage:     10,
			Total:       0,
			LastPage:    1,
			Path:        models.CompletedBuildingPath,
		}, nil
	}

	h := handler.NewBuildingHandler(&mockBuildingPort{}, m)
	resp, err := h.ListCompletedBuildings(context.Background(), &pb.ListCompletedBuildingsRequest{Page: 0})
	require.NoError(t, err)
	assert.Empty(t, resp.Data)
	assert.Equal(t, int32(0), resp.Meta.Total)
}
