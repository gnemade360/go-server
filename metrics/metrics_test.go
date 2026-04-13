package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCounter(t *testing.T) {
	registry := NewRegistry()
	counter := registry.NewCounter("test_counter", "Test counter", nil)
	
	// Test initial value
	if counter.Get() != 0 {
		t.Errorf("Expected initial counter value 0, got %f", counter.Get())
	}
	
	// Test increment
	counter.Inc()
	if counter.Get() != 1 {
		t.Errorf("Expected counter value 1 after Inc(), got %f", counter.Get())
	}
	
	// Test add
	counter.Add(5)
	if counter.Get() != 6 {
		t.Errorf("Expected counter value 6 after Add(5), got %f", counter.Get())
	}
	
	// Test negative add (should be ignored)
	counter.Add(-2)
	if counter.Get() != 6 {
		t.Errorf("Expected counter value 6 after Add(-2), got %f", counter.Get())
	}
}

func TestCounterWithLabels(t *testing.T) {
	registry := NewRegistry()
	labels := map[string]string{"method": "GET", "status": "200"}
	counter := registry.NewCounter("http_requests", "HTTP requests", labels)
	
	counter.Inc()
	
	metric := counter.ToMetric()
	if metric.Type != MetricTypeCounter {
		t.Errorf("Expected metric type counter, got %s", metric.Type)
	}
	
	if metric.Labels["method"] != "GET" {
		t.Errorf("Expected method label GET, got %s", metric.Labels["method"])
	}
	
	if metric.Labels["status"] != "200" {
		t.Errorf("Expected status label 200, got %s", metric.Labels["status"])
	}
}

func TestGauge(t *testing.T) {
	registry := NewRegistry()
	gauge := registry.NewGauge("test_gauge", "Test gauge", nil)
	
	// Test initial value
	if gauge.Get() != 0 {
		t.Errorf("Expected initial gauge value 0, got %f", gauge.Get())
	}
	
	// Test set
	gauge.Set(10)
	if gauge.Get() != 10 {
		t.Errorf("Expected gauge value 10 after Set(10), got %f", gauge.Get())
	}
	
	// Test increment
	gauge.Inc()
	if gauge.Get() != 11 {
		t.Errorf("Expected gauge value 11 after Inc(), got %f", gauge.Get())
	}
	
	// Test decrement
	gauge.Dec()
	if gauge.Get() != 10 {
		t.Errorf("Expected gauge value 10 after Dec(), got %f", gauge.Get())
	}
	
	// Test add positive
	gauge.Add(5)
	if gauge.Get() != 15 {
		t.Errorf("Expected gauge value 15 after Add(5), got %f", gauge.Get())
	}
	
	// Test add negative
	gauge.Add(-3)
	if gauge.Get() != 12 {
		t.Errorf("Expected gauge value 12 after Add(-3), got %f", gauge.Get())
	}
}

func TestHistogram(t *testing.T) {
	registry := NewRegistry()
	buckets := []float64{1, 5, 10, 25, 50, 100}
	histogram := registry.NewHistogram("test_histogram", "Test histogram", nil, buckets)
	
	// Test initial state
	if histogram.GetCount() != 0 {
		t.Errorf("Expected initial histogram count 0, got %d", histogram.GetCount())
	}
	
	if histogram.GetSum() != 0 {
		t.Errorf("Expected initial histogram sum 0, got %f", histogram.GetSum())
	}
	
	if histogram.GetMean() != 0 {
		t.Errorf("Expected initial histogram mean 0, got %f", histogram.GetMean())
	}
	
	// Test observations
	histogram.Observe(3)
	histogram.Observe(7)
	histogram.Observe(15)
	
	if histogram.GetCount() != 3 {
		t.Errorf("Expected histogram count 3, got %d", histogram.GetCount())
	}
	
	expectedSum := 25.0
	if histogram.GetSum() != expectedSum {
		t.Errorf("Expected histogram sum %f, got %f", expectedSum, histogram.GetSum())
	}
	
	expectedMean := expectedSum / 3
	if histogram.GetMean() != expectedMean {
		t.Errorf("Expected histogram mean %f, got %f", expectedMean, histogram.GetMean())
	}
}

func TestHistogramBuckets(t *testing.T) {
	registry := NewRegistry()
	buckets := []float64{1, 5, 10}
	histogram := registry.NewHistogram("test_histogram", "Test histogram", nil, buckets)
	
	// Observe values that fall into different buckets
	histogram.Observe(0.5)  // bucket 1
	histogram.Observe(3)    // bucket 5
	histogram.Observe(7)    // bucket 10
	histogram.Observe(15)   // no bucket (higher than all)
	
	metric := histogram.ToMetric()
	counts, ok := metric.Extra["counts"].([]uint64)
	if !ok {
		t.Fatal("Expected counts in histogram metric extra")
	}
	
	if len(counts) != len(buckets) {
		t.Errorf("Expected %d bucket counts, got %d", len(buckets), len(counts))
	}
	
	// Check bucket counts
	expectedCounts := []uint64{1, 2, 3} // cumulative counts
	for i, expected := range expectedCounts {
		if counts[i] != expected {
			t.Errorf("Expected bucket %d count %d, got %d", i, expected, counts[i])
		}
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()
	
	// Create different types of metrics
	counter := registry.NewCounter("test_counter", "Test counter", nil)
	gauge := registry.NewGauge("test_gauge", "Test gauge", nil)
	histogram := registry.NewHistogram("test_histogram", "Test histogram", nil, []float64{1, 5, 10})
	
	// Update metrics
	counter.Inc()
	gauge.Set(42)
	histogram.Observe(7)
	
	// Get all metrics
	metrics := registry.GetAllMetrics()
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics, got %d", len(metrics))
	}
	
	// Check metrics are sorted by name
	expectedNames := []string{"test_counter", "test_gauge", "test_histogram"}
	for i, metric := range metrics {
		if metric.Name != expectedNames[i] {
			t.Errorf("Expected metric %d name %s, got %s", i, expectedNames[i], metric.Name)
		}
	}
}

func TestRegistryGetMetricsByType(t *testing.T) {
	registry := NewRegistry()
	
	registry.NewCounter("counter1", "Counter 1", nil).Inc()
	registry.NewCounter("counter2", "Counter 2", nil).Inc()
	registry.NewGauge("gauge1", "Gauge 1", nil).Set(10)
	registry.NewHistogram("histogram1", "Histogram 1", nil, []float64{1, 5}).Observe(3)
	
	counters := registry.GetMetricsByType(MetricTypeCounter)
	if len(counters) != 2 {
		t.Errorf("Expected 2 counters, got %d", len(counters))
	}
	
	gauges := registry.GetMetricsByType(MetricTypeGauge)
	if len(gauges) != 1 {
		t.Errorf("Expected 1 gauge, got %d", len(gauges))
	}
	
	histograms := registry.GetMetricsByType(MetricTypeHistogram)
	if len(histograms) != 1 {
		t.Errorf("Expected 1 histogram, got %d", len(histograms))
	}
}

func TestRegistryReset(t *testing.T) {
	registry := NewRegistry()
	
	registry.NewCounter("test_counter", "Test counter", nil).Inc()
	registry.NewGauge("test_gauge", "Test gauge", nil).Set(10)
	
	metrics := registry.GetAllMetrics()
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics before reset, got %d", len(metrics))
	}
	
	registry.Reset()
	
	metrics = registry.GetAllMetrics()
	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics after reset, got %d", len(metrics))
	}
}

func TestJSONHandler(t *testing.T) {
	registry := NewRegistry()
	
	counter := registry.NewCounter("test_counter", "Test counter", nil)
	counter.Inc()
	
	gauge := registry.NewGauge("test_gauge", "Test gauge", nil)
	gauge.Set(42)
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	recorder := httptest.NewRecorder()
	
	handler := registry.Handler()
	handler(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	
	contentType := recorder.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected content type application/json, got %s", contentType)
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
	
	if _, exists := response["metrics"]; !exists {
		t.Error("Expected metrics in response")
	}
	
	if _, exists := response["timestamp"]; !exists {
		t.Error("Expected timestamp in response")
	}
	
	if count, exists := response["count"]; !exists || count != float64(2) {
		t.Errorf("Expected count 2, got %v", count)
	}
}

func TestPrometheusHandler(t *testing.T) {
	registry := NewRegistry()
	
	counter := registry.NewCounter("test_counter", "Test counter help", nil)
	counter.Inc()
	
	req := httptest.NewRequest("GET", "/metrics", nil)
	recorder := httptest.NewRecorder()
	
	handler := registry.PrometheusHandler()
	handler(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}
	
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected content type text/plain, got %s", contentType)
	}
	
	body := recorder.Body.String()
	
	// Check for help comment
	if !strings.Contains(body, "# HELP test_counter Test counter help") {
		t.Error("Expected help comment in Prometheus output")
	}
	
	// Check for type comment
	if !strings.Contains(body, "# TYPE test_counter counter") {
		t.Error("Expected type comment in Prometheus output")
	}
	
	// Check for metric value
	if !strings.Contains(body, "test_counter 1") {
		t.Error("Expected metric value in Prometheus output")
	}
}

func TestDefaultRegistryFunctions(t *testing.T) {
	// Reset default registry to ensure clean state
	Reset()
	
	counter := NewCounter("default_counter", "Default counter", nil)
	counter.Inc()
	
	gauge := NewGauge("default_gauge", "Default gauge", nil)
	gauge.Set(10)
	
	histogram := NewHistogram("default_histogram", "Default histogram", nil, []float64{1, 5, 10})
	histogram.Observe(3)
	
	metrics := GetAllMetrics()
	if len(metrics) != 3 {
		t.Errorf("Expected 3 metrics in default registry, got %d", len(metrics))
	}
	
	// Test default handlers
	req := httptest.NewRequest("GET", "/metrics", nil)
	recorder := httptest.NewRecorder()
	
	handler := Handler()
	handler(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200 from default handler, got %d", recorder.Code)
	}
}

func TestConcurrency(t *testing.T) {
	registry := NewRegistry()
	counter := registry.NewCounter("concurrent_counter", "Concurrent counter", nil)
	gauge := registry.NewGauge("concurrent_gauge", "Concurrent gauge", nil)
	histogram := registry.NewHistogram("concurrent_histogram", "Concurrent histogram", nil, []float64{1, 5, 10})
	
	// Run concurrent operations
	done := make(chan bool)
	
	// Counter operations
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				counter.Inc()
			}
			done <- true
		}()
	}
	
	// Gauge operations
	for i := 0; i < 10; i++ {
		go func(val float64) {
			for j := 0; j < 100; j++ {
				gauge.Set(val)
				gauge.Add(1)
			}
			done <- true
		}(float64(i))
	}
	
	// Histogram operations
	for i := 0; i < 10; i++ {
		go func(val float64) {
			for j := 0; j < 100; j++ {
				histogram.Observe(val)
			}
			done <- true
		}(float64(i))
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 30; i++ {
		<-done
	}
	
	// Verify final values
	if counter.Get() != 1000 {
		t.Errorf("Expected counter value 1000, got %f", counter.Get())
	}
	
	if histogram.GetCount() != 1000 {
		t.Errorf("Expected histogram count 1000, got %d", histogram.GetCount())
	}
}

func BenchmarkCounterInc(b *testing.B) {
	counter := NewCounter("bench_counter", "Benchmark counter", nil)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Inc()
		}
	})
}

func BenchmarkGaugeSet(b *testing.B) {
	gauge := NewGauge("bench_gauge", "Benchmark gauge", nil)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			gauge.Set(42)
		}
	})
}

func BenchmarkHistogramObserve(b *testing.B) {
	histogram := NewHistogram("bench_histogram", "Benchmark histogram", nil, []float64{1, 5, 10, 25, 50, 100})
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			histogram.Observe(25)
		}
	})
}

func BenchmarkRegistryGetAllMetrics(b *testing.B) {
	registry := NewRegistry()
	
	// Create many metrics
	for i := 0; i < 100; i++ {
		registry.NewCounter("counter", "Counter", map[string]string{"id": string(rune(i))})
		registry.NewGauge("gauge", "Gauge", map[string]string{"id": string(rune(i))})
		registry.NewHistogram("histogram", "Histogram", map[string]string{"id": string(rune(i))}, []float64{1, 5, 10})
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GetAllMetrics()
	}
}