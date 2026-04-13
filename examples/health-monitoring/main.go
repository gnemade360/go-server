package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	goserver "github.com/gnemade360/go-server"
	"github.com/gnemade360/go-server/health"
)

// This example demonstrates how to use the health monitoring system
func main() {
	// Create a new server
	server := goserver.NewServer()

	// Configure the server for development
	server.Health().ConfigureForDevelopment()

	// Set service information
	server.Health().SetVersion("1.2.3")
	server.Health().SetEnvironment("development")

	// Add custom health checks
	addCustomHealthChecks(server)

	// Add health routes to the server
	server.AddHealthRoutes()

	// Add some sample routes
	addSampleRoutes(server)

	// Configure and start the server
	server.Configure(":8080")

	log.Println("Server starting on :8080")
	log.Println("Health endpoints:")
	log.Println("  - GET /health       - Full health check")
	log.Println("  - GET /health/ready - Readiness check")
	log.Println("  - GET /health/live  - Liveness check")
	log.Println("Sample endpoints:")
	log.Println("  - GET /             - Hello world")
	log.Println("  - GET /api/status   - Simple status")

	if err := server.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func addCustomHealthChecks(server *goserver.Server) {
	// Add a database health check (simulated)
	server.Health().AddDatabaseCheck("primary", func() error {
		// In a real application, this would ping your actual database
		// For this example, we'll simulate a database that's sometimes unavailable
		if time.Now().Second()%10 == 0 {
			return sql.ErrConnDone // Simulate connection failure
		}
		return nil
	})

	// Add an external service health check
	server.Health().AddExternalServiceCheck("api-service", "https://httpbin.org/status/200")

	// Add a custom application-specific health check
	server.Health().AddCustomCheck("feature-flags", func() (health.Status, string, map[string]interface{}) {
		// Example: Check if feature flags service is working
		featureFlagsEnabled := true // This would be your actual check
		
		if featureFlagsEnabled {
			return health.StatusUp, "Feature flags service is operational", map[string]interface{}{
				"enabled_flags": []string{"new_ui", "enhanced_search"},
				"total_flags":   15,
			}
		}
		
		return health.StatusWarning, "Feature flags service degraded", map[string]interface{}{
			"reason": "Some flags unavailable",
		}
	})

	// Add a business logic health check
	server.Health().AddCustomCheck("cache-hit-rate", func() (health.Status, string, map[string]interface{}) {
		// Example: Monitor cache performance
		hitRate := 0.85 // This would come from your cache metrics
		
		details := map[string]interface{}{
			"hit_rate":    hitRate,
			"cache_size":  "2.3GB",
			"cache_items": 150000,
		}
		
		if hitRate > 0.8 {
			return health.StatusUp, "Cache performance is good", details
		} else if hitRate > 0.6 {
			return health.StatusWarning, "Cache performance is degraded", details
		}
		
		return health.StatusDown, "Cache performance is poor", details
	})
}

func addSampleRoutes(server *goserver.Server) {
	// Add a simple hello world route
	server.Router().Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<h1>Health Monitoring Example</h1>
			<p>This is a sample Go server with health monitoring.</p>
			<h2>Health Endpoints:</h2>
			<ul>
				<li><a href="/health">Full Health Check</a></li>
				<li><a href="/health/ready">Readiness Check</a></li>
				<li><a href="/health/live">Liveness Check</a></li>
			</ul>
			<h2>Sample Endpoints:</h2>
			<ul>
				<li><a href="/api/status">API Status</a></li>
			</ul>
		`))
	})

	// Add a status endpoint
	server.Router().Get("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"status": "running",
			"timestamp": "` + time.Now().Format(time.RFC3339) + `",
			"version": "1.2.3"
		}`))
	})
}