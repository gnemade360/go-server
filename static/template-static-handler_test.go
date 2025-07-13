package static

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateData(t *testing.T) {
	// Test that TemplateData struct is properly defined
	data := TemplateData{
		RoleDescription: "Test Server",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}

	if data.RoleDescription != "Test Server" {
		t.Errorf("Expected RoleDescription 'Test Server', got %s", data.RoleDescription)
	}
	if data.HTTPAddr != ":8080" {
		t.Errorf("Expected HTTPAddr ':8080', got %s", data.HTTPAddr)
	}
	if data.ControlAddr != ":9090" {
		t.Errorf("Expected ControlAddr ':9090', got %s", data.ControlAddr)
	}
}

func TestNewTemplateStaticHandler(t *testing.T) {
	data := TemplateData{
		RoleDescription: "Test",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}

	handler := NewTemplateStaticHandler("./testdata", "index.html", data)

	if handler == nil {
		t.Fatal("NewTemplateStaticHandler returned nil")
	}

	if handler.dir != "./testdata" {
		t.Errorf("Expected dir './testdata', got %s", handler.dir)
	}
	if handler.template != "index.html" {
		t.Errorf("Expected template 'index.html', got %s", handler.template)
	}
	if handler.data.RoleDescription != "Test" {
		t.Errorf("Expected data.RoleDescription 'Test', got %s", handler.data.RoleDescription)
	}
}

func TestTemplateStaticHandler_ServeHTTP_Success(t *testing.T) {
	// Create a temporary directory and template file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test template file
	templateContent := `<!DOCTYPE html>
<html>
<head>
    <title>{{.RoleDescription}}</title>
</head>
<body>
    <h1>Server running on {{.HTTPAddr}}</h1>
    <p>Control port: {{.ControlAddr}}</p>
</body>
</html>`

	templatePath := filepath.Join(tempDir, "test.html")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Create handler
	data := TemplateData{
		RoleDescription: "Test Server",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}
	handler := NewTemplateStaticHandler(tempDir, "test.html", data)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Test Server") {
		t.Error("Expected response to contain 'Test Server'")
	}
	if !strings.Contains(body, ":8080") {
		t.Error("Expected response to contain ':8080'")
	}
	if !strings.Contains(body, ":9090") {
		t.Error("Expected response to contain ':9090'")
	}
}

func TestTemplateStaticHandler_ServeHTTP_TemplateNotFound(t *testing.T) {
	// Create handler with non-existent template
	data := TemplateData{
		RoleDescription: "Test",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}
	handler := NewTemplateStaticHandler("./nonexistent", "missing.html", data)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Error loading template\n" {
		t.Errorf("Expected 'Error loading template\\n', got %s", body)
	}
}

func TestTemplateStaticHandler_ServeHTTP_ParsingError(t *testing.T) {
	// Create a temporary directory and invalid template file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a template file with syntax error that will fail parsing
	invalidTemplateContent := `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title</title>
</head>
</html>`

	templatePath := filepath.Join(tempDir, "invalid.html")
	err = os.WriteFile(templatePath, []byte(invalidTemplateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Create handler
	data := TemplateData{
		RoleDescription: "Test",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}
	handler := NewTemplateStaticHandler(tempDir, "invalid.html", data)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response - should fail during template parsing
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Error loading template\n" {
		t.Errorf("Expected 'Error loading template\\n', got %s", body)
	}
}

func TestTemplateStaticHandler_ServeHTTP_EmptyData(t *testing.T) {
	// Create a temporary directory and template file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple template file
	templateContent := `<html><body><h1>Static Content</h1></body></html>`

	templatePath := filepath.Join(tempDir, "static.html")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Create handler with empty data
	data := TemplateData{}
	handler := NewTemplateStaticHandler(tempDir, "static.html", data)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Static Content") {
		t.Error("Expected response to contain 'Static Content'")
	}
}

func TestTemplateStaticHandler_ServeHTTP_ComplexTemplate(t *testing.T) {
	// Create a temporary directory and complex template file
	tempDir, err := os.MkdirTemp("", "static_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a complex template with conditionals and loops
	templateContent := `<!DOCTYPE html>
<html>
<head>
    <title>{{if .RoleDescription}}{{.RoleDescription}}{{else}}Default Title{{end}}</title>
</head>
<body>
    {{if .HTTPAddr}}
    <p>HTTP Server: {{.HTTPAddr}}</p>
    {{end}}
    {{if .ControlAddr}}
    <p>Control Server: {{.ControlAddr}}</p>
    {{end}}
</body>
</html>`

	templatePath := filepath.Join(tempDir, "complex.html")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Create handler with partial data
	data := TemplateData{
		RoleDescription: "Complex Server",
		HTTPAddr:        ":3000",
		// ControlAddr intentionally left empty
	}
	handler := NewTemplateStaticHandler(tempDir, "complex.html", data)

	// Create test request
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Serve the request
	handler.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Complex Server") {
		t.Error("Expected response to contain 'Complex Server'")
	}
	if !strings.Contains(body, ":3000") {
		t.Error("Expected response to contain ':3000'")
	}
	// Should not contain control server info since ControlAddr is empty
	if strings.Contains(body, "Control Server:") {
		t.Error("Expected response to not contain 'Control Server:' for empty ControlAddr")
	}
}

func BenchmarkTemplateStaticHandler_ServeHTTP(b *testing.B) {
	// Create a temporary directory and template file
	tempDir, err := os.MkdirTemp("", "static_bench")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	templateContent := `<html><body><h1>{{.RoleDescription}}</h1><p>{{.HTTPAddr}}</p></body></html>`
	templatePath := filepath.Join(tempDir, "bench.html")
	err = os.WriteFile(templatePath, []byte(templateContent), 0644)
	if err != nil {
		b.Fatalf("Failed to write template file: %v", err)
	}

	data := TemplateData{
		RoleDescription: "Benchmark Server",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}
	handler := NewTemplateStaticHandler(tempDir, "bench.html", data)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkNewTemplateStaticHandler(b *testing.B) {
	data := TemplateData{
		RoleDescription: "Benchmark Server",
		HTTPAddr:        ":8080",
		ControlAddr:     ":9090",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewTemplateStaticHandler("./testdata", "index.html", data)
	}
}
