package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func gzipRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	return req
}

func TestGzip_InsideRecoveryPanicGetsPlain500(t *testing.T) {
	captureLog(t)
	handler := Recovery()(Gzip()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))
	rec := httptest.NewRecorder()

	recovered := func() (recovered any) {
		defer func() { recovered = recover() }()
		handler.ServeHTTP(rec, gzipRequest())
		return nil
	}()

	if recovered != nil {
		t.Fatalf("expected Recovery to handle the panic, got %v", recovered)
	}
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "internal server error" {
		t.Errorf("expected 500 %q, got %d %q", "internal server error", rec.Code, rec.Body.String())
	}
	if encoding := rec.Header().Get("Content-Encoding"); encoding != "" {
		t.Errorf("expected no Content-Encoding on the 500, got %q", encoding)
	}
}

func TestGzip_EmptyResponseIsNotEncoded(t *testing.T) {
	handler := Gzip()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, gzipRequest())

	if encoding := rec.Header().Get("Content-Encoding"); encoding != "" {
		t.Errorf("expected no Content-Encoding, got %q", encoding)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected an empty body, got %d bytes", rec.Body.Len())
	}
}

func TestGzip_WriteHeaderStartsEncoding(t *testing.T) {
	handler := Gzip()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, gzipRequest())

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}
	if encoding := rec.Header().Get("Content-Encoding"); encoding != "gzip" {
		t.Errorf("expected Content-Encoding gzip, got %q", encoding)
	}
}

func TestGzip_InformationalStatusDoesNotStartEncoding(t *testing.T) {
	var encodingAfterHints string
	handler := Gzip()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusEarlyHints)
		encodingAfterHints = w.Header().Get("Content-Encoding")
	}))

	handler.ServeHTTP(httptest.NewRecorder(), gzipRequest())

	if encodingAfterHints != "" {
		t.Errorf("expected no Content-Encoding after a 103, got %q", encodingAfterHints)
	}
}
