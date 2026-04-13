package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gnemade360/go-server"
)

func main() {
	// Create server with comprehensive configuration
	server := createConfiguredServer()

	// Add all endpoints
	setupRoutes(server)

	// Configure server with middleware stack
	server.Configure(":8080",
		// Add default middleware
		goserver.WithMiddleware(
			goserver.RequestLogging(),           // Log all requests
			goserver.RateLimitSimple(100),       // Rate limit to 100 requests/minute
		),
	)

	// Add health and metrics endpoints
	server.AddHealthRoutes()
	server.AddMetricsRoutes()
	server.EnableHTTPMetrics()

	fmt.Println("🚀 Server utilities example starting on :8080")
	printEndpoints()

	// Start server with graceful shutdown
	startServerWithGracefulShutdown(server)
}

// createConfiguredServer demonstrates comprehensive server configuration
func createConfiguredServer() *goserver.Server {
	// Custom server configuration
	config := goserver.DefaultServerConfig()
	config.Addr = ":8080"
	config.CORSOrigin = "*"
	config.EnableGzip = true
	config.CacheDuration = 1 * time.Hour

	// Create server with config
	server := goserver.NewServerWithConfig(config)

	// Add custom health checks
	server.Health().AddCustomCheck("database", databaseHealthCheck)
	server.Health().AddExternalServiceCheck("httpbin", "https://httpbin.org/status/200")
	server.Health().AddCustomCheck("disk_space", diskSpaceHealthCheck)

	return server
}

// setupRoutes demonstrates various routing patterns and utilities
func setupRoutes(server *goserver.Server) {
	router := server.Router()

	// Main page
	router.Get("/", indexHandler)

	// API endpoints with different patterns
	router.Get("/api/info", serverInfoHandler)
	router.Get("/api/status", statusHandler)
	router.Get("/api/config", configHandler)

	// Examples of different HTTP methods
	router.Post("/api/data", createDataHandler)
	router.Put("/api/data/123", updateDataHandler)
	router.Delete("/api/data/123", deleteDataHandler)

	// Utility endpoints
	router.Get("/api/echo", echoHandler)
	router.Get("/api/headers", headersHandler)
	router.Get("/api/ip", ipHandler)
	router.Get("/api/time", timeHandler)

	// Examples with middleware
	loggingMw := goserver.RequestLogging()
	timeoutMw := timeoutMiddleware(5*time.Second)
	slowHandlerWithMiddleware := loggingMw(timeoutMw(http.HandlerFunc(slowHandler)))
	router.HandleFunc(http.MethodGet, "/api/slow", slowHandlerWithMiddleware.ServeHTTP)

	authMw := authMiddleware()
	protectedHandlerWithMiddleware := authMw(http.HandlerFunc(protectedHandler))
	router.HandleFunc(http.MethodGet, "/api/protected", protectedHandlerWithMiddleware.ServeHTTP)

	// File serving example
	router.Get("/static/*", staticFileHandler)

	// WebSocket example endpoint (placeholder)
	router.Get("/ws", websocketHandler)
}

// Utility middleware examples

func timeoutMiddleware(timeout time.Duration) goserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func authMiddleware() goserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "demo-key" {
				http.Error(w, `{"error": "Unauthorized", "message": "Valid X-API-Key header required"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Health check examples

func databaseHealthCheck() (goserver.Status, string, map[string]interface{}) {
	// Simulate database check
	time.Sleep(10 * time.Millisecond)
	return goserver.StatusUp, "Database connection is healthy", map[string]interface{}{"response_time": "10ms"}
}

// externalAPIHealthCheck is handled by AddExternalServiceCheck

func diskSpaceHealthCheck() (goserver.Status, string, map[string]interface{}) {
	// Simulate disk space check
	return goserver.StatusUp, "Disk space is sufficient", map[string]interface{}{"free_space": "75%"}
}

// Handler examples

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Server Utilities Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; line-height: 1.6; }
        .endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .method { color: #2196F3; font-weight: bold; }
        .path { color: #4CAF50; font-weight: bold; }
        .category { background: #e3f2fd; padding: 15px; margin: 20px 0; border-radius: 5px; }
        h3 { margin-top: 0; }
        code { background: #f0f0f0; padding: 2px 4px; border-radius: 3px; }
    </style>
</head>
<body>
    <h1>🚀 Server Utilities Example</h1>
    <p>This example demonstrates comprehensive server utilities, middleware, and best practices.</p>
    
    <div class="category">
        <h3>🔍 Health & Monitoring</h3>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/health</span> - Overall health check
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/health/ready</span> - Readiness check
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/health/live</span> - Liveness check
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/metrics</span> - Metrics in JSON format
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/metrics/prometheus</span> - Prometheus format metrics
        </div>
    </div>
    
    <div class="category">
        <h3>ℹ️ Server Information</h3>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/info</span> - Server information
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/status</span> - Current server status
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/config</span> - Server configuration
        </div>
    </div>
    
    <div class="category">
        <h3>🔧 Utility Endpoints</h3>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/echo?message=hello</span> - Echo service
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/headers</span> - Show request headers
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/ip</span> - Show client IP
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/time</span> - Server time
        </div>
    </div>
    
    <div class="category">
        <h3>🛡️ Middleware Examples</h3>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/slow</span> - Timeout middleware demo
        </div>
        <div class="endpoint">
            <span class="method">GET</span> <span class="path">/api/protected</span> - Auth middleware (requires <code>X-API-Key: demo-key</code>)
        </div>
    </div>
    
    <div class="category">
        <h3>📊 Data Operations</h3>
        <div class="endpoint">
            <span class="method">POST</span> <span class="path">/api/data</span> - Create data
        </div>
        <div class="endpoint">
            <span class="method">PUT</span> <span class="path">/api/data/123</span> - Update data
        </div>
        <div class="endpoint">
            <span class="method">DELETE</span> <span class="path">/api/data/123</span> - Delete data
        </div>
    </div>
    
    <p><strong>Features demonstrated:</strong></p>
    <ul>
        <li>🚦 Rate limiting (100 requests/minute)</li>
        <li>📝 Structured request logging</li>
        <li>🏥 Health checks with custom checks</li>
        <li>📈 Metrics collection and HTTP metrics</li>
        <li>🔒 CORS and security middleware</li>
        <li>⚡ Gzip compression</li>
        <li>🛡️ Custom authentication middleware</li>
        <li>⏱️ Request timeout handling</li>
        <li>🔄 Graceful shutdown</li>
    </ul>
</body>
</html>`
	w.Write([]byte(html))
}

func serverInfoHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"name":        "Go Server Utilities Example",
		"version":     "1.0.0",
		"go_version":  "go1.21+",
		"start_time":  startTime.Format(time.RFC3339),
		"uptime":      time.Since(startTime).String(),
		"environment": getEnv("ENVIRONMENT", "development"),
		"features": []string{
			"Rate Limiting",
			"Structured Logging",
			"Health Checks",
			"Metrics Collection",
			"CORS Support",
			"Gzip Compression",
			"Graceful Shutdown",
		},
	}
	
	writeJSONResponse(w, http.StatusOK, info)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"status":     "healthy",
		"timestamp":  time.Now().Format(time.RFC3339),
		"uptime":     time.Since(startTime).String(),
		"memory":     getMemoryStats(),
		"goroutines": getGoroutineCount(),
	}
	
	writeJSONResponse(w, http.StatusOK, status)
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	config := map[string]interface{}{
		"server": map[string]interface{}{
			"addr":            ":8080",
			"cors_origin":     "*",
			"gzip_enabled":    true,
			"cache_duration":  "1h",
		},
		"rate_limiting": map[string]interface{}{
			"enabled":            true,
			"requests_per_minute": 100,
		},
		"logging": map[string]interface{}{
			"enabled": true,
			"format":  "structured",
		},
	}
	
	writeJSONResponse(w, http.StatusOK, config)
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	message := r.URL.Query().Get("message")
	if message == "" {
		message = "Hello, World!"
	}
	
	response := map[string]interface{}{
		"echo":      message,
		"timestamp": time.Now().Format(time.RFC3339),
		"method":    r.Method,
		"path":      r.URL.Path,
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func headersHandler(w http.ResponseWriter, r *http.Request) {
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}
	
	response := map[string]interface{}{
		"headers":   headers,
		"method":    r.Method,
		"url":       r.URL.String(),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func ipHandler(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	
	response := map[string]interface{}{
		"client_ip":    clientIP,
		"remote_addr":  r.RemoteAddr,
		"x_forwarded":  r.Header.Get("X-Forwarded-For"),
		"x_real_ip":    r.Header.Get("X-Real-IP"),
		"timestamp":    time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func timeHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	
	response := map[string]interface{}{
		"unix_timestamp": now.Unix(),
		"rfc3339":        now.Format(time.RFC3339),
		"utc":            now.UTC().Format(time.RFC3339),
		"local":          now.Format("2006-01-02 15:04:05 MST"),
		"timezone":       now.Format("MST"),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func createDataHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message":   "Data created successfully",
		"id":        generateID(),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusCreated, response)
}

func updateDataHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/data/"):]
	
	response := map[string]interface{}{
		"message":   "Data updated successfully",
		"id":        id,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func deleteDataHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/data/"):]
	
	response := map[string]interface{}{
		"message":   "Data deleted successfully",
		"id":        id,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate slow processing
	select {
	case <-time.After(3 * time.Second):
		response := map[string]interface{}{
			"message":   "Slow operation completed",
			"duration":  "3 seconds",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		writeJSONResponse(w, http.StatusOK, response)
	case <-r.Context().Done():
		http.Error(w, `{"error": "Request timeout"}`, http.StatusRequestTimeout)
	}
}

func protectedHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"message":   "Access granted to protected resource",
		"user":      "authenticated",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

func staticFileHandler(w http.ResponseWriter, r *http.Request) {
	// Simple static file serving example
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Static file content - " + r.URL.Path))
}

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Placeholder for WebSocket endpoint
	response := map[string]interface{}{
		"message": "WebSocket endpoint placeholder",
		"note":    "Implement WebSocket upgrade logic here",
		"path":    r.URL.Path,
	}
	
	writeJSONResponse(w, http.StatusOK, response)
}

// Utility functions

var startTime = time.Now()

func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func generateID() string {
	return fmt.Sprintf("id_%d", time.Now().UnixNano())
}

func getMemoryStats() map[string]interface{} {
	// Simplified memory stats
	return map[string]interface{}{
		"note": "Memory stats would be implemented here",
	}
}

func getGoroutineCount() int {
	// Simplified goroutine count
	return 42 // placeholder
}

func printEndpoints() {
	endpoints := []string{
		"Health:     http://localhost:8080/health",
		"Metrics:    http://localhost:8080/metrics", 
		"Main page:  http://localhost:8080/",
		"Server info: http://localhost:8080/api/info",
		"Echo:       http://localhost:8080/api/echo?message=hello",
	}
	
	fmt.Println("📍 Available endpoints:")
	for _, endpoint := range endpoints {
		fmt.Printf("   %s\n", endpoint)
	}
	fmt.Println()
}

func startServerWithGracefulShutdown(server *goserver.Server) {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	
	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start server in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	
	go func() {
		defer wg.Done()
		fmt.Println("🎯 Server is ready to handle requests")
		
		if err := server.Start(ctx); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	
	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n🛑 Shutdown signal received, stopping server...")
	
	// Cancel context to trigger graceful shutdown
	cancel()
	
	// Wait for server to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		fmt.Println("✅ Server stopped gracefully")
	case <-time.After(30 * time.Second):
		fmt.Println("⚠️ Server shutdown timeout")
	}
}