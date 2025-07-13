package goserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/go-server/filter"
	"github.com/yourorg/go-server/router"
	"github.com/yourorg/go-server/static"
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

	// Test that shutdown doesn't error on a server that hasn't started
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := server.Shutdown(ctx)
	// Should return nil since httpServer.Shutdown on non-started server succeeds
	if err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if len(opts) == 0 {
		t.Error("Expected default options to be non-empty")
	}
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Addr != ":8080" {
		t.Errorf("Expected default addr :8080, got %s", config.Addr)
	}
	if config.CORSOrigin != "*" {
		t.Errorf("Expected default CORS origin *, got %s", config.CORSOrigin)
	}
	if config.CacheDuration != 24*time.Hour {
		t.Errorf("Expected default cache duration 24h, got %v", config.CacheDuration)
	}
	if !config.EnableGzip {
		t.Error("Expected default gzip to be enabled")
	}
}

func TestServerRouter(t *testing.T) {
	server := NewServer()
	router := server.Router()

	if router == nil {
		t.Error("Expected router to be non-nil")
	}
}

func TestServerSetRouter(t *testing.T) {
	server := NewServer()
	newRouter := router.NewRouter()

	server.SetRouter(newRouter)

	if server.router != newRouter {
		t.Error("SetRouter did not set the router correctly")
	}

	// Test that the new router is returned by Router()
	if server.Router() != newRouter {
		t.Error("Router() did not return the new router")
	}
}

func TestServerHandlerChain(t *testing.T) {
	server := NewServer()
	handler := server.handlerChain()

	if handler == nil {
		t.Error("Expected handler chain to be non-nil")
	}
}

func TestServerListenAndServe(t *testing.T) {
	server := NewServer()
	// Use an invalid address to ensure quick failure
	server.httpServer.Addr = "invalid-address:99999"

	// This should fail quickly due to invalid address
	err := server.ListenAndServe()
	if err == nil {
		t.Error("Expected ListenAndServe to return an error with invalid address")
	}
}

func TestServerStart(t *testing.T) {
	server := NewServer()
	// Use an invalid address to ensure quick failure
	server.httpServer.Addr = "invalid-address:99999"

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should fail quickly due to invalid address
	err := server.Start(ctx)
	if err == nil {
		t.Error("Expected Start to return an error with invalid address")
	}
}

func TestServerConfigureWithAddr(t *testing.T) {
	server := NewServer()
	configured := server.Configure(":9999")

	if configured.httpServer.Addr != ":9999" {
		t.Errorf("Expected addr :9999, got %s", configured.httpServer.Addr)
	}
}

func TestServerConfigureWithEmptyAddr(t *testing.T) {
	server := NewServer()
	originalAddr := server.httpServer.Addr
	configured := server.Configure("")

	if configured.httpServer.Addr != originalAddr {
		t.Error("Empty addr should not change server address")
	}
}

func TestServerConfigureWithTimeout(t *testing.T) {
	config := ServerConfig{
		Timeout: 30 * time.Second,
	}
	server := NewServerWithConfig(config)
	server.Configure(":8080")

	if server.timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", server.timeout)
	}
}

func TestServerConfigureWithCORS(t *testing.T) {
	config := ServerConfig{
		CORSOrigin: "https://example.com",
	}
	server := NewServerWithConfig(config)
	server.Configure(":8080")

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "https://example.com" {
		t.Errorf("Expected CORS origin https://example.com, got %s", origin)
	}
}

func TestServerConfigureWithCacheDuration(t *testing.T) {
	config := ServerConfig{
		CacheDuration: 2 * time.Hour,
	}
	server := NewServerWithConfig(config)
	server.Configure(":8080")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(w, req)

	cacheControl := w.Header().Get("Cache-Control")
	if !strings.Contains(cacheControl, "max-age=7200") {
		t.Errorf("Expected Cache-Control with max-age=7200, got %s", cacheControl)
	}
}

func TestServerConfigureWithGzip(t *testing.T) {
	config := ServerConfig{
		EnableGzip: true,
	}
	server := NewServerWithConfig(config)
	server.Configure(":8080")

	server.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(w, req)

	encoding := w.Header().Get("Content-Encoding")
	if encoding != "gzip" {
		t.Errorf("Expected Content-Encoding gzip, got %s", encoding)
	}
}

func TestWithTimeouts(t *testing.T) {
	server := NewServer()
	option := WithTimeouts(60*time.Second, 30*time.Second, 15*time.Second)

	option(server)

	if server.httpServer.IdleTimeout != 60*time.Second {
		t.Errorf("Expected idle timeout 60s, got %v", server.httpServer.IdleTimeout)
	}
	if server.httpServer.WriteTimeout != 30*time.Second {
		t.Errorf("Expected write timeout 30s, got %v", server.httpServer.WriteTimeout)
	}
	if server.httpServer.ReadTimeout != 15*time.Second {
		t.Errorf("Expected read timeout 15s, got %v", server.httpServer.ReadTimeout)
	}
}

func TestWithMiddlewareOption(t *testing.T) {
	server := NewServer()

	testMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-MW", "applied")
			next.ServeHTTP(w, r)
		})
	}

	option := WithMiddleware(testMW)
	option(server)

	server.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(w, req)

	if w.Header().Get("X-Test-MW") != "applied" {
		t.Error("Middleware was not applied")
	}
}

func TestWithFilter(t *testing.T) {
	server := NewServer()

	testFilter := filter.NewFilterFn(1, func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		w.Header().Set("X-Test-Filter", "applied")
		next.ServeHTTP(w, r)
	})

	option := WithFilter(testFilter)
	option(server)

	server.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(w, req)

	if w.Header().Get("X-Test-Filter") != "applied" {
		t.Error("Filter was not applied")
	}
}

func TestWithFilterFn(t *testing.T) {
	server := NewServer()

	filterFn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		w.Header().Set("X-Filter-Fn", "applied")
		next.ServeHTTP(w, r)
	}

	option := WithFilterFn(1, filterFn)
	option(server)

	server.Router().Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	server.handlerChain().ServeHTTP(w, req)

	if w.Header().Get("X-Filter-Fn") != "applied" {
		t.Error("Filter function was not applied")
	}
}

func TestWithTimeout(t *testing.T) {
	server := NewServer()
	option := WithTimeout(5 * time.Second)

	option(server)

	if server.timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", server.timeout)
	}
}

func TestWithTimeoutZero(t *testing.T) {
	server := NewServer()
	option := WithTimeout(0)

	option(server)

	if server.timeout != 0 {
		t.Errorf("Expected timeout 0, got %v", server.timeout)
	}
}

func TestServerNewFilterFn(t *testing.T) {
	server := NewServer()

	filterFn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		next.ServeHTTP(w, r)
	}

	filter := server.NewFilterFn(1, filterFn)
	if filter == nil {
		t.Error("Expected filter to be non-nil")
	}
}

func TestServerNewTemplateStaticHandler(t *testing.T) {
	server := NewServer()

	data := static.TemplateData{
		RoleDescription: "Test",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}

	handler := server.NewTemplateStaticHandler("./testdata", "index.html", data)
	if handler == nil {
		t.Error("Expected template handler to be non-nil")
	}
}

func TestServerSecureJoinPath(t *testing.T) {
	server := NewServer()

	// Test normal path
	result, err := server.SecureJoinPath("/base", "subdir/file.txt")
	if err != nil {
		t.Errorf("SecureJoinPath failed: %v", err)
	}
	if !strings.Contains(result, "subdir") {
		t.Errorf("Expected path to contain 'subdir', got %s", result)
	}

	// Test path traversal attempt
	_, err = server.SecureJoinPath("/base", "../../../etc/passwd")
	if err == nil {
		t.Error("Expected SecureJoinPath to reject path traversal")
	}
}

func TestNew(t *testing.T) {
	server := New(":8080")
	if server == nil {
		t.Error("New() returned nil")
	}
	if server.httpServer.Addr != ":8080" {
		t.Errorf("Expected addr :8080, got %s", server.httpServer.Addr)
	}
}

func TestNewWithConfig(t *testing.T) {
	config := ServerConfig{
		Addr: ":9090",
	}

	server := NewWithConfig(config, ":8080")
	if server == nil {
		t.Error("NewWithConfig() returned nil")
	}
	if server.httpServer.Addr != ":8080" {
		t.Errorf("Expected addr :8080, got %s", server.httpServer.Addr)
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if config.Addr != ":8080" {
		t.Errorf("Expected default addr :8080, got %s", config.Addr)
	}
	if config.CORSOrigin != "*" {
		t.Errorf("Expected default CORS origin *, got %s", config.CORSOrigin)
	}
	if config.CacheDuration != 24*time.Hour {
		t.Errorf("Expected default cache duration 24h, got %v", config.CacheDuration)
	}
	if !config.EnableGzip {
		t.Error("Expected default gzip to be enabled")
	}
}

func TestExportedFunctions(t *testing.T) {
	// Test exported filter function
	filterFn := func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		next.ServeHTTP(w, r)
	}
	filter := NewFilterFn(1, filterFn)
	if filter == nil {
		t.Error("NewFilterFn should create a filter")
	}

	// Test exported middleware functions
	gzipMW := Gzip()
	if gzipMW == nil {
		t.Error("Gzip should create middleware")
	}

	cacheMW := CacheControl(1 * time.Hour)
	if cacheMW == nil {
		t.Error("CacheControl should create middleware")
	}

	corsMW := CORS("*")
	if corsMW == nil {
		t.Error("CORS should create middleware")
	}

	// Test exported static handler function
	data := static.TemplateData{RoleDescription: "test", HTTPAddr: ":8080", ControlAddr: ":9090"}
	handler := NewTemplateStaticHandler("./testdata", "index.html", data)
	if handler == nil {
		t.Error("NewTemplateStaticHandler should create handler")
	}
}

func TestGlobalSecureJoinPath(t *testing.T) {
	// Test normal path
	result, err := SecureJoinPath("/base", "subdir/file.txt")
	if err != nil {
		t.Errorf("SecureJoinPath failed: %v", err)
	}
	if !strings.Contains(result, "subdir") {
		t.Errorf("Expected path to contain 'subdir', got %s", result)
	}

	// Test path traversal attempt
	_, err = SecureJoinPath("/base", "../../../etc/passwd")
	if err == nil {
		t.Error("Expected SecureJoinPath to reject path traversal")
	}
}

func TestSecureJoinPathEdgeCases(t *testing.T) {
	server := NewServer()

	// Test with leading slash in relative path
	result, err := server.SecureJoinPath("/base", "/subdir/file.txt")
	if err != nil {
		t.Errorf("SecureJoinPath failed with leading slash: %v", err)
	}

	// Test with empty relative path
	result, err = server.SecureJoinPath("/base", "")
	if err != nil {
		t.Errorf("SecureJoinPath failed with empty path: %v", err)
	}
	expectedPath := filepath.Clean("/base")
	resultClean := filepath.Clean(result)
	if !strings.HasSuffix(resultClean, filepath.Base(expectedPath)) {
		t.Errorf("Expected result to end with base path component")
	}

	// Test with dot path
	result, err = server.SecureJoinPath("/base", ".")
	if err != nil {
		t.Errorf("SecureJoinPath failed with dot path: %v", err)
	}
}

func TestTypeAliases(t *testing.T) {
	// Test that type aliases are properly defined
	var mw Middleware
	var f Filter
	var cf ConditionalFilter
	var so StaticOptions
	var td TemplateData
	var tsh *TemplateStaticHandler

	// These should compile without errors
	_ = mw
	_ = f
	_ = cf
	_ = so
	_ = td
	_ = tsh
}
