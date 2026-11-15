package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestRecovery_PanicAfterWriteAppendsNothing(t *testing.T) {
	captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: x\n\n"))
		panic("mid-stream")
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("expected the status already sent (200), got %d", rec.Code)
	}
	if rec.Body.String() != "data: x\n\n" {
		t.Errorf("expected body %q only, got %q", "data: x\n\n", rec.Body.String())
	}
}

func TestRecovery_PanicAfterFlushAppendsNothing(t *testing.T) {
	captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NewResponseController(w).Flush()
		panic("after flush")
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if rec.Body.Len() != 0 {
		t.Errorf("expected an empty body, got %q", rec.Body.String())
	}
}

func TestRecovery_ReraisesErrAbortHandler(t *testing.T) {
	logBuf := captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	recovered := serveAndRecover(handler)

	if recovered != http.ErrAbortHandler {
		t.Errorf("expected http.ErrAbortHandler to be re-panicked, got %v", recovered)
	}
	if logBuf.Len() != 0 {
		t.Errorf("expected nothing logged for ErrAbortHandler, got %q", logBuf.String())
	}
}

func TestRecovery_DefaultLogHasNoEmoji(t *testing.T) {
	logBuf := captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	logged := logBuf.String()
	if !strings.Contains(logged, "panic recovered: boom") {
		t.Errorf("expected the panic value in the log, got %q", logged)
	}
	if strings.Contains(logged, "‼") {
		t.Errorf("expected no emoji in the log, got %q", logged)
	}
}

func TestRecovery_DefaultResponseIsPlainText500(t *testing.T) {
	captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("expected text/plain, got %q", contentType)
	}
	if rec.Body.String() != "internal server error" {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
}

func serveAndRecover(handler http.Handler) (recovered any) {
	defer func() { recovered = recover() }()
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	return nil
}
