package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gnemade360/go-server"
)

func main() {
	// Create a new server
	server := goserver.NewServer()

	// Example 1: Simple rate limiting (60 requests per minute)
	middleware1 := goserver.RateLimitSimple(60) // 60 requests per minute
	handler1 := middleware1(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/simple", handler1)

	// Example 2: Custom rate limiting configuration
	rateLimitConfig := goserver.DefaultRateLimitConfig()
	rateLimitConfig.RequestsPerMinute = 30 // 30 requests per minute
	rateLimitConfig.BurstSize = 5          // Allow burst of 5 requests
	
	middleware2 := goserver.RateLimit(rateLimitConfig)
	handler2 := middleware2(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/custom", handler2)

	// Example 3: Rate limiting by user ID
	userExtractor := func(r *http.Request) string {
		return r.Header.Get("X-User-ID")
	}
	
	middleware3 := goserver.RateLimitByUserID(100, userExtractor) // 100 requests per minute per user
	handler3 := middleware3(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/user", handler3)

	// Example 4: Global rate limiting (shared across all clients)
	middleware4 := goserver.RateLimitGlobal(1000) // 1000 requests per minute globally
	handler4 := middleware4(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/global", handler4)

	// Example 5: Custom rate limit handler
	customConfig := goserver.DefaultRateLimitConfig()
	customConfig.RequestsPerMinute = 10
	customConfig.BurstSize = 2
	customConfig.OnLimitExceeded = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "Slow down! Try again later.", "retry_after": 60}`))
	}
	
	middleware5 := goserver.RateLimit(customConfig)
	handler5 := middleware5(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/custom-handler", handler5)

	// Add some utility endpoints
	server.Router().Get("/", indexHandler)
	server.Router().Get("/test", testRateLimitHandler)

	// Configure and start server
	server.Configure(":8080")

	fmt.Println("Rate limiting example server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  GET /               - Main page with instructions")
	fmt.Println("  GET /test           - Test endpoint for rate limiting")
	fmt.Println("  GET /api/simple     - Simple rate limiting (60/min)")
	fmt.Println("  GET /api/custom     - Custom rate limiting (30/min, burst 5)")
	fmt.Println("  GET /api/user       - User-based rate limiting (requires X-User-ID header)")
	fmt.Println("  GET /api/global     - Global rate limiting (1000/min shared)")
	fmt.Println("  GET /api/custom-handler - Custom error handler")

	// Start server with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func simpleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Success! Request was not rate limited.", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Rate Limiting Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .method { color: #2196F3; font-weight: bold; }
        .path { color: #4CAF50; font-weight: bold; }
        .description { color: #666; margin-top: 5px; }
        button { background: #2196F3; color: white; border: none; padding: 10px 20px; margin: 5px; border-radius: 3px; cursor: pointer; }
        button:hover { background: #1976D2; }
        #output { background: #f9f9f9; border: 1px solid #ddd; padding: 15px; margin-top: 20px; border-radius: 5px; font-family: monospace; white-space: pre-wrap; }
    </style>
</head>
<body>
    <h1>Rate Limiting Example Server</h1>
    <p>This server demonstrates different rate limiting strategies. Click the buttons below to test each endpoint.</p>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/simple</span>
        <div class="description">Simple rate limiting: 60 requests per minute with burst of 10</div>
        <button onclick="testEndpoint('/api/simple')">Test Simple Rate Limiting</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/custom</span>
        <div class="description">Custom rate limiting: 30 requests per minute with burst of 5</div>
        <button onclick="testEndpoint('/api/custom')">Test Custom Rate Limiting</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/user</span>
        <div class="description">User-based rate limiting: 100 requests per minute per user (requires X-User-ID header)</div>
        <button onclick="testEndpoint('/api/user', {'X-User-ID': 'user123'})">Test User Rate Limiting</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/global</span>
        <div class="description">Global rate limiting: 1000 requests per minute shared across all clients</div>
        <button onclick="testEndpoint('/api/global')">Test Global Rate Limiting</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/custom-handler</span>
        <div class="description">Custom error handler: 10 requests per minute with custom rate limit response</div>
        <button onclick="testEndpoint('/api/custom-handler')">Test Custom Handler</button>
    </div>
    
    <div style="margin-top: 20px;">
        <button onclick="burstTest()">Burst Test (Send 15 requests quickly)</button>
        <button onclick="clearOutput()">Clear Output</button>
    </div>
    
    <h3>Output:</h3>
    <div id="output">Click any button above to test rate limiting...</div>

    <script>
        function testEndpoint(path, headers = {}) {
            fetch(path, {
                method: 'GET',
                headers: headers
            })
            .then(response => {
                const output = document.getElementById('output');
                const timestamp = new Date().toISOString();
                let headerInfo = '';
                
                // Show rate limit headers
                if (response.headers.get('X-RateLimit-Limit')) {
                    headerInfo = '\\nRate Limit Headers:';
                    headerInfo += '\\n  X-RateLimit-Limit: ' + response.headers.get('X-RateLimit-Limit');
                    headerInfo += '\\n  X-RateLimit-Remaining: ' + response.headers.get('X-RateLimit-Remaining');
                    headerInfo += '\\n  X-RateLimit-Reset: ' + response.headers.get('X-RateLimit-Reset');
                    if (response.headers.get('Retry-After')) {
                        headerInfo += '\\n  Retry-After: ' + response.headers.get('Retry-After');
                    }
                }
                
                return response.text().then(text => {
                    output.textContent += '[' + timestamp + '] ' + response.status + ' ' + path + headerInfo + '\\n' + text + '\\n\\n';
                    output.scrollTop = output.scrollHeight;
                });
            })
            .catch(error => {
                const output = document.getElementById('output');
                output.textContent += '[ERROR] ' + error.message + '\\n\\n';
                output.scrollTop = output.scrollHeight;
            });
        }
        
        function burstTest() {
            const output = document.getElementById('output');
            output.textContent += '[BURST TEST] Sending 15 requests to /api/custom...\\n';
            
            for (let i = 0; i < 15; i++) {
                setTimeout(() => {
                    testEndpoint('/api/custom');
                }, i * 100); // 100ms between requests
            }
        }
        
        function clearOutput() {
            document.getElementById('output').textContent = 'Output cleared...\\n';
        }
    </script>
</body>
</html>`
	w.Write([]byte(html))
}

func testRateLimitHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"message":    "This endpoint has no rate limiting for comparison",
		"timestamp":  time.Now().Format(time.RFC3339),
		"method":     r.Method,
		"path":       r.URL.Path,
		"remote_ip":  r.RemoteAddr,
		"user_agent": r.Header.Get("User-Agent"),
	}
	
	w.Write([]byte(fmt.Sprintf(`{
		"message": "%s",
		"timestamp": "%s",
		"method": "%s",
		"path": "%s",
		"remote_ip": "%s",
		"user_agent": "%s"
	}`, response["message"], response["timestamp"], response["method"], response["path"], response["remote_ip"], response["user_agent"])))
}