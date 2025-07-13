package middleware

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddlewareType(t *testing.T) {
	// Test that Middleware is a function type
	var mw Middleware
	if mw != nil {
		t.Error("Expected nil middleware function")
	}

	// Create a simple middleware
	mw = func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "middleware")
			next.ServeHTTP(w, r)
		})
	}

	if mw == nil {
		t.Error("Expected non-nil middleware function")
	}
}

func TestMiddlewareManager_AddMiddleware(t *testing.T) {
	m := &MiddlewareManager{}

	// Initially empty
	if len(m.middleware) != 0 {
		t.Errorf("Expected 0 middleware, got %d", len(m.middleware))
	}

	mw1 := func(next http.Handler) http.Handler { return next }
	mw2 := func(next http.Handler) http.Handler { return next }

	// Add single middleware
	m.AddMiddleware(mw1)
	if len(m.middleware) != 1 {
		t.Errorf("Expected 1 middleware, got %d", len(m.middleware))
	}

	// Add multiple middleware
	m.AddMiddleware(mw2, mw1)
	if len(m.middleware) != 3 {
		t.Errorf("Expected 3 middleware, got %d", len(m.middleware))
	}
}

func TestMiddlewareManager_ApplyMiddleware(t *testing.T) {
	m := &MiddlewareManager{}

	var order []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw1-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "mw2-before")
			next.ServeHTTP(w, r)
			order = append(order, "mw2-after")
		})
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	})

	m.AddMiddleware(mw1, mw2)
	wrapped := m.ApplyMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Middleware should be applied in reverse order (last added wraps first)
	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Errorf("Expected %d events, got %d", len(expected), len(order))
	}

	for i, exp := range expected {
		if i >= len(order) || order[i] != exp {
			t.Errorf("Expected order[%d] = %s, got %s", i, exp, order[i])
		}
	}
}

func TestMiddlewareManager_ApplyMiddleware_Empty(t *testing.T) {
	m := &MiddlewareManager{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := m.ApplyMiddleware(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCacheControl(t *testing.T) {
	maxAge := 1 * time.Hour
	mw := CacheControl(maxAge)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Check Cache-Control header
	cacheControl := w.Header().Get("Cache-Control")
	expected := "public, max-age=3600"
	if cacheControl != expected {
		t.Errorf("Expected Cache-Control '%s', got '%s'", expected, cacheControl)
	}

	// Check Expires header is set
	expires := w.Header().Get("Expires")
	if expires == "" {
		t.Error("Expected Expires header to be set")
	}
}

func TestCacheControl_ZeroDuration(t *testing.T) {
	mw := CacheControl(0)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	cacheControl := w.Header().Get("Cache-Control")
	expected := "public, max-age=0"
	if cacheControl != expected {
		t.Errorf("Expected Cache-Control '%s', got '%s'", expected, cacheControl)
	}
}

func TestCORS_AllowAll(t *testing.T) {
	mw := CORS("*")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin 'https://example.com', got '%s'", origin)
	}

	vary := w.Header().Get("Vary")
	if vary != "Origin" {
		t.Errorf("Expected Vary 'Origin', got '%s'", vary)
	}
}

func TestCORS_SpecificOrigins(t *testing.T) {
	mw := CORS("https://example.com", "https://test.com")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	// Test allowed origin
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Expected Access-Control-Allow-Origin 'https://example.com', got '%s'", origin)
	}

	// Test disallowed origin
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.Header.Set("Origin", "https://evil.com")
	w2 := httptest.NewRecorder()
	wrapped.ServeHTTP(w2, req2)

	origin2 := w2.Header().Get("Access-Control-Allow-Origin")
	if origin2 != "" {
		t.Errorf("Expected no Access-Control-Allow-Origin for disallowed origin, got '%s'", origin2)
	}
}

func TestCORS_NoOrigins(t *testing.T) {
	mw := CORS()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://any.com")
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://any.com" {
		t.Errorf("Expected Access-Control-Allow-Origin 'https://any.com', got '%s'", origin)
	}
}

func TestCORS_OptionsRequest(t *testing.T) {
	mw := CORS("*")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called for OPTIONS request")
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("OPTIONS", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}

	methods := w.Header().Get("Access-Control-Allow-Methods")
	if !strings.Contains(methods, "GET") {
		t.Errorf("Expected Allow-Methods to contain 'GET', got '%s'", methods)
	}

	headers := w.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "Content-Type") {
		t.Errorf("Expected Allow-Headers to contain 'Content-Type', got '%s'", headers)
	}
}

func TestGzip_WithGzipSupport(t *testing.T) {
	mw := Gzip()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	encoding := w.Header().Get("Content-Encoding")
	if encoding != "gzip" {
		t.Errorf("Expected Content-Encoding 'gzip', got '%s'", encoding)
	}

	// Decode gzip response
	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to read gzip response: %v", err)
	}

	if string(body) != "test response" {
		t.Errorf("Expected 'test response', got '%s'", string(body))
	}
}

func TestGzip_WithoutGzipSupport(t *testing.T) {
	mw := Gzip()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	// No Accept-Encoding header
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	encoding := w.Header().Get("Content-Encoding")
	if encoding != "" {
		t.Errorf("Expected no Content-Encoding, got '%s'", encoding)
	}

	body := w.Body.String()
	if body != "test response" {
		t.Errorf("Expected 'test response', got '%s'", body)
	}
}

func TestGzipResponseWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)

	w := httptest.NewRecorder()
	grw := gzipResponseWriter{Writer: gz, ResponseWriter: w}

	n, err := grw.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("Expected 4 bytes written, got %d", n)
	}
}

func TestRecovery(t *testing.T) {
	mw := Recovery()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	wrapped := mw(handler)

	// Capture log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "internal server error" {
		t.Errorf("Expected 'internal server error', got '%s'", body)
	}

	// Check that panic was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "panic recovered") {
		t.Error("Expected panic to be logged")
	}
}

func TestRecovery_NoPanic(t *testing.T) {
	mw := Recovery()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("normal response"))
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "normal response" {
		t.Errorf("Expected 'normal response', got '%s'", body)
	}
}

func TestRecovery_AlreadyWrittenHeader(t *testing.T) {
	mw := Recovery()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("partial response"))
		panic("test panic")
	})

	wrapped := mw(handler)

	// Capture log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Status should remain as set by handler before panic
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestResponseRecorder_WriteHeader(t *testing.T) {
	w := httptest.NewRecorder()
	rr := &responseRecorder{ResponseWriter: w, wroteHeader: false}

	if rr.wroteHeader {
		t.Error("Expected wroteHeader to be false initially")
	}

	rr.WriteHeader(http.StatusAccepted)

	if !rr.wroteHeader {
		t.Error("Expected wroteHeader to be true after WriteHeader")
	}

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	mw := LoggingMiddleware()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	// Capture log output
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(nil)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "Request started: GET /test") {
		t.Error("Expected start log message")
	}
	if !strings.Contains(logOutput, "Request completed: GET /test") {
		t.Error("Expected completion log message")
	}
}

func TestTimeout_WithTimeout(t *testing.T) {
	mw := Timeout(100 * time.Millisecond)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		select {
		case <-time.After(200 * time.Millisecond):
			w.Write([]byte("should not reach here"))
		case <-ctx.Done():
			w.WriteHeader(http.StatusRequestTimeout)
			w.Write([]byte("timeout"))
		}
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status 408, got %d", w.Code)
	}
}

func TestTimeout_NoTimeout(t *testing.T) {
	mw := Timeout(0)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "success" {
		t.Errorf("Expected 'success', got '%s'", body)
	}
}

func TestTimeout_NegativeTimeout(t *testing.T) {
	mw := Timeout(-1 * time.Second)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTimeout_ContextPropagation(t *testing.T) {
	mw := Timeout(1 * time.Second)

	var receivedCtx context.Context
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	})

	wrapped := mw(handler)

	originalCtx := context.WithValue(context.Background(), "key", "value")
	req := httptest.NewRequest("GET", "/", nil).WithContext(originalCtx)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Check that original context values are preserved
	if receivedCtx.Value("key") != "value" {
		t.Error("Expected original context values to be preserved")
	}

	// Check that timeout was applied
	_, hasDeadline := receivedCtx.Deadline()
	if !hasDeadline {
		t.Error("Expected context to have deadline")
	}
}

func BenchmarkCacheControl(b *testing.B) {
	mw := CacheControl(1 * time.Hour)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkCORS(b *testing.B) {
	mw := CORS("*")
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}

func BenchmarkGzip(b *testing.B) {
	mw := Gzip()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response data"))
	})
	wrapped := mw(handler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		wrapped.ServeHTTP(w, req)
	}
}
