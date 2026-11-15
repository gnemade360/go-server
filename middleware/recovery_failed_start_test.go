package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// plainWriter implements only http.ResponseWriter: no Flush, no Hijack.
type plainWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func newPlainWriter() *plainWriter {
	return &plainWriter{header: http.Header{}}
}

func (p *plainWriter) Header() http.Header { return p.header }

func (p *plainWriter) WriteHeader(code int) { p.code = code }

func (p *plainWriter) Write(b []byte) (int, error) {
	if p.code == 0 {
		p.code = http.StatusOK
	}
	return p.body.Write(b)
}

func TestRecovery_FailedHijackThenPanicStillResponds500(t *testing.T) {
	captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, _, err := http.NewResponseController(w).Hijack(); err == nil {
			t.Fatal("expected Hijack on a ResponseRecorder to fail")
		}
		panic("after failed hijack")
	}))

	rec, recovered := serveRecorderAndRecover(handler)

	if recovered != nil {
		t.Fatalf("expected Recovery to handle the panic, got %v", recovered)
	}
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "internal server error" {
		t.Errorf("expected 500 %q, got %d %q", "internal server error", rec.Code, rec.Body.String())
	}
}

func TestRecovery_FailedFlushThenPanicStillResponds500(t *testing.T) {
	captureLog(t)
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NewResponseController(w).Flush()
		panic("after failed flush")
	}))
	writer := newPlainWriter()

	recovered := func() (recovered any) {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/", nil))
		return nil
	}()

	if recovered != nil {
		t.Fatalf("expected Recovery to handle the panic, got %v", recovered)
	}
	if writer.code != http.StatusInternalServerError || writer.body.String() != "internal server error" {
		t.Errorf("expected 500 %q, got %d %q", "internal server error", writer.code, writer.body.String())
	}
}
