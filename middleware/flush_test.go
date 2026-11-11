package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func flushingHandler(t *testing.T, rec *httptest.ResponseRecorder) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: first\n\n"))

		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatalf("Flush through middleware failed: %v", err)
		}
		if !rec.Flushed {
			t.Fatal("expected the underlying writer to be flushed before the handler returned")
		}
		if rec.Body.Len() == 0 {
			t.Fatal("expected bytes to reach the client before the handler returned")
		}
	})
}

func TestStructuredLogging_PassesFlushThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	config := DefaultLoggingConfig()
	config.LogHandler = func(LogEntry) {}
	handler := StructuredLogging(config)(flushingHandler(t, rec))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestStructuredLogging_FlusherTypeAssertion(t *testing.T) {
	rec := httptest.NewRecorder()
	config := DefaultLoggingConfig()
	config.LogHandler = func(LogEntry) {}
	handler := StructuredLogging(config)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("wrapped writer does not implement http.Flusher")
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
}

func TestGzip_PassesFlushThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := Gzip()(flushingHandler(t, rec))
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	handler.ServeHTTP(rec, req)

	reader, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("response is not valid gzip: %v", err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading gzip body: %v", err)
	}
	if string(body) != "data: first\n\n" {
		t.Errorf("unexpected body %q", body)
	}
}
