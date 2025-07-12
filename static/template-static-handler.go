// Package static provides utilities for serving static files
package static

import (
	"html/template"
	"net/http"
	"path/filepath"
)

// TemplateData represents the data to be injected into the template
type TemplateData struct {
	RoleDescription string
	HTTPAddr        string
	ControlAddr     string
}

// TemplateStaticHandler is a handler that serves static files with template variable replacement
type TemplateStaticHandler struct {
	dir      string
	template string
	data     TemplateData
}

// NewTemplateStaticHandler creates a new TemplateStaticHandler
func NewTemplateStaticHandler(dir, templateFile string, data TemplateData) *TemplateStaticHandler {
	return &TemplateStaticHandler{
		dir:      dir,
		template: templateFile,
		data:     data,
	}
}

// ServeHTTP implements the http.Handler interface
func (h *TemplateStaticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Parse the template
	tmpl, err := template.ParseFiles(filepath.Join(h.dir, h.template))
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		return
	}

	// Execute the template with the data
	if err := tmpl.Execute(w, h.data); err != nil {
		http.Error(w, "Error executing template", http.StatusInternalServerError)
		return
	}
}
