package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type unwrapper interface {
	Unwrap() http.ResponseWriter
}

func assertUnwrapsTo(t *testing.T, w http.ResponseWriter, inner http.ResponseWriter) {
	t.Helper()
	wrapped, ok := w.(unwrapper)
	if !ok {
		t.Fatal("wrapped writer does not implement Unwrap")
	}
	if wrapped.Unwrap() != inner {
		t.Error("Unwrap did not return the inner writer")
	}
}

func TestStructuredLogging_UnwrapReturnsInnerWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	config := DefaultLoggingConfig()
	config.LogHandler = func(LogEntry) {}
	handler := StructuredLogging(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertUnwrapsTo(t, w, rec)
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestGzip_UnwrapReturnsInnerWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := Gzip()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertUnwrapsTo(t, w, rec)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rec, req)
}

func TestStructuredLogging_FlushBeforeWriteLogsStatus200(t *testing.T) {
	var logged LogEntry
	config := DefaultLoggingConfig()
	config.LogHandler = func(entry LogEntry) { logged = entry }
	handler := StructuredLogging(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NewResponseController(w).Flush()
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/stream", nil))

	if logged.Status != http.StatusOK {
		t.Errorf("expected logged status 200 after a flush with no write, got %d", logged.Status)
	}
}
