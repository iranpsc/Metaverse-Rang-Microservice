// Package metrics provides Prometheus metrics for the features service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// MarketplaceMetrics holds Prometheus metrics for marketplace operations
type MarketplaceMetrics struct {
	BuyRequestsTotal  *prometheus.CounterVec
	SellRequestsTotal prometheus.Counter
	TradesTotal       *prometheus.CounterVec
	TradeValuePSC     prometheus.Histogram
	TradeValueIRR     prometheus.Histogram
	LockedAssetsPSC   prometheus.Gauge
	LockedAssetsIRR   prometheus.Gauge
}

// NewMarketplaceMetrics creates a new marketplace metrics instance
func NewMarketplaceMetrics() *MarketplaceMetrics {
	return &MarketplaceMetrics{
		BuyRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "buy_requests_total",
				Help:      "Total number of buy requests",
			},
			[]string{"status"}, // status: accepted, rejected, cancelled
		),
		SellRequestsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "sell_requests_total",
				Help:      "Total number of sell requests",
			},
		),
		TradesTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "trades_total",
				Help:      "Total number of trades",
			},
			[]string{"type"}, // type: limited, rgb, user
		),
		TradeValuePSC: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "trade_value_psc",
				Help:      "Trade value in PSC",
				Buckets:   prometheus.ExponentialBuckets(0.1, 10, 6), // 0.1 to 100000
			},
		),
		TradeValueIRR: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "trade_value_irr",
				Help:      "Trade value in IRR",
				Buckets:   prometheus.ExponentialBuckets(1000, 10, 6), // 1000 to 1,000,000,000
			},
		),
		LockedAssetsPSC: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "buy_request_locked_assets_psc",
				Help:      "Total locked PSC assets in buy requests",
			},
		),
		LockedAssetsIRR: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "metarang",
				Subsystem: "features",
				Name:      "buy_request_locked_assets_irr",
				Help:      "Total locked IRR assets in buy requests",
			},
		),
	}
}

// RecordBuyRequest records a buy request with status
func (m *MarketplaceMetrics) RecordBuyRequest(status string) {
	m.BuyRequestsTotal.WithLabelValues(status).Inc()
}

// RecordSellRequest records a sell request
func (m *MarketplaceMetrics) RecordSellRequest() {
	m.SellRequestsTotal.Inc()
}

// RecordTrade records a trade with type and values
func (m *MarketplaceMetrics) RecordTrade(tradeType string, pscAmount, irrAmount float64) {
	m.TradesTotal.WithLabelValues(tradeType).Inc()
	if pscAmount > 0 {
		m.TradeValuePSC.Observe(pscAmount)
	}
	if irrAmount > 0 {
		m.TradeValueIRR.Observe(irrAmount)
	}
}

// UpdateLockedAssets updates the locked assets gauge
func (m *MarketplaceMetrics) UpdateLockedAssets(pscAmount, irrAmount float64) {
	m.LockedAssetsPSC.Set(pscAmount)
	m.LockedAssetsIRR.Set(irrAmount)
}
