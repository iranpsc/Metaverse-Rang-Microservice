package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"metarang/features-service/internal/models"
	"metarang/features-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTradeHistoryFeatureRepo struct {
	feature    *models.Feature
	properties *models.FeatureProperties
	err        error
}

func (m *mockTradeHistoryFeatureRepo) FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	if m.feature == nil {
		return nil, nil, sql.ErrNoRows
	}
	if m.feature.ID != id {
		return nil, nil, sql.ErrNoRows
	}
	return m.feature, m.properties, nil
}

type mockTradeHistoryTradeRepo struct {
	trades       []models.TradeHistoryTrade
	systemUserID uint64
	systemErr    error
	listErr      error
}

func (m *mockTradeHistoryTradeRepo) ListByFeatureWithDetails(ctx context.Context, featureID uint64) ([]models.TradeHistoryTrade, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	out := make([]models.TradeHistoryTrade, 0, len(m.trades))
	for _, t := range m.trades {
		if t.FeatureID == featureID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockTradeHistoryTradeRepo) FindSystemUserID(ctx context.Context) (uint64, error) {
	if m.systemErr != nil {
		return 0, m.systemErr
	}
	return m.systemUserID, nil
}

func newTradeHistoryService(
	featureRepo *mockTradeHistoryFeatureRepo,
	tradeRepo *mockTradeHistoryTradeRepo,
) *service.FeatureTradeHistoryService {
	return service.NewFeatureTradeHistoryService(featureRepo, tradeRepo)
}

func sampleFeature(ownerID uint64, createdAt time.Time) *models.Feature {
	return &models.Feature{
		ID:        10,
		OwnerID:   ownerID,
		CreatedAt: createdAt,
	}
}

func sampleProperties(karbari string, stability float64) *models.FeatureProperties {
	return &models.FeatureProperties{
		ID:        "P-10",
		FeatureID: 10,
		Karbari:   karbari,
		Stability: stability,
	}
}

func TestFeatureTradeHistoryService_NotFound(t *testing.T) {
	svc := newTradeHistoryService(&mockTradeHistoryFeatureRepo{}, &mockTradeHistoryTradeRepo{})
	_, err := svc.Paginate(context.Background(), 99, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, models.ErrFeatureNotFound)
}

func TestFeatureTradeHistoryService_GenesisOnly(t *testing.T) {
	createdAt := time.Date(2022, 11, 1, 9, 0, 0, 0, time.UTC) // 1401/08/10
	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{feature: sampleFeature(7, createdAt)},
		&mockTradeHistoryTradeRepo{systemUserID: 1},
	)

	page, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, 1, page.Total)
	assert.Equal(t, 1, page.LastPage)
	assert.Equal(t, models.TradeHistoryPerPage, page.PerPage)

	item := page.Items[0]
	assert.Nil(t, item.ID)
	assert.Equal(t, models.TradeHistoryTypeGenesis, item.Type)
	assert.Nil(t, item.ParticipantCode)
	assert.Equal(t, models.SystemOwnerLabel, item.ParticipantLabel)
	assert.Equal(t, models.TradeHistoryPriceCurrency, item.Price.Type)
	require.NotNil(t, item.Price.PricePSC)
	require.NotNil(t, item.Price.PriceIRR)
	assert.Equal(t, int64(0), *item.Price.PricePSC)
	assert.Equal(t, int64(0), *item.Price.PriceIRR)
	assert.Nil(t, item.Price.Color)
	assert.Equal(t, "آبان", item.DateTime.MonthName)
	assert.Equal(t, 1401, item.DateTime.Year)
	assert.Equal(t, "09:00:00", item.DateTime.Time)
	assert.Equal(t, "آبان 1401 | 09:00:00", item.DateTime.Formatted)
}

func TestFeatureTradeHistoryService_RGBPurchaseFallsBackToFeatureProperties(t *testing.T) {
	// RGB / system purchases historically only move wallet balances and leave
	// trades without linked color withdraw transactions.
	createdAt := time.Date(2022, 2, 4, 10, 28, 24, 0, time.UTC) // ~1400/11/15-ish genesis style
	tradeAt := time.Date(2026, 9, 17, 7, 34, 43, 0, time.UTC)
	systemUserID := uint64(1)

	trades := []models.TradeHistoryTrade{
		{
			ID:        287,
			FeatureID: 10,
			BuyerID:   491,
			SellerID:  systemUserID,
			PSCAmount: 0,
			IRRAmount: 0,
			CreatedAt: sql.NullTime{Time: tradeAt, Valid: true},
			BuyerCode: "hm-2000491",
			BuyerName: "Parsa",
			// No Transactions — reproduces the null color_* bug.
		},
	}

	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{
			feature:    sampleFeature(491, createdAt),
			properties: sampleProperties("t", 1250), // tejari → red
		},
		&mockTradeHistoryTradeRepo{systemUserID: systemUserID, trades: trades},
	)

	page, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)

	item := page.Items[0]
	require.NotNil(t, item.ID)
	assert.Equal(t, uint64(287), *item.ID)
	assert.Equal(t, models.TradeHistoryTypeTrade, item.Type)
	require.NotNil(t, item.ParticipantCode)
	assert.Equal(t, "HM-2000491", *item.ParticipantCode)
	assert.Equal(t, "Parsa", item.ParticipantLabel)
	assert.Equal(t, models.TradeHistoryPriceColor, item.Price.Type)
	assert.Nil(t, item.Price.PricePSC)
	assert.Nil(t, item.Price.PriceIRR)
	require.NotNil(t, item.Price.Color)
	assert.Equal(t, "red", *item.Price.Color)
	require.NotNil(t, item.Price.ColorName)
	assert.Equal(t, "قرمز", *item.Price.ColorName)
	require.NotNil(t, item.Price.ColorAmount)
	assert.Equal(t, int64(1250), *item.Price.ColorAmount)

	assert.Equal(t, models.TradeHistoryTypeGenesis, page.Items[1].Type)
}

func TestFeatureTradeHistoryService_SortDescendingAndCurrencyVsColor(t *testing.T) {
	createdAt := time.Date(2022, 11, 1, 9, 0, 0, 0, time.UTC)
	systemUserID := uint64(1)
	older := time.Date(2026, 1, 5, 15, 37, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 2, 12, 16, 0, 0, time.UTC)

	trades := []models.TradeHistoryTrade{
		{
			ID:        42,
			FeatureID: 10,
			BuyerID:   3,
			SellerID:  81,
			PSCAmount: 2500000,
			IRRAmount: 100000,
			CreatedAt: sql.NullTime{Time: newer, Valid: true},
			BuyerCode: "hm-2000003",
			BuyerName: "کاربر فعلی",
		},
		{
			ID:        41,
			FeatureID: 10,
			BuyerID:   81,
			SellerID:  systemUserID,
			PSCAmount: 0,
			IRRAmount: 0,
			CreatedAt: sql.NullTime{Time: older, Valid: true},
			BuyerCode: "hm-2000081",
			BuyerName: "خریدار اول",
			Transactions: []models.TradeHistoryTransaction{
				{Asset: "red", Amount: 250, Action: "withdraw"},
			},
		},
	}

	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{
			feature:    sampleFeature(3, createdAt),
			properties: sampleProperties("t", 250),
		},
		&mockTradeHistoryTradeRepo{systemUserID: systemUserID, trades: trades},
	)

	page, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 3)
	assert.Equal(t, 3, page.Total)

	// Newest trade first — P2P currency
	assert.Equal(t, models.TradeHistoryTypeTrade, page.Items[0].Type)
	require.NotNil(t, page.Items[0].ID)
	assert.Equal(t, uint64(42), *page.Items[0].ID)
	require.NotNil(t, page.Items[0].ParticipantCode)
	assert.Equal(t, "HM-2000003", *page.Items[0].ParticipantCode)
	assert.Equal(t, "کاربر فعلی", page.Items[0].ParticipantLabel)
	assert.Equal(t, models.TradeHistoryPriceCurrency, page.Items[0].Price.Type)
	require.NotNil(t, page.Items[0].Price.PricePSC)
	assert.Equal(t, int64(2500000), *page.Items[0].Price.PricePSC)
	require.NotNil(t, page.Items[0].Price.PriceIRR)
	assert.Equal(t, int64(100000), *page.Items[0].Price.PriceIRR)
	assert.Nil(t, page.Items[0].Price.Color)
	assert.Nil(t, page.Items[0].Price.ColorName)
	assert.Nil(t, page.Items[0].Price.ColorAmount)

	// Older system purchase uses color price from linked transactions
	assert.Equal(t, models.TradeHistoryTypeTrade, page.Items[1].Type)
	require.NotNil(t, page.Items[1].ID)
	assert.Equal(t, uint64(41), *page.Items[1].ID)
	require.NotNil(t, page.Items[1].ParticipantCode)
	assert.Equal(t, "HM-2000081", *page.Items[1].ParticipantCode)
	assert.Equal(t, models.TradeHistoryPriceColor, page.Items[1].Price.Type)
	assert.Nil(t, page.Items[1].Price.PricePSC)
	assert.Nil(t, page.Items[1].Price.PriceIRR)
	require.NotNil(t, page.Items[1].Price.Color)
	assert.Equal(t, "red", *page.Items[1].Price.Color)
	require.NotNil(t, page.Items[1].Price.ColorName)
	assert.Equal(t, "قرمز", *page.Items[1].Price.ColorName)
	require.NotNil(t, page.Items[1].Price.ColorAmount)
	assert.Equal(t, int64(250), *page.Items[1].Price.ColorAmount)

	// Genesis last
	assert.Equal(t, models.TradeHistoryTypeGenesis, page.Items[2].Type)
	assert.Nil(t, page.Items[2].ID)
}

func TestFeatureTradeHistoryService_ColorViaTransactionsWithoutSystemSeller(t *testing.T) {
	createdAt := time.Date(2022, 11, 1, 9, 0, 0, 0, time.UTC)
	tradeAt := time.Date(2026, 1, 5, 15, 37, 0, 0, time.UTC)
	trades := []models.TradeHistoryTrade{
		{
			ID:        50,
			FeatureID: 10,
			BuyerID:   7,
			SellerID:  99,
			PSCAmount: 0,
			IRRAmount: 0,
			CreatedAt: sql.NullTime{Time: tradeAt, Valid: true},
			BuyerCode: "hm-2000007",
			BuyerName: "خریدار",
			Transactions: []models.TradeHistoryTransaction{
				{Asset: "blue", Amount: 100, Action: "withdraw"},
			},
		},
	}

	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{
			feature:    sampleFeature(7, createdAt),
			properties: sampleProperties("a", 999), // fallback must NOT override linked tx
		},
		&mockTradeHistoryTradeRepo{systemUserID: 1, trades: trades},
	)

	page, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(page.Items), 1)
	assert.Equal(t, models.TradeHistoryPriceColor, page.Items[0].Price.Type)
	require.NotNil(t, page.Items[0].Price.Color)
	assert.Equal(t, "blue", *page.Items[0].Price.Color)
	require.NotNil(t, page.Items[0].Price.ColorName)
	assert.Equal(t, "آبی", *page.Items[0].Price.ColorName)
	require.NotNil(t, page.Items[0].Price.ColorAmount)
	assert.Equal(t, int64(100), *page.Items[0].Price.ColorAmount)
}

func TestFeatureTradeHistoryService_UsesDateWhenCreatedAtMissing(t *testing.T) {
	featureCreated := time.Date(2022, 11, 1, 9, 0, 0, 0, time.UTC)
	tradeDate := time.Date(2025, 3, 20, 18, 0, 0, 0, time.UTC)
	trades := []models.TradeHistoryTrade{
		{
			ID:        60,
			FeatureID: 10,
			BuyerID:   7,
			SellerID:  8,
			PSCAmount: 100,
			IRRAmount: 50,
			Date:      sql.NullTime{Time: tradeDate, Valid: true},
			BuyerCode: "hm-7",
			BuyerName: "Buyer",
		},
	}

	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{feature: sampleFeature(7, featureCreated)},
		&mockTradeHistoryTradeRepo{systemUserID: 1, trades: trades},
	)

	page, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(page.Items), 1)
	assert.Equal(t, "00:00:00", page.Items[0].DateTime.Time)
}

func TestFeatureTradeHistoryService_PaginationGenesisOnLastPage(t *testing.T) {
	createdAt := time.Date(2022, 11, 1, 9, 0, 0, 0, time.UTC)
	trades := make([]models.TradeHistoryTrade, 0, 12)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 12; i++ {
		id := uint64(100 + i)
		trades = append(trades, models.TradeHistoryTrade{
			ID:        id,
			FeatureID: 10,
			BuyerID:   7,
			SellerID:  8,
			PSCAmount: float64(i + 1),
			CreatedAt: sql.NullTime{Time: base.Add(time.Duration(i) * time.Hour), Valid: true},
			BuyerCode: "hm-7",
			BuyerName: "Owner",
		})
	}

	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{feature: sampleFeature(7, createdAt)},
		&mockTradeHistoryTradeRepo{systemUserID: 1, trades: trades},
	)

	page1, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	assert.Equal(t, 13, page1.Total) // 12 trades + genesis
	assert.Equal(t, 2, page1.LastPage)
	require.Len(t, page1.Items, 10)
	assert.Equal(t, models.TradeHistoryTypeTrade, page1.Items[0].Type)
	for _, item := range page1.Items {
		assert.NotEqual(t, models.TradeHistoryTypeGenesis, item.Type)
	}
	require.NotNil(t, page1.From)
	require.NotNil(t, page1.To)
	assert.Equal(t, 1, *page1.From)
	assert.Equal(t, 10, *page1.To)

	page2, err := svc.Paginate(context.Background(), 10, 2)
	require.NoError(t, err)
	require.Len(t, page2.Items, 3) // 2 trades + genesis
	assert.Equal(t, models.TradeHistoryTypeGenesis, page2.Items[len(page2.Items)-1].Type)
	require.NotNil(t, page2.From)
	require.NotNil(t, page2.To)
	assert.Equal(t, 11, *page2.From)
	assert.Equal(t, 13, *page2.To)
}

func TestFeatureTradeHistoryService_SystemUserLookupFailureStillWorks(t *testing.T) {
	createdAt := time.Date(2022, 11, 1, 9, 0, 0, 0, time.UTC)
	svc := newTradeHistoryService(
		&mockTradeHistoryFeatureRepo{feature: sampleFeature(7, createdAt)},
		&mockTradeHistoryTradeRepo{systemErr: errors.New("missing system user")},
	)

	page, err := svc.Paginate(context.Background(), 10, 1)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, models.TradeHistoryTypeGenesis, page.Items[0].Type)
}
