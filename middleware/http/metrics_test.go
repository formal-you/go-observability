package httpmw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsRecordsDuration(t *testing.T) {
	_, reader := newTestMeter(t)
	handler := Metrics(MetricsConfig{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	points := collectHistogram(t, reader, metricHTTPServerRequestDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if metricAttrString(t, points[0].Attributes, "http.request.method") != "GET" {
		t.Fatalf("method attr = %s, want GET", metricAttrString(t, points[0].Attributes, "http.request.method"))
	}
	if metricAttrInt(t, points[0].Attributes, "http.response.status_code") != http.StatusOK {
		t.Fatalf("status attr = %d, want 200", metricAttrInt(t, points[0].Attributes, "http.response.status_code"))
	}
	if points[0].Count != 1 {
		t.Fatalf("count = %d, want 1", points[0].Count)
	}
}

func TestMetricsRouteFromMuxPattern(t *testing.T) {
	_, reader := newTestMeter(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Metrics(MetricsConfig{})(mux)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))

	points := collectHistogram(t, reader, metricHTTPServerRequestDuration)
	if len(points) != 1 {
		t.Fatalf("data points = %d, want 1", len(points))
	}
	if got := metricAttrString(t, points[0].Attributes, "http.route"); got != "GET /ok" {
		t.Fatalf("http.route = %q, want GET /ok（ServeMux r.Pattern）", got)
	}
}
