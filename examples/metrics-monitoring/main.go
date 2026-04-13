package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	goserver "github.com/gnemade360/go-server"
)

// This example demonstrates how to use the metrics collection system in the go-server framework.
func main() {
	// Create a new server
	server := goserver.NewServer()

	// Enable HTTP metrics collection
	server.EnableHTTPMetrics()

	// Add metrics routes to the server
	server.AddMetricsRoutes()

	// Add custom metrics
	addCustomMetrics(server)

	// Add sample routes that generate metrics
	addSampleRoutes(server)

	// Configure and start the server
	server.Configure(":8080")

	log.Println("Server starting on :8080")
	log.Println("Metrics endpoints:")
	log.Println("  - GET /metrics            - JSON metrics")
	log.Println("  - GET /metrics/prometheus - Prometheus format metrics")
	log.Println("Sample endpoints:")
	log.Println("  - GET /                   - Hello world")
	log.Println("  - GET /api/users          - User API (fast)")
	log.Println("  - GET /api/slow           - Slow API (demonstrates duration metrics)")
	log.Println("  - GET /api/random         - Random response time")
	log.Println("  - POST /api/data          - Data API (demonstrates request/response sizes)")

	if err := server.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func addCustomMetrics(server *goserver.Server) {
	registry := server.Metrics()

	// Create custom application metrics
	activeUsers := registry.NewGauge("app_active_users", "Number of active users", nil)
	
	// Database connection pool metrics
	dbConnections := registry.NewGauge("db_connections_active", "Active database connections", map[string]string{
		"pool": "primary",
	})
	
	// Business logic metrics
	ordersTotal := registry.NewCounter("orders_total", "Total number of orders processed", nil)
	orderValue := registry.NewHistogram("order_value_dollars", "Order value in dollars", nil, 
		[]float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000})

	// Cache metrics
	cacheHits := registry.NewCounter("cache_hits_total", "Total cache hits", map[string]string{
		"cache": "redis",
	})
	cacheMisses := registry.NewCounter("cache_misses_total", "Total cache misses", map[string]string{
		"cache": "redis",
	})

	// Simulate some background metrics collection
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				// Simulate changing metrics
				activeUsers.Set(float64(50 + rand.Intn(100)))
				dbConnections.Set(float64(5 + rand.Intn(15)))
				
				// Simulate business events
				if rand.Float64() < 0.3 { // 30% chance of an order
					ordersTotal.Inc()
					orderValue.Observe(float64(25 + rand.Intn(975))) // $25-$1000 orders
				}
				
				// Simulate cache operations
				if rand.Float64() < 0.8 { // 80% cache hit rate
					cacheHits.Inc()
				} else {
					cacheMisses.Inc()
				}
			}
		}
	}()
}

func addSampleRoutes(server *goserver.Server) {
	// Add a simple hello world route
	server.Router().Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<h1>Metrics Monitoring Example</h1>
			<p>This is a sample Go server with metrics collection.</p>
			<h2>Metrics Endpoints:</h2>
			<ul>
				<li><a href="/metrics">JSON Metrics</a></li>
				<li><a href="/metrics/prometheus">Prometheus Format Metrics</a></li>
			</ul>
			<h2>Sample Endpoints:</h2>
			<ul>
				<li><a href="/api/users">Fast API</a></li>
				<li><a href="/api/slow">Slow API</a></li>
				<li><a href="/api/random">Random Speed API</a></li>
			</ul>
		`))
	})

	// Fast API endpoint
	server.Router().Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"users": [
				{"id": 1, "name": "Alice", "email": "alice@example.com"},
				{"id": 2, "name": "Bob", "email": "bob@example.com"},
				{"id": 3, "name": "Carol", "email": "carol@example.com"}
			],
			"total": 3
		}`))
	})

	// Slow API endpoint (demonstrates duration metrics)
	server.Router().Get("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow processing
		time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"message": "This endpoint simulates slow processing",
			"processing_time": "500-1500ms",
			"timestamp": "` + time.Now().Format(time.RFC3339) + `"
		}`))
	})

	// Random response time API
	server.Router().Get("/api/random", func(w http.ResponseWriter, r *http.Request) {
		// Random delay between 10ms and 2 seconds
		delay := time.Duration(rand.Intn(1990)+10) * time.Millisecond
		time.Sleep(delay)
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"message": "Random response time endpoint",
			"delay_ms": ` + strconv.FormatInt(delay.Milliseconds(), 10) + `,
			"random_value": ` + strconv.Itoa(rand.Intn(1000)) + `
		}`))
	})

	// Data API endpoint (demonstrates request/response size metrics)
	server.Router().Post("/api/data", func(w http.ResponseWriter, r *http.Request) {
		// Read the request body (contributes to request size metrics)
		// Generate a larger response (contributes to response size metrics)
		
		responseSize := "small"
		responseData := `{"message": "Data processed", "size": "small"}`
		
		// 30% chance of large response
		if rand.Float64() < 0.3 {
			responseSize = "large"
			responseData = `{
				"message": "Data processed with large response",
				"size": "large",
				"data": [` + generateLargeData() + `],
				"timestamp": "` + time.Now().Format(time.RFC3339) + `",
				"metadata": {
					"processing_time": "` + time.Now().Format(time.RFC3339) + `",
					"server_version": "1.0.0",
					"api_version": "v1"
				}
			}`
		}
		
		// Custom metric for this endpoint
		server.Metrics().NewCounter("data_api_requests_total", "Data API requests", map[string]string{
			"response_size": responseSize,
		}).Inc()
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(responseData))
	})

	// Error endpoint (demonstrates error status code metrics)
	server.Router().Get("/api/error", func(w http.ResponseWriter, r *http.Request) {
		// Randomly return different error codes
		errorCodes := []int{400, 401, 403, 404, 429, 500, 503}
		statusCode := errorCodes[rand.Intn(len(errorCodes))]
		
		w.WriteHeader(statusCode)
		w.Write([]byte(`{"error": "Simulated error", "code": ` + strconv.Itoa(statusCode) + `}`))
	})

	// Status endpoint showing current metrics
	server.Router().Get("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Get HTTP metrics summary
		httpMetrics := server.HTTPMetrics().GetMetrics()
		
		response := `{
			"status": "running",
			"timestamp": "` + time.Now().Format(time.RFC3339) + `",
			"version": "1.0.0",
			"metrics_summary": {
				"requests_total": ` + formatFloat64(httpMetrics["requests_total"].Value) + `,
				"active_requests": ` + formatFloat64(httpMetrics["active_requests"].Value) + `,
				"avg_request_duration": ` + formatFloat64(httpMetrics["request_duration"].Value) + `
			}
		}`
		
		w.Write([]byte(response))
	})
}

func generateLargeData() string {
	// Generate a large JSON array to increase response size
	data := ""
	for i := 0; i < 100; i++ {
		if i > 0 {
			data += ","
		}
		data += `{"id": ` + strconv.Itoa(i) + `, "value": "data_` + strconv.Itoa(i) + `", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`
	}
	return data
}

func formatFloat64(value float64) string {
	return fmt.Sprintf("%.2f", value)
}