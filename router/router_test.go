package router

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/yourorg/go-server/static"
)

func TestNewRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("NewRouter() returned nil")
	}
	if r.routes == nil {
		t.Error("routes map is nil")
	}
	if r.notFound == nil {
		t.Error("notFound handler is nil")
	}
}

func TestRouterHandle(t *testing.T) {
	r := NewRouter()
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Test literal route
	r.Handle("GET", "/test", handler)

	if len(r.routes["GET"]) != 1 {
		t.Errorf("Expected 1 route, got %d", len(r.routes["GET"]))
	}

	route := r.routes["GET"][0]
	if route.literal != "/test" {
		t.Errorf("Expected literal '/test', got '%s'", route.literal)
	}
	if route.pattern != nil {
		t.Error("Expected pattern to be nil for literal route")
	}

	// Test regex route
	r.Handle("POST", "^/api/users/([0-9]+)$", handler)

	if len(r.routes["POST"]) != 1 {
		t.Errorf("Expected 1 POST route, got %d", len(r.routes["POST"]))
	}

	postRoute := r.routes["POST"][0]
	if postRoute.literal != "" {
		t.Error("Expected literal to be empty for regex route")
	}
	if postRoute.pattern == nil {
		t.Error("Expected pattern to be non-nil for regex route")
	}
	if postRoute.pattern.String() != "^/api/users/([0-9]+)$" {
		t.Errorf("Expected pattern '^/api/users/([0-9]+)$', got '%s'", postRoute.pattern.String())
	}
}

func TestRouterHandleOverride(t *testing.T) {
	r := NewRouter()

	handler1 := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("handler1"))
	})
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("handler2"))
	})

	// Add first handler
	r.Handle("GET", "/test", handler1)

	// Override with second handler
	r.Handle("GET", "/test", handler2)

	// Should still have only one route
	if len(r.routes["GET"]) != 1 {
		t.Errorf("Expected 1 route after override, got %d", len(r.routes["GET"]))
	}

	// Test that the override worked
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if body != "handler2" {
		t.Errorf("Expected 'handler2', got '%s'", body)
	}
}

func TestRouterHandleRegexOverride(t *testing.T) {
	r := NewRouter()

	handler1 := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("handler1"))
	})
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("handler2"))
	})

	pattern := "^/api/users/([0-9]+)$"

	// Add first handler
	r.Handle("GET", pattern, handler1)

	// Override with second handler using same pattern
	r.Handle("GET", pattern, handler2)

	// Should still have only one route
	if len(r.routes["GET"]) != 1 {
		t.Errorf("Expected 1 route after regex override, got %d", len(r.routes["GET"]))
	}

	// Test that the override worked
	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body := w.Body.String()
	if body != "handler2" {
		t.Errorf("Expected 'handler2', got '%s'", body)
	}
}

func TestRouterHandleFunc(t *testing.T) {
	r := NewRouter()
	handlerFunc := func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("test func"))
	}

	r.HandleFunc("GET", "/func", handlerFunc)

	if len(r.routes["GET"]) != 1 {
		t.Errorf("Expected 1 route, got %d", len(r.routes["GET"]))
	}

	req := httptest.NewRequest("GET", "/func", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "test func" {
		t.Errorf("Expected 'test func', got '%s'", w.Body.String())
	}
}

func TestRouterGet(t *testing.T) {
	r := NewRouter()
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("GET response"))
	}

	r.Get("/get-test", handler)

	if len(r.routes["GET"]) != 1 {
		t.Errorf("Expected 1 GET route, got %d", len(r.routes["GET"]))
	}

	req := httptest.NewRequest("GET", "/get-test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "GET response" {
		t.Errorf("Expected 'GET response', got '%s'", w.Body.String())
	}
}

func TestRouterPost(t *testing.T) {
	r := NewRouter()
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("POST response"))
	}

	r.Post("/post-test", handler)

	if len(r.routes["POST"]) != 1 {
		t.Errorf("Expected 1 POST route, got %d", len(r.routes["POST"]))
	}

	req := httptest.NewRequest("POST", "/post-test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "POST response" {
		t.Errorf("Expected 'POST response', got '%s'", w.Body.String())
	}
}

func TestRouterStatic(t *testing.T) {
	r := NewRouter()

	// Create a mock static handler for testing
	opts := static.Options{
		Prefix: "/static/",
		Dir:    "./testdata",
	}

	r.Static(opts)

	if len(r.routes["GET"]) != 1 {
		t.Errorf("Expected 1 GET route for static, got %d", len(r.routes["GET"]))
	}

	route := r.routes["GET"][0]
	if route.pattern == nil {
		t.Error("Expected regex pattern for static route")
	}

	expectedPattern := "^" + regexp.QuoteMeta("/static/") + "(.*)?$"
	if route.pattern.String() != expectedPattern {
		t.Errorf("Expected pattern '%s', got '%s'", expectedPattern, route.pattern.String())
	}
}

func TestRouterStaticDefaultPrefix(t *testing.T) {
	r := NewRouter()

	opts := static.Options{
		Prefix: "", // Empty prefix should default to "/"
		Dir:    "./testdata",
	}

	r.Static(opts)

	route := r.routes["GET"][0]
	expectedPattern := "^" + regexp.QuoteMeta("/") + "(.*)?$"
	if route.pattern.String() != expectedPattern {
		t.Errorf("Expected pattern '%s', got '%s'", expectedPattern, route.pattern.String())
	}
}

func TestRouterServeHTTPLiteralRoute(t *testing.T) {
	r := NewRouter()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("literal match"))
	})

	r.Handle("GET", "/exact", handler)

	// Test exact match
	req := httptest.NewRequest("GET", "/exact", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "literal match" {
		t.Errorf("Expected 'literal match', got '%s'", w.Body.String())
	}
}

func TestRouterServeHTTPRegexRoute(t *testing.T) {
	r := NewRouter()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("regex match"))
	})

	r.Handle("GET", "^/api/users/([0-9]+)$", handler)

	// Test regex match
	req := httptest.NewRequest("GET", "/api/users/123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if w.Body.String() != "regex match" {
		t.Errorf("Expected 'regex match', got '%s'", w.Body.String())
	}
}

func TestRouterServeHTTPNotFound(t *testing.T) {
	r := NewRouter()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("found"))
	})

	r.Handle("GET", "/exists", handler)

	// Test not found
	req := httptest.NewRequest("GET", "/notfound", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRouterServeHTTPMethodNotFound(t *testing.T) {
	r := NewRouter()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("GET handler"))
	})

	r.Handle("GET", "/test", handler)

	// Test POST to GET route (should be not found)
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestRouterServeHTTPMultipleRoutes(t *testing.T) {
	r := NewRouter()

	handler1 := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("handler1"))
	})
	handler2 := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("handler2"))
	})

	r.Handle("GET", "/route1", handler1)
	r.Handle("GET", "/route2", handler2)

	// Test first route
	req1 := httptest.NewRequest("GET", "/route1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Body.String() != "handler1" {
		t.Errorf("Expected 'handler1', got '%s'", w1.Body.String())
	}

	// Test second route
	req2 := httptest.NewRequest("GET", "/route2", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Body.String() != "handler2" {
		t.Errorf("Expected 'handler2', got '%s'", w2.Body.String())
	}
}

func TestRouterServeHTTPLiteralVsRegex(t *testing.T) {
	r := NewRouter()

	literalHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("literal"))
	})
	regexHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("regex"))
	})

	// Add regex route first
	r.Handle("GET", "^/test.*$", regexHandler)
	// Add literal route second
	r.Handle("GET", "/test", literalHandler)

	// Test that routes are processed in order (first match wins)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The first route added (regex) should match first
	if w.Body.String() != "regex" {
		t.Errorf("Expected 'regex', got '%s'", w.Body.String())
	}
}

func TestRouterStaticOptionsType(t *testing.T) {
	// Test that StaticOptions is properly exported as an alias
	var opts StaticOptions
	opts.Prefix = "/test/"
	opts.Dir = "./testdata"

	if opts.Prefix != "/test/" {
		t.Errorf("Expected prefix '/test/', got '%s'", opts.Prefix)
	}
	if opts.Dir != "./testdata" {
		t.Errorf("Expected dir './testdata', got '%s'", opts.Dir)
	}
}

func TestRouterInvalidRegex(t *testing.T) {
	r := NewRouter()

	// This should panic due to invalid regex
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid regex, but didn't panic")
		}
	}()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})
	r.Handle("GET", "^[invalid", handler) // Invalid regex
}

func TestRouterRouteStruct(t *testing.T) {
	// Test route struct fields directly
	re := regexp.MustCompile("^/test$")
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})

	// Test literal route
	literalRoute := route{
		literal: "/test",
		pattern: nil,
		handler: handler,
	}

	if literalRoute.literal != "/test" {
		t.Errorf("Expected literal '/test', got '%s'", literalRoute.literal)
	}
	if literalRoute.pattern != nil {
		t.Error("Expected pattern to be nil")
	}

	// Test regex route
	regexRoute := route{
		literal: "",
		pattern: re,
		handler: handler,
	}

	if regexRoute.literal != "" {
		t.Errorf("Expected empty literal, got '%s'", regexRoute.literal)
	}
	if regexRoute.pattern == nil {
		t.Error("Expected pattern to be non-nil")
	}
}

func TestRouterConcurrentAccess(t *testing.T) {
	r := NewRouter()

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("concurrent"))
	})

	r.Handle("GET", "/concurrent", handler)

	// Test concurrent access
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest("GET", "/concurrent", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Body.String() != "concurrent" {
				t.Errorf("Expected 'concurrent', got '%s'", w.Body.String())
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkRouterLiteralRoute(b *testing.B) {
	r := NewRouter()
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})
	r.Handle("GET", "/benchmark", handler)

	req := httptest.NewRequest("GET", "/benchmark", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRouterRegexRoute(b *testing.B) {
	r := NewRouter()
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})
	r.Handle("GET", "^/api/users/([0-9]+)$", handler)

	req := httptest.NewRequest("GET", "/api/users/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
