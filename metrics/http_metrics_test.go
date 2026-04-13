package metrics

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPMetrics(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	// Create a test handler
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Make a request
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	
	// Check response
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	
	// Check metrics
	metrics := registry.GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be recorded")
	}
	
	// Check for specific metrics
	var foundRequestsTotal, foundDuration bool
	for _, metric := range metrics {
		if metric.Name == "http_requests_total" {
			foundRequestsTotal = true
			if metric.Value != 1 {
				t.Errorf("Expected http_requests_total value 1, got %f", metric.Value)
			}
		}
		if metric.Name == "http_request_duration_seconds" {
			foundDuration = true
			if metric.Extra == nil {
				t.Error("Expected duration metric to have extra data")
			}
		}
	}
	
	if !foundRequestsTotal {
		t.Error("Expected to find http_requests_total metric")
	}
	
	if !foundDuration {
		t.Error("Expected to find http_request_duration_seconds metric")
	}
}

func TestHTTPMetricsWithLabels(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	// Create a test handler that returns different status codes
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	
	// Make successful request
	req1 := httptest.NewRequest("GET", "/test", nil)
	recorder1 := httptest.NewRecorder()
	handler.ServeHTTP(recorder1, req1)
	
	// Make error request
	req2 := httptest.NewRequest("POST", "/error", nil)
	recorder2 := httptest.NewRecorder()
	handler.ServeHTTP(recorder2, req2)
	
	// Check metrics
	metrics := registry.GetAllMetrics()
	
	var foundGET200, foundPOST500 bool
	for _, metric := range metrics {
		if metric.Name == "http_requests_total" {
			if metric.Labels != nil {
				method := metric.Labels["method"]
				status := metric.Labels["status"]
				
				if method == "GET" && status == "200" {
					foundGET200 = true
				}
				if method == "POST" && status == "500" {
					foundPOST500 = true
				}
			}
		}
	}
	
	if !foundGET200 {
		t.Error("Expected to find GET 200 metric")
	}
	
	if !foundPOST500 {
		t.Error("Expected to find POST 500 metric")
	}
}

func TestHTTPMetricsActiveRequests(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	// Create a test handler that takes some time
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Make concurrent requests
	done := make(chan bool, 3)
	
	for i := 0; i < 3; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
			done <- true
		}()
	}
	
	// Wait a bit and check active requests
	time.Sleep(5 * time.Millisecond)
	
	// Check if active requests gauge is working
	metrics := httpMetrics.GetMetrics()
	activeRequests := metrics["active_requests"]
	
	// The exact value depends on timing, but should be reasonable
	if activeRequests.Value < 0 || activeRequests.Value > 3 {
		t.Errorf("Expected active requests between 0 and 3, got %f", activeRequests.Value)
	}
	
	// Wait for all requests to complete
	for i := 0; i < 3; i++ {
		<-done
	}
	
	// Check that active requests returns to 0
	time.Sleep(5 * time.Millisecond)
	finalMetrics := httpMetrics.GetMetrics()
	finalActiveRequests := finalMetrics["active_requests"]
	
	if finalActiveRequests.Value != 0 {
		t.Errorf("Expected final active requests 0, got %f", finalActiveRequests.Value)
	}
}

func TestHTTPMetricsRequestSize(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Create request with body
	body := strings.NewReader("test request body")
	req := httptest.NewRequest("POST", "/test", body)
	req.Header.Set("Content-Length", "17") // Length of "test request body"
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	// Check request size metric
	metrics := httpMetrics.GetMetrics()
	requestSize := metrics["request_size"]
	
	if requestSize.Extra == nil {
		t.Error("Expected request size metric to have extra data")
	}
	
	// Check that we recorded at least one observation
	if count, ok := requestSize.Extra["count"].(uint64); !ok || count == 0 {
		t.Error("Expected request size metric to have recorded observations")
	}
}

func TestHTTPMetricsResponseSize(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	responseBody := "This is a longer response body to test response size metrics"
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	// Check response size metric
	metrics := httpMetrics.GetMetrics()
	responseSize := metrics["response_size"]
	
	if responseSize.Extra == nil {
		t.Error("Expected response size metric to have extra data")
	}
	
	// Check that we recorded at least one observation
	if count, ok := responseSize.Extra["count"].(uint64); !ok || count == 0 {
		t.Error("Expected response size metric to have recorded observations")
	}
}

func TestHTTPMetricsMiddlewareFunc(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	// Test the MiddlewareFunc wrapper
	baseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
	
	wrappedHandler := httpMetrics.MiddlewareFunc(baseHandler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	
	wrappedHandler(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	
	// Check that metrics were recorded
	metrics := registry.GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be recorded with MiddlewareFunc")
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"/api/users", "/api/users"},
		{"/api/users/123", "/api/users/123"},
		{"/very/long/path/that/exceeds/the/fifty/character/limit/and/should/be/truncated", "/long_path"},
	}
	
	for _, test := range tests {
		result := sanitizePath(test.input)
		if result != test.expected {
			t.Errorf("sanitizePath(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestDefaultHTTPMetrics(t *testing.T) {
	// Reset default registry
	Reset()
	
	// Test default middleware
	handler := Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	
	// Check that default metrics were recorded
	metrics := GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected default metrics to be recorded")
	}
	
	// Test default GetHTTPMetrics
	httpMetrics := GetHTTPMetrics()
	if len(httpMetrics) == 0 {
		t.Error("Expected default HTTP metrics to be available")
	}
}

func TestDefaultMiddlewareFunc(t *testing.T) {
	// Reset default registry
	Reset()
	
	baseHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
	
	wrappedHandler := MiddlewareFunc(baseHandler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	
	wrappedHandler(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	
	// Check that metrics were recorded
	metrics := GetAllMetrics()
	if len(metrics) == 0 {
		t.Error("Expected metrics to be recorded with default MiddlewareFunc")
	}
}

func TestHTTPMetricsMultipleRequests(t *testing.T) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	// Make multiple requests
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", fmt.Sprintf("/test/%d", i), nil)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
	
	// Check metrics
	metrics := httpMetrics.GetMetrics()
	
	// Check requests total
	requestsTotal := metrics["requests_total"]
	if requestsTotal.Value != 5 {
		t.Errorf("Expected requests total 5, got %f", requestsTotal.Value)
	}
	
	// Check request duration
	requestDuration := metrics["request_duration"]
	if count, ok := requestDuration.Extra["count"].(uint64); !ok || count != 5 {
		t.Errorf("Expected request duration count 5, got %v", requestDuration.Extra["count"])
	}
}

func BenchmarkHTTPMetricsMiddleware(b *testing.B) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	req := httptest.NewRequest("GET", "/test", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
}

func BenchmarkHTTPMetricsMiddlewareParallel(b *testing.B) {
	registry := NewRegistry()
	httpMetrics := NewHTTPMetrics(registry)
	
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/test", nil)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)
		}
	})
}