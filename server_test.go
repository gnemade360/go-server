package goserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	server := NewServer()
	if server == nil {
		t.Fatal("NewServer() returned nil")
	}

	if server.Config.Addr != ":8080" {
		t.Errorf("Expected default addr :8080, got %s", server.Config.Addr)
	}
}

func TestNewServerWithConfig(t *testing.T) {
	config := ServerConfig{
		Addr:          ":9090",
		CORSOrigin:    "*",
		EnableGzip:    true,
		CacheDuration: 1 * time.Hour,
	}

	server := NewServerWithConfig(config)
	if server == nil {
		t.Fatal("NewServerWithConfig() returned nil")
	}

	if server.Config.Addr != ":9090" {
		t.Errorf("Expected addr :9090, got %s", server.Config.Addr)
	}
}

func TestServerRouting(t *testing.T) {
	server := NewServer()

	// Add a test route
	server.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Handle the request
	server.handlerChain().ServeHTTP(w, req)

	// Check the response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "test response" {
		t.Errorf("Expected 'test response', got %s", w.Body.String())
	}
}

func TestServerConfiguration(t *testing.T) {
	server := NewServer()

	// Configure the server
	configured := server.Configure(":3000")

	if configured.httpServer.Addr != ":3000" {
		t.Errorf("Expected addr :3000, got %s", configured.httpServer.Addr)
	}
}

func TestWithMiddleware(t *testing.T) {
	server := NewServer()

	// Test middleware that adds a header
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test", "middleware-applied")
			next.ServeHTTP(w, r)
		})
	}

	server.Configure(":8080", WithMiddleware(testMiddleware))

	// Add a test route
	server.Router().Get("/middleware-test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a test request
	req := httptest.NewRequest("GET", "/middleware-test", nil)
	w := httptest.NewRecorder()

	// Handle the request
	server.handlerChain().ServeHTTP(w, req)

	// Check that middleware was applied
	if w.Header().Get("X-Test") != "middleware-applied" {
		t.Error("Middleware was not applied correctly")
	}
}

func TestServerShutdown(t *testing.T) {
	server := NewServer()
	server.Configure(":0") // Use any available port

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test that shutdown doesn't error
	err := server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}
