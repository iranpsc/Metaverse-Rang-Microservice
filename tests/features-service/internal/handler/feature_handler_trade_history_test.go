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

type mockTradeHistoryPort struct {
	paginate func(ctx context.Context, featureID uint64, page int) (*models.TradeHistoryPage, error)
}

func (m *mockTradeHistoryPort) Paginate(ctx context.Context, featureID uint64, page int) (*models.TradeHistoryPage, error) {
	if m.paginate != nil {
		return m.paginate(ctx, featureID, page)
	}
	return nil, errors.New("not implemented")
}

func TestFeatureHandler_GetFeatureTradeHistory_MissingFeatureID(t *testing.T) {
	h := handler.NewFeatureHandler(&mockFeaturePort{}, &mockTradeHistoryPort{})
	_, err := h.GetFeatureTradeHistory(context.Background(), &pb.GetFeatureTradeHistoryRequest{})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestFeatureHandler_GetFeatureTradeHistory_NotFound(t *testing.T) {
	m := &mockTradeHistoryPort{}
	m.paginate = func(ctx context.Context, featureID uint64, page int) (*models.TradeHistoryPage, error) {
		return nil, models.ErrFeatureNotFound
	}
	h := handler.NewFeatureHandler(&mockFeaturePort{}, m)
	_, err := h.GetFeatureTradeHistory(context.Background(), &pb.GetFeatureTradeHistoryRequest{FeatureId: 9, Page: 1})
	st, _ := status.FromError(err)
	assert.Equal(t, codes.NotFound, st.Code())
}

func TestFeatureHandler_GetFeatureTradeHistory_Success(t *testing.T) {
	zero := int64(0)
	code := "HM-2000003"
	id := uint64(42)
	from, to := 1, 1
	m := &mockTradeHistoryPort{}
	m.paginate = func(ctx context.Context, featureID uint64, page int) (*models.TradeHistoryPage, error) {
		assert.Equal(t, uint64(10), featureID)
		assert.Equal(t, 2, page)
		return &models.TradeHistoryPage{
			Items: []models.TradeHistoryItem{
				{
					ID:               &id,
					Type:             models.TradeHistoryTypeTrade,
					ParticipantCode:  &code,
					ParticipantLabel: "کاربر",
					DateTime: models.TradeHistoryDateTime{
						Date:      "1405/02/12",
						MonthName: "اردیبهشت",
						Year:      1405,
						Time:      "12:16:00",
						Formatted: "اردیبهشت 1405 | 12:16:00",
					},
					Price: models.TradeHistoryPrice{
						Type:     models.TradeHistoryPriceCurrency,
						PricePSC: &zero,
						PriceIRR: &zero,
					},
				},
			},
			CurrentPage: 2,
			PerPage:     10,
			Total:       1,
			LastPage:    1,
			From:        &from,
			To:          &to,
			Path:        "/api/features/10/trade-history",
		}, nil
	}

	h := handler.NewFeatureHandler(&mockFeaturePort{}, m)
	resp, err := h.GetFeatureTradeHistory(context.Background(), &pb.GetFeatureTradeHistoryRequest{FeatureId: 10, Page: 2})
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, uint64(42), resp.Data[0].GetId())
	assert.Equal(t, "HM-2000003", resp.Data[0].GetParticipantCode())
	assert.Equal(t, "trade", resp.Data[0].Type)
	assert.Equal(t, int32(2), resp.Meta.CurrentPage)
	assert.Equal(t, "/api/features/10/trade-history?page=1", resp.Links.First)
	assert.Contains(t, resp.Links.Prev, "page=1")
}

func TestFeatureHandler_GetFeatureTradeHistory_MapsColorAndCurrencyPrices(t *testing.T) {
	tradeID := uint64(287)
	code := "HM-2000491"
	color := "red"
	colorName := "قرمز"
	colorAmount := int64(1250)
	psc := int64(2500000)
	irr := int64(100000)

	m := &mockTradeHistoryPort{}
	m.paginate = func(ctx context.Context, featureID uint64, page int) (*models.TradeHistoryPage, error) {
		return &models.TradeHistoryPage{
			Items: []models.TradeHistoryItem{
				{
					ID:               &tradeID,
					Type:             models.TradeHistoryTypeTrade,
					ParticipantCode:  &code,
					ParticipantLabel: "Parsa",
					DateTime: models.TradeHistoryDateTime{
						Date:      "1405/06/26",
						MonthName: "شهریور",
						Year:      1405,
						Time:      "11:04:43",
						Formatted: "شهریور 1405 | 11:04:43",
					},
					Price: models.TradeHistoryPrice{
						Type:        models.TradeHistoryPriceColor,
						Color:       &color,
						ColorName:   &colorName,
						ColorAmount: &colorAmount,
					},
				},
				{
					ID:               nil,
					Type:             models.TradeHistoryTypeGenesis,
					ParticipantCode:  nil,
					ParticipantLabel: models.SystemOwnerLabel,
					DateTime: models.TradeHistoryDateTime{
						Date:      "1401/11/15",
						MonthName: "بهمن",
						Year:      1401,
						Time:      "13:58:24",
						Formatted: "بهمن 1401 | 13:58:24",
					},
					Price: models.TradeHistoryPrice{
						Type:     models.TradeHistoryPriceCurrency,
						PricePSC: &psc,
						PriceIRR: &irr,
					},
				},
			},
			CurrentPage: 1,
			PerPage:     10,
			Total:       2,
			LastPage:    1,
			Path:        "/api/features/10/trade-history",
		}, nil
	}

	h := handler.NewFeatureHandler(&mockFeaturePort{}, m)
	resp, err := h.GetFeatureTradeHistory(context.Background(), &pb.GetFeatureTradeHistoryRequest{FeatureId: 10, Page: 1})
	require.NoError(t, err)
	require.Len(t, resp.Data, 2)

	rgbItem := resp.Data[0]
	require.NotNil(t, rgbItem.Id)
	assert.Equal(t, uint64(287), *rgbItem.Id)
	require.NotNil(t, rgbItem.ParticipantCode)
	assert.Equal(t, "HM-2000491", *rgbItem.ParticipantCode)
	require.NotNil(t, rgbItem.Price)
	assert.Equal(t, models.TradeHistoryPriceColor, rgbItem.Price.Type)
	require.NotNil(t, rgbItem.Price.Color)
	assert.Equal(t, "red", *rgbItem.Price.Color)
	require.NotNil(t, rgbItem.Price.ColorName)
	assert.Equal(t, "قرمز", *rgbItem.Price.ColorName)
	require.NotNil(t, rgbItem.Price.ColorAmount)
	assert.Equal(t, int64(1250), *rgbItem.Price.ColorAmount)
	assert.Nil(t, rgbItem.Price.PricePsc)
	assert.Nil(t, rgbItem.Price.PriceIrr)

	genesis := resp.Data[1]
	assert.Nil(t, genesis.Id)
	assert.Nil(t, genesis.ParticipantCode)
	assert.Equal(t, models.SystemOwnerLabel, genesis.ParticipantLabel)
	require.NotNil(t, genesis.Price)
	assert.Equal(t, models.TradeHistoryPriceCurrency, genesis.Price.Type)
	require.NotNil(t, genesis.Price.PricePsc)
	assert.Equal(t, int64(2500000), *genesis.Price.PricePsc)
	require.NotNil(t, genesis.Price.PriceIrr)
	assert.Equal(t, int64(100000), *genesis.Price.PriceIrr)
	assert.Nil(t, genesis.Price.Color)
}

func TestFeatureHandler_GetFeatureTradeHistory_DefaultPage(t *testing.T) {
	var gotPage int
	m := &mockTradeHistoryPort{}
	m.paginate = func(ctx context.Context, featureID uint64, page int) (*models.TradeHistoryPage, error) {
		gotPage = page
		return &models.TradeHistoryPage{
			Items:       []models.TradeHistoryItem{},
			CurrentPage: 1,
			PerPage:     10,
			Total:       0,
			LastPage:    1,
			Path:        "/api/features/1/trade-history",
		}, nil
	}
	h := handler.NewFeatureHandler(&mockFeaturePort{}, m)
	_, err := h.GetFeatureTradeHistory(context.Background(), &pb.GetFeatureTradeHistoryRequest{FeatureId: 1, Page: 0})
	require.NoError(t, err)
	assert.Equal(t, 1, gotPage)
}
