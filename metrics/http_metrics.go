package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// HTTPMetrics holds metrics for HTTP requests
type HTTPMetrics struct {
	registry         *Registry
	requestsTotal    *Counter
	requestDuration  *Histogram
	requestSize      *Histogram
	responseSize     *Histogram
	activeRequests   *Gauge
}

// NewHTTPMetrics creates a new HTTPMetrics instance
func NewHTTPMetrics(registry *Registry) *HTTPMetrics {
	if registry == nil {
		registry = defaultRegistry
	}
	
	// Default histogram buckets for request duration (in seconds)
	durationBuckets := []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	
	// Default histogram buckets for request/response size (in bytes)
	sizeBuckets := []float64{64, 256, 1024, 4096, 16384, 65536, 262144, 1048576, 4194304}
	
	return &HTTPMetrics{
		registry: registry,
		requestsTotal: registry.NewCounter(
			"http_requests_total",
			"Total number of HTTP requests",
			map[string]string{},
		),
		requestDuration: registry.NewHistogram(
			"http_request_duration_seconds",
			"HTTP request duration in seconds",
			map[string]string{},
			durationBuckets,
		),
		requestSize: registry.NewHistogram(
			"http_request_size_bytes",
			"HTTP request size in bytes",
			map[string]string{},
			sizeBuckets,
		),
		responseSize: registry.NewHistogram(
			"http_response_size_bytes",
			"HTTP response size in bytes",
			map[string]string{},
			sizeBuckets,
		),
		activeRequests: registry.NewGauge(
			"http_requests_active",
			"Number of active HTTP requests",
			map[string]string{},
		),
	}
}

// responseWriter wraps http.ResponseWriter to capture response metrics
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += size
	return size, err
}

// Middleware returns an HTTP middleware that collects metrics
func (hm *HTTPMetrics) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Increment active requests
			hm.activeRequests.Inc()
			defer hm.activeRequests.Dec()
			
			// Wrap the response writer
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     200, // Default status code
			}
			
			// Record request size
			if r.ContentLength > 0 {
				hm.requestSize.Observe(float64(r.ContentLength))
			}
			
			// Call the next handler
			next.ServeHTTP(rw, r)
			
			// Record metrics after request completion
			duration := time.Since(start).Seconds()
			hm.requestDuration.Observe(duration)
			
			// Record response size
			if rw.size > 0 {
				hm.responseSize.Observe(float64(rw.size))
			}
			
			// Increment base request counter
			hm.requestsTotal.Inc()
			
			// Increment request counter with labels
			labels := map[string]string{
				"method": r.Method,
				"status": strconv.Itoa(rw.statusCode),
				"path":   sanitizePath(r.URL.Path),
			}
			
			hm.registry.NewCounter(
				"http_requests_total",
				"Total number of HTTP requests",
				labels,
			).Inc()
			
			// Record duration with labels
			hm.registry.NewHistogram(
				"http_request_duration_seconds",
				"HTTP request duration in seconds",
				labels,
				[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			).Observe(duration)
		})
	}
}

// MiddlewareFunc returns a middleware function that can be used directly
func (hm *HTTPMetrics) MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	middleware := hm.Middleware()
	return middleware(next).ServeHTTP
}

// GetMetrics returns HTTP-specific metrics
func (hm *HTTPMetrics) GetMetrics() map[string]*Metric {
	metrics := make(map[string]*Metric)
	
	metrics["requests_total"] = hm.requestsTotal.ToMetric()
	metrics["request_duration"] = hm.requestDuration.ToMetric()
	metrics["request_size"] = hm.requestSize.ToMetric()
	metrics["response_size"] = hm.responseSize.ToMetric()
	metrics["active_requests"] = hm.activeRequests.ToMetric()
	
	return metrics
}

// sanitizePath cleans up the path for metrics labeling
func sanitizePath(path string) string {
	if path == "" {
		return "/"
	}
	
	// For metrics, we might want to normalize paths with IDs
	// This is a simple implementation - in production you might want
	// more sophisticated path normalization
	if len(path) > 50 {
		return "/long_path"
	}
	
	return path
}

// Package-level convenience functions using the default registry
var defaultHTTPMetrics = NewHTTPMetrics(defaultRegistry)

// Middleware returns the default HTTP metrics middleware
func Middleware() func(http.Handler) http.Handler {
	return defaultHTTPMetrics.Middleware()
}

// MiddlewareFunc returns the default HTTP metrics middleware function
func MiddlewareFunc(next http.HandlerFunc) http.HandlerFunc {
	return defaultHTTPMetrics.MiddlewareFunc(next)
}

// GetHTTPMetrics returns the default HTTP metrics
func GetHTTPMetrics() map[string]*Metric {
	return defaultHTTPMetrics.GetMetrics()
}