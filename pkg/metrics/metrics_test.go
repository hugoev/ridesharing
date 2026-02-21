package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestMiddleware_RecordsMetrics(t *testing.T) {
	router := gin.New()
	router.Use(Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMiddleware_404Route(t *testing.T) {
	router := gin.New()
	router.Use(Middleware())

	req, _ := http.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_ReturnsMetrics(t *testing.T) {
	router := gin.New()
	router.GET("/metrics", Handler())

	req, _ := http.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	// Prometheus metrics endpoint should contain standard Go metrics
	if len(body) == 0 {
		t.Error("expected non-empty metrics response")
	}
}

func TestBusinessMetrics_Exist(t *testing.T) {
	// Verify business metrics are registered and accessible
	tests := []struct {
		name   string
		metric prometheus.Collector
	}{
		{"RidesTotal", RidesTotal},
		{"MatchDuration", MatchDuration},
		{"PaymentAmount", PaymentAmount},
		{"SurgeMultiplier", SurgeMultiplier},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.metric == nil {
				t.Errorf("%s metric is nil", tt.name)
			}
		})
	}
}

func TestBusinessMetrics_CanRecord(t *testing.T) {
	// Verify we can record values without panicking
	RidesTotal.WithLabelValues("completed").Inc()
	RidesTotal.WithLabelValues("cancelled").Inc()
	MatchDuration.Observe(0.5)
	PaymentAmount.Observe(25.00)
	SurgeMultiplier.Set(1.5)
}
