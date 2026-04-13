package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultLoggingConfig(t *testing.T) {
	config := DefaultLoggingConfig()
	
	if config.IncludeHeaders {
		t.Error("Expected IncludeHeaders to be false by default")
	}
	
	if !config.IncludeQuery {
		t.Error("Expected IncludeQuery to be true by default")
	}
	
	if !config.IncludeUserAgent {
		t.Error("Expected IncludeUserAgent to be true by default")
	}
	
	if !config.IncludeReferer {
		t.Error("Expected IncludeReferer to be true by default")
	}
	
	if config.RequestIDHeader != "X-Request-ID" {
		t.Errorf("Expected RequestIDHeader to be 'X-Request-ID', got %s", config.RequestIDHeader)
	}
	
	if config.LogHandler == nil {
		t.Error("Expected LogHandler to be set")
	}
	
	expectedHeaders := []string{"Content-Type", "Authorization", "X-Forwarded-For"}
	if len(config.HeadersToLog) != len(expectedHeaders) {
		t.Errorf("Expected %d headers to log, got %d", len(expectedHeaders), len(config.HeadersToLog))
	}
}

func TestResponseWriter(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapped := &responseWriter{ResponseWriter: recorder}
	
	// Test WriteHeader
	wrapped.WriteHeader(http.StatusCreated)
	if wrapped.status != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, wrapped.status)
	}
	
	// Test multiple WriteHeader calls (should keep first status)
	wrapped.WriteHeader(http.StatusBadRequest)
	if wrapped.status != http.StatusCreated {
		t.Errorf("Expected status to remain %d, got %d", http.StatusCreated, wrapped.status)
	}
	
	// Test Write
	data := []byte("Hello, World!")
	n, err := wrapped.Write(data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}
	
	if wrapped.size != len(data) {
		t.Errorf("Expected size %d, got %d", len(data), wrapped.size)
	}
	
	// Test implicit 200 status when Write is called without WriteHeader
	wrapped2 := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	wrapped2.Write([]byte("test"))
	if wrapped2.status != http.StatusOK {
		t.Errorf("Expected implicit status %d, got %d", http.StatusOK, wrapped2.status)
	}
}

func TestCreateLogEntry(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?param=value", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Referer", "https://example.com")
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.1:12345"
	
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		status:        http.StatusOK,
		size:          100,
	}
	
	start := time.Now()
	time.Sleep(1 * time.Millisecond) // Ensure some duration
	
	config := DefaultLoggingConfig()
	config.IncludeHeaders = true
	
	entry := createLogEntry(req, wrapped, start, config)
	
	// Test basic fields
	if entry.Method != "GET" {
		t.Errorf("Expected method GET, got %s", entry.Method)
	}
	
	if entry.URL != "/test?param=value" {
		t.Errorf("Expected URL '/test?param=value', got %s", entry.URL)
	}
	
	if entry.Status != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, entry.Status)
	}
	
	if entry.StatusText != "OK" {
		t.Errorf("Expected status text 'OK', got %s", entry.StatusText)
	}
	
	if entry.ResponseSize != 100 {
		t.Errorf("Expected response size 100, got %d", entry.ResponseSize)
	}
	
	if entry.RemoteAddr != "192.168.1.1:12345" {
		t.Errorf("Expected remote addr '192.168.1.1:12345', got %s", entry.RemoteAddr)
	}
	
	if entry.UserAgent != "test-agent" {
		t.Errorf("Expected user agent 'test-agent', got %s", entry.UserAgent)
	}
	
	if entry.Referer != "https://example.com" {
		t.Errorf("Expected referer 'https://example.com', got %s", entry.Referer)
	}
	
	if entry.RequestID != "req-123" {
		t.Errorf("Expected request ID 'req-123', got %s", entry.RequestID)
	}
	
	// Test duration
	if entry.Duration <= 0 {
		t.Error("Expected positive duration")
	}
	
	if entry.DurationMS <= 0 {
		t.Error("Expected positive duration in milliseconds")
	}
	
	// Test headers
	if entry.Headers == nil {
		t.Error("Expected headers to be included")
	}
	
	if entry.Headers["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type header 'application/json', got %s", entry.Headers["Content-Type"])
	}
	
	// Test query parameters
	if entry.Query == nil {
		t.Error("Expected query parameters to be included")
	}
	
	if entry.Query["param"] != "value" {
		t.Errorf("Expected query param 'value', got %s", entry.Query["param"])
	}
}

func TestCreateLogEntryWithoutOptionalFields(t *testing.T) {
	req := httptest.NewRequest("POST", "/api", nil)
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		status:        http.StatusCreated,
		size:          50,
	}
	
	config := LoggingConfig{
		IncludeHeaders:   false,
		IncludeQuery:     false,
		IncludeUserAgent: false,
		IncludeReferer:   false,
		RequestIDHeader:  "",
	}
	
	entry := createLogEntry(req, wrapped, time.Now(), config)
	
	if entry.UserAgent != "" {
		t.Errorf("Expected empty user agent, got %s", entry.UserAgent)
	}
	
	if entry.Referer != "" {
		t.Errorf("Expected empty referer, got %s", entry.Referer)
	}
	
	if entry.RequestID != "" {
		t.Errorf("Expected empty request ID, got %s", entry.RequestID)
	}
	
	if entry.Headers != nil {
		t.Error("Expected headers to be nil")
	}
	
	if entry.Query != nil {
		t.Error("Expected query to be nil")
	}
}

func TestStructuredLoggingMiddleware(t *testing.T) {
	var capturedEntry LogEntry
	config := DefaultLoggingConfig()
	config.LogHandler = func(entry LogEntry) {
		capturedEntry = entry
	}
	
	middleware := StructuredLogging(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("test response"))
	}))
	
	req := httptest.NewRequest("PUT", "/api/test?key=value", nil)
	req.Header.Set("User-Agent", "test-client")
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	// Verify response
	if recorder.Code != http.StatusAccepted {
		t.Errorf("Expected status %d, got %d", http.StatusAccepted, recorder.Code)
	}
	
	if recorder.Body.String() != "test response" {
		t.Errorf("Expected body 'test response', got %s", recorder.Body.String())
	}
	
	// Verify logged entry
	if capturedEntry.Method != "PUT" {
		t.Errorf("Expected logged method PUT, got %s", capturedEntry.Method)
	}
	
	if capturedEntry.Status != http.StatusAccepted {
		t.Errorf("Expected logged status %d, got %d", http.StatusAccepted, capturedEntry.Status)
	}
	
	if capturedEntry.UserAgent != "test-client" {
		t.Errorf("Expected logged user agent 'test-client', got %s", capturedEntry.UserAgent)
	}
	
	if capturedEntry.Query["key"] != "value" {
		t.Errorf("Expected query param 'value', got %s", capturedEntry.Query["key"])
	}
}

func TestStructuredLoggingWithPanic(t *testing.T) {
	var capturedEntry LogEntry
	config := DefaultLoggingConfig()
	config.LogHandler = func(entry LogEntry) {
		capturedEntry = entry
	}
	
	middleware := StructuredLogging(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	
	req := httptest.NewRequest("GET", "/panic", nil)
	recorder := httptest.NewRecorder()
	
	// Should panic but log the error
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to be re-raised")
		} else {
			// Verify logged entry includes error
			if capturedEntry.Status != http.StatusInternalServerError {
				t.Errorf("Expected logged status %d, got %d", http.StatusInternalServerError, capturedEntry.Status)
			}
			
			if capturedEntry.ErrorMessage != "test panic" {
				t.Errorf("Expected error message 'test panic', got %s", capturedEntry.ErrorMessage)
			}
		}
	}()
	
	handler.ServeHTTP(recorder, req)
}

func TestRequestLogging(t *testing.T) {
	middleware := RequestLogging()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestLoggingWithHeaders(t *testing.T) {
	middleware := RequestLoggingWithHeaders("Custom-Header", "Another-Header")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Custom-Header", "custom-value")
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}
}

func TestRequestLoggingSimple(t *testing.T) {
	middleware := RequestLoggingSimple()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Simple"))
	}))
	
	req := httptest.NewRequest("POST", "/simple", nil)
	recorder := httptest.NewRecorder()
	
	handler.ServeHTTP(recorder, req)
	
	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	
	if recorder.Body.String() != "Simple" {
		t.Errorf("Expected body 'Simple', got %s", recorder.Body.String())
	}
}

func TestDefaultLogHandler(t *testing.T) {
	entry := LogEntry{
		Timestamp:    time.Now(),
		Method:       "GET",
		URL:          "/test",
		Status:       200,
		StatusText:   "OK",
		ResponseSize: 100,
		Duration:     time.Millisecond * 50,
		DurationMS:   50.0,
		RemoteAddr:   "127.0.0.1:12345",
		UserAgent:    "test-agent",
	}
	
	// This test primarily ensures defaultLogHandler doesn't panic
	// In a real scenario, you'd capture stdout to verify output
	defaultLogHandler(entry)
}

func TestLogEntryJSONMarshaling(t *testing.T) {
	entry := LogEntry{
		Timestamp:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Method:       "GET",
		URL:          "/api/test",
		Protocol:     "HTTP/1.1",
		Status:       200,
		StatusText:   "OK",
		ResponseSize: 150,
		Duration:     time.Millisecond * 25,
		DurationMS:   25.0,
		RemoteAddr:   "192.168.1.100:54321",
		UserAgent:    "Go-Test-Client",
		Referer:      "https://example.com",
		RequestID:    "req-456",
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Query: map[string]string{
			"q": "search term",
		},
	}
	
	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		t.Errorf("Failed to marshal LogEntry: %v", err)
	}
	
	// Verify it's valid JSON by unmarshaling
	var unmarshaled LogEntry
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Errorf("Failed to unmarshal LogEntry: %v", err)
	}
	
	// Verify key fields
	if unmarshaled.Method != entry.Method {
		t.Errorf("Expected method %s, got %s", entry.Method, unmarshaled.Method)
	}
	
	if unmarshaled.Status != entry.Status {
		t.Errorf("Expected status %d, got %d", entry.Status, unmarshaled.Status)
	}
	
	if unmarshaled.DurationMS != entry.DurationMS {
		t.Errorf("Expected duration %f, got %f", entry.DurationMS, unmarshaled.DurationMS)
	}
}

func TestMultipleQueryValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/test?key=value1&key=value2&other=single", nil)
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		status:        http.StatusOK,
	}
	
	config := DefaultLoggingConfig()
	entry := createLogEntry(req, wrapped, time.Now(), config)
	
	// Should take the first value when multiple values exist
	if entry.Query["key"] != "value1" {
		t.Errorf("Expected first query value 'value1', got %s", entry.Query["key"])
	}
	
	if entry.Query["other"] != "single" {
		t.Errorf("Expected query value 'single', got %s", entry.Query["other"])
	}
}

func TestEmptyHeadersAndQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	wrapped := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		status:        http.StatusOK,
	}
	
	config := DefaultLoggingConfig()
	config.IncludeHeaders = true
	
	entry := createLogEntry(req, wrapped, time.Now(), config)
	
	// Should not include empty headers map
	if entry.Headers != nil && len(entry.Headers) > 0 {
		t.Error("Expected headers to be empty or nil")
	}
	
	// Should not include empty query map
	if entry.Query != nil && len(entry.Query) > 0 {
		t.Error("Expected query to be empty or nil")
	}
}

func BenchmarkStructuredLogging(b *testing.B) {
	config := DefaultLoggingConfig()
	config.LogHandler = func(LogEntry) {} // No-op handler for benchmarking
	
	middleware := StructuredLogging(config)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	
	req := httptest.NewRequest("GET", "/benchmark?param=value", nil)
	req.Header.Set("User-Agent", "benchmark-client")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
}

func BenchmarkSimpleLogging(b *testing.B) {
	middleware := RequestLoggingSimple()
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	req := httptest.NewRequest("GET", "/benchmark", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
	}
}