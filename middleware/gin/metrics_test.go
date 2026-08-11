package ginmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetricsRecordsDuration(t *testing.T) {
	_, reader := newTestMeter(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(Metrics(MetricsConfig{}))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

	points := collectHistogram(t, reader, metricHTTPServerRequestDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if metricAttrString(t, points[0].Attributes, "http.route") != "/ok" {
		t.Fatalf("http.route = %q, want /ok", metricAttrString(t, points[0].Attributes, "http.route"))
	}
	if metricAttrInt(t, points[0].Attributes, "http.response.status_code") != http.StatusOK {
		t.Fatalf("status attr = %d, want 200", metricAttrInt(t, points[0].Attributes, "http.response.status_code"))
	}
}
