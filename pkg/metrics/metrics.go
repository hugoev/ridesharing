// Package metrics provides Prometheus instrumentation for HTTP services.
package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// httpRequestsTotal counts total HTTP requests by method, path, and status.
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// httpRequestDuration tracks request latency in seconds.
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency distribution.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// httpResponseSize tracks response size in bytes.
	httpResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_response_size_bytes",
			Help:    "HTTP response size distribution.",
			Buckets: prometheus.ExponentialBuckets(100, 10, 6), // 100B, 1KB, 10KB, 100KB, 1MB, 10MB
		},
		[]string{"method", "path"},
	)

	// activeConnections tracks in-flight requests.
	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_connections",
			Help: "Number of active HTTP connections.",
		},
	)

	// --- Business metrics ---

	// RidesTotal counts ride requests by status.
	RidesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rides_total",
			Help: "Total ride requests by final status.",
		},
		[]string{"status"},
	)

	// MatchDuration tracks how long ride matching takes.
	MatchDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "ride_match_duration_seconds",
			Help:    "Time to match a ride with a driver.",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
	)

	// PaymentAmount tracks payment values.
	PaymentAmount = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "payment_amount_usd",
			Help:    "Payment amounts in USD.",
			Buckets: []float64{5, 10, 15, 20, 30, 50, 75, 100, 200},
		},
	)

	// SurgeMultiplier tracks the current surge multiplier.
	SurgeMultiplier = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ride_surge_multiplier",
			Help: "Current surge pricing multiplier.",
		},
	)
)

// Middleware returns a Gin middleware that records Prometheus metrics.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		activeConnections.Inc()

		c.Next()

		activeConnections.Dec()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path // fallback for unmatched routes
		}

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
		httpResponseSize.WithLabelValues(c.Request.Method, path).Observe(float64(c.Writer.Size()))
	}
}

// Handler returns an HTTP handler for the /metrics endpoint.
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
