package middleware

import (
	"encoding/json"
	"net/http"
	"time"
)

// LogEntry represents a structured log entry for HTTP requests
type LogEntry struct {
	Timestamp      time.Time         `json:"timestamp"`
	Method         string            `json:"method"`
	URL            string            `json:"url"`
	Protocol       string            `json:"protocol"`
	Status         int               `json:"status"`
	StatusText     string            `json:"status_text"`
	ResponseSize   int               `json:"response_size"`
	Duration       time.Duration     `json:"duration_ns"`
	DurationMS     float64           `json:"duration_ms"`
	RemoteAddr     string            `json:"remote_addr"`
	UserAgent      string            `json:"user_agent"`
	Referer        string            `json:"referer"`
	RequestID      string            `json:"request_id,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Query          map[string]string `json:"query,omitempty"`
	ErrorMessage   string            `json:"error,omitempty"`
}

// LoggingConfig holds configuration for request logging
type LoggingConfig struct {
	// IncludeHeaders specifies whether to include request headers in logs
	IncludeHeaders bool
	// IncludeQuery specifies whether to include query parameters in logs
	IncludeQuery bool
	// IncludeUserAgent specifies whether to include User-Agent header
	IncludeUserAgent bool
	// IncludeReferer specifies whether to include Referer header
	IncludeReferer bool
	// HeadersToLog specifies which headers to log (if IncludeHeaders is true)
	HeadersToLog []string
	// LogHandler is called with each log entry (defaults to JSON output)
	LogHandler func(LogEntry)
	// RequestIDHeader specifies header name containing request ID
	RequestIDHeader string
}

// DefaultLoggingConfig returns a default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		IncludeHeaders:   false,
		IncludeQuery:     true,
		IncludeUserAgent: true,
		IncludeReferer:   true,
		HeadersToLog:     []string{"Content-Type", "Authorization", "X-Forwarded-For"},
		LogHandler:       defaultLogHandler,
		RequestIDHeader:  "X-Request-ID",
	}
}

// defaultLogHandler prints log entries as JSON
func defaultLogHandler(entry LogEntry) {
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple format if JSON marshaling fails
		println("ERROR: Failed to marshal log entry:", err.Error())
		return
	}
	println(string(jsonBytes))
}

// responseWriter wraps http.ResponseWriter to capture response information
type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.status == 0 {
		rw.status = code
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(data)
	rw.size += n
	return n, err
}

// StructuredLogging creates a middleware for structured request logging
func StructuredLogging(config LoggingConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Wrap response writer to capture status and size
			wrapped := &responseWriter{ResponseWriter: w}
			
			// Handle panics and log them
			defer func() {
				if rec := recover(); rec != nil {
					wrapped.status = http.StatusInternalServerError
					
					entry := createLogEntry(r, wrapped, start, config)
					if recStr, ok := rec.(string); ok {
						entry.ErrorMessage = recStr
					} else {
						entry.ErrorMessage = "panic occurred during request processing"
					}
					
					config.LogHandler(entry)
					panic(rec) // Re-panic to let recovery middleware handle it
				}
			}()
			
			// Process request
			next.ServeHTTP(wrapped, r)
			
			// Log the completed request
			entry := createLogEntry(r, wrapped, start, config)
			config.LogHandler(entry)
		})
	}
}

// createLogEntry creates a log entry from request and response information
func createLogEntry(r *http.Request, w *responseWriter, start time.Time, config LoggingConfig) LogEntry {
	duration := time.Since(start)
	
	entry := LogEntry{
		Timestamp:    start,
		Method:       r.Method,
		URL:          r.URL.String(),
		Protocol:     r.Proto,
		Status:       w.status,
		StatusText:   http.StatusText(w.status),
		ResponseSize: w.size,
		Duration:     duration,
		DurationMS:   float64(duration.Nanoseconds()) / 1e6,
		RemoteAddr:   r.RemoteAddr,
	}
	
	// Extract User-Agent if configured
	if config.IncludeUserAgent {
		entry.UserAgent = r.Header.Get("User-Agent")
	}
	
	// Extract Referer if configured
	if config.IncludeReferer {
		entry.Referer = r.Header.Get("Referer")
	}
	
	// Extract Request ID if configured
	if config.RequestIDHeader != "" {
		entry.RequestID = r.Header.Get(config.RequestIDHeader)
	}
	
	// Include headers if configured
	if config.IncludeHeaders && len(config.HeadersToLog) > 0 {
		entry.Headers = make(map[string]string)
		for _, headerName := range config.HeadersToLog {
			if value := r.Header.Get(headerName); value != "" {
				entry.Headers[headerName] = value
			}
		}
	}
	
	// Include query parameters if configured
	if config.IncludeQuery && len(r.URL.Query()) > 0 {
		entry.Query = make(map[string]string)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				entry.Query[key] = values[0] // Take first value if multiple
			}
		}
	}
	
	return entry
}

// RequestLogging creates a simple request logging middleware
func RequestLogging() Middleware {
	config := DefaultLoggingConfig()
	return StructuredLogging(config)
}

// RequestLoggingWithHeaders creates a request logging middleware that includes headers
func RequestLoggingWithHeaders(headers ...string) Middleware {
	config := DefaultLoggingConfig()
	config.IncludeHeaders = true
	if len(headers) > 0 {
		config.HeadersToLog = headers
	}
	return StructuredLogging(config)
}

// RequestLoggingSimple creates a simple text-based request logging middleware
func RequestLoggingSimple() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w}
			
			next.ServeHTTP(wrapped, r)
			
			duration := time.Since(start)
			println(
				start.Format("2006-01-02 15:04:05"),
				r.Method,
				r.URL.String(),
				wrapped.status,
				http.StatusText(wrapped.status),
				duration.String(),
				r.RemoteAddr,
			)
		})
	}
}