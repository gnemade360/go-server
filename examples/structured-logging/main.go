package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gnemade360/go-server"
)

func main() {
	// Create a new server
	server := goserver.NewServer()

	// Example 1: Simple request logging (default configuration)
	middleware1 := goserver.RequestLogging()
	handler1 := middleware1(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/simple", handler1)

	// Example 2: Request logging with headers
	middleware2 := goserver.RequestLoggingWithHeaders("Content-Type", "Authorization", "X-Custom-Header")
	handler2 := middleware2(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/with-headers", handler2)

	// Example 3: Simple text-based logging
	middleware3 := goserver.RequestLoggingSimple()
	handler3 := middleware3(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/simple-text", handler3)

	// Example 4: Custom structured logging configuration
	customConfig := goserver.DefaultLoggingConfig()
	customConfig.IncludeHeaders = true
	customConfig.IncludeQuery = true
	customConfig.IncludeUserAgent = true
	customConfig.IncludeReferer = true
	customConfig.HeadersToLog = []string{"Content-Type", "Authorization", "X-Request-ID", "X-User-ID"}
	customConfig.RequestIDHeader = "X-Request-ID"
	customConfig.LogHandler = customLogHandler // Custom log handler

	middleware4 := goserver.StructuredLogging(customConfig)
	handler4 := middleware4(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/custom", handler4)

	// Example 5: File-based logging
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	fileConfig := goserver.DefaultLoggingConfig()
	fileConfig.LogHandler = func(entry goserver.LogEntry) {
		jsonBytes, err := json.Marshal(entry)
		if err != nil {
			log.Printf("Failed to marshal log entry: %v", err)
			return
		}
		logFile.Write(append(jsonBytes, '\n'))
	}

	middleware5 := goserver.StructuredLogging(fileConfig)
	handler5 := middleware5(http.HandlerFunc(simpleHandler))
	server.Router().Handle(http.MethodGet, "/api/file-log", handler5)

	// Example 6: Multiple middleware with logging
	loggingMw := goserver.RequestLogging()
	rateLimitMw := goserver.RateLimitSimple(60)
	handler6 := loggingMw(rateLimitMw(http.HandlerFunc(simpleHandler)))
	server.Router().Handle(http.MethodGet, "/api/multiple", handler6)

	// Add some test endpoints
	server.Router().Get("/", indexHandler)
	server.Router().Post("/api/post", postHandler)
	server.Router().Get("/api/error", errorHandler)
	server.Router().Get("/api/slow", slowHandler)

	// Configure and start server
	server.Configure(":8080")

	fmt.Println("Structured logging example server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  GET /                   - Main page with examples")
	fmt.Println("  GET /api/simple         - Simple logging")
	fmt.Println("  GET /api/with-headers   - Logging with headers")
	fmt.Println("  GET /api/simple-text    - Simple text logging")
	fmt.Println("  GET /api/custom         - Custom logging configuration")
	fmt.Println("  GET /api/file-log       - File-based logging")
	fmt.Println("  GET /api/multiple       - Multiple middleware")
	fmt.Println("  POST /api/post          - POST request example")
	fmt.Println("  GET /api/error          - Error response example")
	fmt.Println("  GET /api/slow           - Slow response example")
	fmt.Println()
	fmt.Println("Check 'server.log' file for file-based logging output")

	// Start server with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// customLogHandler demonstrates a custom log handler with colored output
func customLogHandler(entry goserver.LogEntry) {
	// Color codes for terminal output
	const (
		Reset  = "\033[0m"
		Red    = "\033[31m"
		Green  = "\033[32m"
		Yellow = "\033[33m"
		Blue   = "\033[34m"
		Purple = "\033[35m"
		Cyan   = "\033[36m"
		Gray   = "\033[37m"
	)

	// Choose color based on status code
	var statusColor string
	switch {
	case entry.Status >= 200 && entry.Status < 300:
		statusColor = Green
	case entry.Status >= 300 && entry.Status < 400:
		statusColor = Blue
	case entry.Status >= 400 && entry.Status < 500:
		statusColor = Yellow
	case entry.Status >= 500:
		statusColor = Red
	default:
		statusColor = Gray
	}

	// Format duration
	durationStr := fmt.Sprintf("%.2fms", entry.DurationMS)

	// Print colored log line
	fmt.Printf("%s[%s]%s %s%s%s %s%s%s %s%d%s %s %s%s%s",
		Gray, entry.Timestamp.Format("2006-01-02 15:04:05"), Reset,
		Purple, entry.Method, Reset,
		Cyan, entry.URL, Reset,
		statusColor, entry.Status, Reset,
		entry.StatusText,
		Yellow, durationStr, Reset,
	)

	// Add request ID if present
	if entry.RequestID != "" {
		fmt.Printf(" %s[%s]%s", Blue, entry.RequestID, Reset)
	}

	// Add error if present
	if entry.ErrorMessage != "" {
		fmt.Printf(" %sERROR: %s%s", Red, entry.ErrorMessage, Reset)
	}

	fmt.Println()

	// Also log as JSON for structured analysis
	jsonBytes, _ := json.Marshal(entry)
	log.Printf("STRUCTURED: %s", string(jsonBytes))
}

func simpleHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"message":    "Success! Request logged.",
		"timestamp":  time.Now().Format(time.RFC3339),
		"method":     r.Method,
		"path":       r.URL.Path,
		"query":      r.URL.Query(),
		"headers": map[string]string{
			"User-Agent":    r.Header.Get("User-Agent"),
			"Content-Type":  r.Header.Get("Content-Type"),
			"X-Request-ID":  r.Header.Get("X-Request-ID"),
			"X-User-ID":     r.Header.Get("X-User-ID"),
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	
	response := map[string]interface{}{
		"message":     "POST request processed",
		"timestamp":   time.Now().Format(time.RFC3339),
		"content_length": r.ContentLength,
		"content_type": r.Header.Get("Content-Type"),
	}
	
	json.NewEncoder(w).Encode(response)
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	
	response := map[string]interface{}{
		"error":     "This is an example error response",
		"timestamp": time.Now().Format(time.RFC3339),
		"code":      "EXAMPLE_ERROR",
	}
	
	json.NewEncoder(w).Encode(response)
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate slow processing
	time.Sleep(2 * time.Second)
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	response := map[string]interface{}{
		"message":    "Slow response completed",
		"timestamp":  time.Now().Format(time.RFC3339),
		"duration":   "2 seconds",
	}
	
	json.NewEncoder(w).Encode(response)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Structured Logging Example</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { background: #f5f5f5; padding: 10px; margin: 10px 0; border-radius: 5px; }
        .method { color: #2196F3; font-weight: bold; }
        .path { color: #4CAF50; font-weight: bold; }
        .description { color: #666; margin-top: 5px; }
        button { background: #2196F3; color: white; border: none; padding: 10px 20px; margin: 5px; border-radius: 3px; cursor: pointer; }
        button:hover { background: #1976D2; }
        .input-group { margin: 10px 0; }
        label { display: inline-block; width: 120px; }
        input { padding: 5px; margin: 2px; }
        #output { background: #f9f9f9; border: 1px solid #ddd; padding: 15px; margin-top: 20px; border-radius: 5px; font-family: monospace; white-space: pre-wrap; max-height: 400px; overflow-y: auto; }
    </style>
</head>
<body>
    <h1>Structured Logging Example Server</h1>
    <p>This server demonstrates different logging middleware configurations. Check your server console and server.log file to see the logging output.</p>
    
    <div style="background: #e3f2fd; padding: 15px; border-radius: 5px; margin-bottom: 20px;">
        <h3>Request Headers (optional)</h3>
        <div class="input-group">
            <label>X-Request-ID:</label>
            <input type="text" id="requestId" placeholder="req-123" />
        </div>
        <div class="input-group">
            <label>X-User-ID:</label>
            <input type="text" id="userId" placeholder="user-456" />
        </div>
        <div class="input-group">
            <label>X-Custom-Header:</label>
            <input type="text" id="customHeader" placeholder="custom-value" />
        </div>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/simple</span>
        <div class="description">Simple request logging with default configuration</div>
        <button onclick="testEndpoint('/api/simple')">Test Simple Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/with-headers</span>
        <div class="description">Request logging that includes specific headers</div>
        <button onclick="testEndpoint('/api/with-headers')">Test Header Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/simple-text</span>
        <div class="description">Simple text-based logging (not JSON)</div>
        <button onclick="testEndpoint('/api/simple-text')">Test Text Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/custom</span>
        <div class="description">Custom logging configuration with colored output</div>
        <button onclick="testEndpoint('/api/custom')">Test Custom Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/file-log</span>
        <div class="description">File-based logging (check server.log file)</div>
        <button onclick="testEndpoint('/api/file-log')">Test File Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/multiple</span>
        <div class="description">Multiple middleware: logging + rate limiting</div>
        <button onclick="testEndpoint('/api/multiple')">Test Multiple Middleware</button>
    </div>
    
    <div class="endpoint">
        <span class="method">POST</span> <span class="path">/api/post</span>
        <div class="description">POST request logging example</div>
        <button onclick="testPost('/api/post')">Test POST Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/error</span>
        <div class="description">Error response logging (400 status)</div>
        <button onclick="testEndpoint('/api/error')">Test Error Logging</button>
    </div>
    
    <div class="endpoint">
        <span class="method">GET</span> <span class="path">/api/slow</span>
        <div class="description">Slow response logging (2 second delay)</div>
        <button onclick="testEndpoint('/api/slow')">Test Slow Response</button>
    </div>
    
    <div style="margin-top: 20px;">
        <button onclick="clearOutput()">Clear Output</button>
        <button onclick="loadTest()">Load Test (10 requests)</button>
    </div>
    
    <h3>Response Output:</h3>
    <div id="output">Click any button above to test logging. Check your server console for log output...</div>

    <script>
        function getHeaders() {
            const headers = {
                'Content-Type': 'application/json'
            };
            
            const requestId = document.getElementById('requestId').value;
            const userId = document.getElementById('userId').value;
            const customHeader = document.getElementById('customHeader').value;
            
            if (requestId) headers['X-Request-ID'] = requestId;
            if (userId) headers['X-User-ID'] = userId;
            if (customHeader) headers['X-Custom-Header'] = customHeader;
            
            return headers;
        }
        
        function testEndpoint(path) {
            const startTime = Date.now();
            
            fetch(path, {
                method: 'GET',
                headers: getHeaders()
            })
            .then(response => {
                const endTime = Date.now();
                const duration = endTime - startTime;
                
                return response.text().then(text => {
                    const output = document.getElementById('output');
                    const timestamp = new Date().toISOString();
                    
                    output.textContent += '[' + timestamp + '] ' + response.status + ' ' + path + ' (' + duration + 'ms)\\n';
                    output.textContent += text + '\\n\\n';
                    output.scrollTop = output.scrollHeight;
                });
            })
            .catch(error => {
                const output = document.getElementById('output');
                output.textContent += '[ERROR] ' + error.message + '\\n\\n';
                output.scrollTop = output.scrollHeight;
            });
        }
        
        function testPost(path) {
            const startTime = Date.now();
            const payload = {
                message: "Test POST request",
                timestamp: new Date().toISOString(),
                data: { key: "value", number: 42 }
            };
            
            fetch(path, {
                method: 'POST',
                headers: getHeaders(),
                body: JSON.stringify(payload)
            })
            .then(response => {
                const endTime = Date.now();
                const duration = endTime - startTime;
                
                return response.text().then(text => {
                    const output = document.getElementById('output');
                    const timestamp = new Date().toISOString();
                    
                    output.textContent += '[' + timestamp + '] ' + response.status + ' POST ' + path + ' (' + duration + 'ms)\\n';
                    output.textContent += text + '\\n\\n';
                    output.scrollTop = output.scrollHeight;
                });
            })
            .catch(error => {
                const output = document.getElementById('output');
                output.textContent += '[ERROR] ' + error.message + '\\n\\n';
                output.scrollTop = output.scrollHeight;
            });
        }
        
        function loadTest() {
            const output = document.getElementById('output');
            output.textContent += '[LOAD TEST] Sending 10 requests to /api/simple...\\n';
            
            for (let i = 0; i < 10; i++) {
                setTimeout(() => {
                    testEndpoint('/api/simple');
                }, i * 200); // 200ms between requests
            }
        }
        
        function clearOutput() {
            document.getElementById('output').textContent = 'Output cleared. Check server console for log output...\\n';
        }
    </script>
</body>
</html>`
	w.Write([]byte(html))
}