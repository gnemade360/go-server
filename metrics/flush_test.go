package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMetrics_PassesFlushThrough(t *testing.T) {
	httpMetrics := NewHTTPMetrics(NewRegistry())
	rec := httptest.NewRecorder()
	handler := httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: first\n\n"))

		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Fatalf("Flush through metrics middleware failed: %v", err)
		}
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("wrapped writer does not implement http.Flusher")
		}
		if !rec.Flushed {
			t.Fatal("expected the underlying writer to be flushed before the handler returned")
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
}
