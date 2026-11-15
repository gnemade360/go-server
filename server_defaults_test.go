package goserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultServerConfig_TurnsOffCORSCacheAndGzip(t *testing.T) {
	config := DefaultServerConfig()

	if config.Addr != ":8080" {
		t.Errorf("expected addr :8080, got %s", config.Addr)
	}
	if config.CORSOrigin != "" {
		t.Errorf("expected CORS off, got origin %q", config.CORSOrigin)
	}
	if config.CacheDuration != 0 {
		t.Errorf("expected caching off, got %v", config.CacheDuration)
	}
	if config.EnableGzip {
		t.Error("expected gzip off")
	}
}

func TestNew_ResponsesCarryNoCORSCacheOrGzip(t *testing.T) {
	server := New(":0")
	server.Router().Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(rec, req)

	for _, header := range []string{"Access-Control-Allow-Origin", "Cache-Control", "Content-Encoding"} {
		if value := rec.Header().Get(header); value != "" {
			t.Errorf("expected no %s header, got %q", header, value)
		}
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected plain body %q, got %q", "ok", rec.Body.String())
	}
}

func TestNew_ReturnsDistinctServers(t *testing.T) {
	first := New(":8081")
	second := New(":8082")

	if first == second {
		t.Fatal("expected each New call to return a new server")
	}
	if first.httpServer.Addr != ":8081" {
		t.Errorf("expected the first server to keep :8081, got %s", first.httpServer.Addr)
	}
}

func TestNew_EachServerRunsItsMiddlewareOnce(t *testing.T) {
	calls := 0
	counting := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			next.ServeHTTP(w, r)
		})
	}
	New(":0", WithMiddleware(counting))
	second := New(":0", WithMiddleware(counting))

	second.handlerChain().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if calls != 1 {
		t.Errorf("expected the middleware to run once per request, ran %d times", calls)
	}
}
