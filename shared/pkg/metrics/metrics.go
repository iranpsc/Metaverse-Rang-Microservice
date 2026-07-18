// Package metrics exposes Prometheus metrics registration helpers.
package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Metrics holds Prometheus metrics for a service
type Metrics struct {
	RequestCounter   *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight *prometheus.GaugeVec
	DBConnPoolStats  *prometheus.GaugeVec
}

// NewMetrics creates a new metrics instance
func NewMetrics(serviceName string) *Metrics {
	return &Metrics{
		RequestCounter: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "metarang",
				Subsystem: serviceName,
				Name:      "requests_total",
				Help:      "Total number of requests",
			},
			[]string{"method", "status"},
		),
		RequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "metarang",
				Subsystem: serviceName,
				Name:      "request_duration_seconds",
				Help:      "Request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method"},
		),
		RequestsInFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "metarang",
				Subsystem: serviceName,
				Name:      "requests_in_flight",
				Help:      "Number of requests currently being processed",
			},
			[]string{"method"},
		),
		DBConnPoolStats: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "metarang",
				Subsystem: serviceName,
				Name:      "db_connection_pool",
				Help:      "Database connection pool statistics",
			},
			[]string{"stat"}, // stat can be: open, in_use, idle, wait_count, etc.
		),
	}
}

// UnaryServerInterceptor returns a new unary server interceptor for metrics
func UnaryServerInterceptor(metrics *Metrics) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		method := info.FullMethod

		// Track requests in flight
		metrics.RequestsInFlight.WithLabelValues(method).Inc()
		defer metrics.RequestsInFlight.WithLabelValues(method).Dec()

		// Track request duration
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			metrics.RequestDuration.WithLabelValues(method).Observe(duration)
		}()

		// Call handler
		resp, err := handler(ctx, req)

		// Track request count with status
		statusCode := "ok"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}
		metrics.RequestCounter.WithLabelValues(method, statusCode).Inc()

		return resp, err
	}
}

// StreamServerInterceptor returns a new stream server interceptor for metrics
func StreamServerInterceptor(metrics *Metrics) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		method := info.FullMethod

		// Track requests in flight
		metrics.RequestsInFlight.WithLabelValues(method).Inc()
		defer metrics.RequestsInFlight.WithLabelValues(method).Dec()

		// Track request duration
		start := time.Now()
		defer func() {
			duration := time.Since(start).Seconds()
			metrics.RequestDuration.WithLabelValues(method).Observe(duration)
		}()

		// Call handler
		err := handler(srv, stream)

		// Track request count with status
		statusCode := "ok"
		if err != nil {
			st, _ := status.FromError(err)
			statusCode = st.Code().String()
		}
		metrics.RequestCounter.WithLabelValues(method, statusCode).Inc()

		return err
	}
}

// RecordDBPoolStats records database connection pool statistics
func (m *Metrics) RecordDBPoolStats(open, inUse, idle int, waitCount int64, waitDuration time.Duration) {
	m.DBConnPoolStats.WithLabelValues("open").Set(float64(open))
	m.DBConnPoolStats.WithLabelValues("in_use").Set(float64(inUse))
	m.DBConnPoolStats.WithLabelValues("idle").Set(float64(idle))
	m.DBConnPoolStats.WithLabelValues("wait_count").Set(float64(waitCount))
	m.DBConnPoolStats.WithLabelValues("wait_duration_ms").Set(float64(waitDuration.Milliseconds()))
}
