package static

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptions(t *testing.T) {
	// Test that Options struct is properly defined
	opts := Options{
		Dir:         "./testdata",
		Prefix:      "/static/",
		IndexFile:   "home.html",
		StripPrefix: true,
		HotReload:   true,
	}

	if opts.Dir != "./testdata" {
		t.Errorf("Expected Dir './testdata', got %s", opts.Dir)
	}
	if opts.Prefix != "/static/" {
		t.Errorf("Expected Prefix '/static/', got %s", opts.Prefix)
	}
	if opts.IndexFile != "home.html" {
		t.Errorf("Expected IndexFile 'home.html', got %s", opts.IndexFile)
	}
	if !opts.StripPrefix {
		t.Error("Expected StripPrefix to be true")
	}
	if !opts.HotReload {
		t.Error("Expected HotReload to be true")
	}
}

func TestNewStaticHandler_DefaultOptions(t *testing.T) {
	handler := NewStaticHandler(Options{})

	staticHandler, ok := handler.(*StaticHandler)
	if !ok {
		t.Fatal("Expected handler to be *StaticHandler")
	}

	if staticHandler.options.Dir != "." {
		t.Errorf("Expected default Dir '.', got %s", staticHandler.options.Dir)
	}
	if staticHandler.options.Prefix != "/" {
		t.Errorf("Expected default Prefix '/', got %s", staticHandler.options.Prefix)
	}
	if staticHandler.options.IndexFile != "index.html" {
		t.Errorf("Expected default IndexFile 'index.html', got %s", staticHandler.options.IndexFile)
	}
	if staticHandler.options.StripPrefix {
		t.Error("Expected default StripPrefix to be false")
	}
	if staticHandler.options.HotReload {
		t.Error("Expected default HotReload to be false")
	}
}

func TestNewStaticHandler_CustomOptions(t *testing.T) {
	opts := Options{
		Dir:         "./custom",
		Prefix:      "/assets/",
		IndexFile:   "main.html",
		StripPrefix: true,
		HotReload:   true,
	}

	handler := NewStaticHandler(opts)

	staticHandler, ok := handler.(*StaticHandler)
	if !ok {
		t.Fatal("Expected handler to be *StaticHandler")
	}

	if staticHandler.options.Dir != "./custom" {
		t.Errorf("Expected Dir './custom', got %s", staticHandler.options.Dir)
	}
	if staticHandler.options.Prefix != "/assets/" {
		t.Errorf("Expected Prefix '/assets/', got %s", staticHandler.options.Prefix)
	}
	if staticHandler.options.IndexFile != "main.html" {
		t.Errorf("Expected IndexFile 'main.html', got %s", staticHandler.options.IndexFile)
	}
	if !staticHandler.options.StripPrefix {
		t.Error("Expected StripPrefix to be true")
	}
	if !staticHandler.options.HotReload {
		t.Error("Expected HotReload to be true")
	}
}

func TestStaticHandler_ServeHTTP_HotReload(t *testing.T) {
	// Create a temporary directory and file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	opts := Options{
		Dir:       tempDir,
		HotReload: true,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check hot reload headers
	if w.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
		t.Error("Expected no-cache Cache-Control header for hot reload")
	}
	if w.Header().Get("Pragma") != "no-cache" {
		t.Error("Expected no-cache Pragma header for hot reload")
	}
	if w.Header().Get("Expires") != "0" {
		t.Error("Expected Expires header to be '0' for hot reload")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestStaticHandler_ServeHTTP_NoHotReload(t *testing.T) {
	// Create a temporary directory and file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	opts := Options{
		Dir:       tempDir,
		HotReload: false,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Check that hot reload headers are not set
	if w.Header().Get("Cache-Control") != "" {
		t.Error("Expected no Cache-Control header without hot reload")
	}
	if w.Header().Get("Pragma") != "" {
		t.Error("Expected no Pragma header without hot reload")
	}
	if w.Header().Get("Expires") != "" {
		t.Error("Expected no Expires header without hot reload")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestStaticHandler_ServeHTTP_StripPrefix(t *testing.T) {
	// Create a temporary directory and file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	opts := Options{
		Dir:         tempDir,
		Prefix:      "/static/",
		StripPrefix: true,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/static/test.txt", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "test content" {
		t.Errorf("Expected 'test content', got %s", body)
	}
}

func TestStaticHandler_ServeHTTP_StripPrefixRoot(t *testing.T) {
	// Create a temporary directory and index file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	indexFile := filepath.Join(tempDir, "index.html")
	err = os.WriteFile(indexFile, []byte("index content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write index file: %v", err)
	}

	opts := Options{
		Dir:         tempDir,
		Prefix:      "/static/",
		StripPrefix: true,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/static/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestStaticHandler_ServeHTTP_DirectoryWithIndex(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	indexFile := filepath.Join(subDir, "index.html")
	err = os.WriteFile(indexFile, []byte("subdir index"), 0644)
	if err != nil {
		t.Fatalf("Failed to write index file: %v", err)
	}

	opts := Options{
		Dir: tempDir,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/subdir/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "subdir index") {
		t.Error("Expected response to contain 'subdir index'")
	}
}

func TestStaticHandler_ServeHTTP_FileNotFound_FallbackToIndex(t *testing.T) {
	// Create a temporary directory and index file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	indexFile := filepath.Join(tempDir, "index.html")
	err = os.WriteFile(indexFile, []byte("fallback index"), 0644)
	if err != nil {
		t.Fatalf("Failed to write index file: %v", err)
	}

	opts := Options{
		Dir: tempDir,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "fallback index") {
		t.Error("Expected response to contain 'fallback index'")
	}
}

func TestStaticHandler_ServeHTTP_FileNotFound_NoFallback(t *testing.T) {
	// Create a temporary directory without index file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	opts := Options{
		Dir: tempDir,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestStaticHandler_ServeHTTP_CustomIndexFile(t *testing.T) {
	// Create a temporary directory and custom index file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	customIndexFile := filepath.Join(tempDir, "main.html")
	err = os.WriteFile(customIndexFile, []byte("custom index"), 0644)
	if err != nil {
		t.Fatalf("Failed to write custom index file: %v", err)
	}

	opts := Options{
		Dir:       tempDir,
		IndexFile: "main.html",
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "custom index") {
		t.Error("Expected response to contain 'custom index'")
	}
}

func BenchmarkStaticHandler_ServeHTTP(b *testing.B) {
	// Create a temporary directory and file
	tempDir, err := os.MkdirTemp("", "static_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("benchmark content"), 0644)
	if err != nil {
		b.Fatalf("Failed to write test file: %v", err)
	}

	opts := Options{
		Dir: tempDir,
	}
	handler := NewStaticHandler(opts)

	req := httptest.NewRequest("GET", "/test.txt", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkNewStaticHandler(b *testing.B) {
	opts := Options{
		Dir:         "./testdata",
		Prefix:      "/static/",
		IndexFile:   "index.html",
		StripPrefix: true,
		HotReload:   false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewStaticHandler(opts)
	}
}
