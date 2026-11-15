package router

import (
	"net/http"
	"testing"
)

func TestMethodNotAllowed_SingleMethod(t *testing.T) {
	r := NewRouter()
	r.Get("/items", writeText("list"))

	rec := serve(r, http.MethodPost, "/items")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET" {
		t.Errorf("expected Allow %q, got %q", "GET", allow)
	}
	if rec.Body.String() != "method not allowed\n" {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
}

func TestMethodNotAllowed_AllowIsSorted(t *testing.T) {
	r := NewRouter()
	r.Put("/items", writeText("put"))
	r.Get("/items", writeText("get"))
	r.Delete("/items", writeText("delete"))

	rec := serve(r, http.MethodPost, "/items")

	if allow := rec.Header().Get("Allow"); allow != "DELETE, GET, PUT" {
		t.Errorf("expected Allow %q, got %q", "DELETE, GET, PUT", allow)
	}
}

func TestMethodNotAllowed_OnlyMethodsMatchingThePath(t *testing.T) {
	r := NewRouter()
	r.Get("/a", writeText("a"))
	r.Post("/b", writeText("b"))

	rec := serve(r, http.MethodPut, "/a")

	if allow := rec.Header().Get("Allow"); allow != "GET" {
		t.Errorf("expected Allow %q, got %q", "GET", allow)
	}
}

func TestMethodNotAllowed_TemplateAndRegexRoutes(t *testing.T) {
	r := NewRouter()
	r.Get("/users/{id}", writePathValues("id"))
	r.Post(`^/files/(?P<name>[^/]+)$`, writePathValues("name"))

	if rec := serve(r, http.MethodDelete, "/users/5"); rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET" {
		t.Errorf("template: expected 405 with Allow GET, got %d %q", rec.Code, rec.Header().Get("Allow"))
	}
	if rec := serve(r, http.MethodGet, "/files/report"); rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Errorf("regex: expected 405 with Allow POST, got %d %q", rec.Code, rec.Header().Get("Allow"))
	}
}

func TestMethodNotAllowed_UnknownPathIs404(t *testing.T) {
	r := NewRouter()
	r.Get("/items", writeText("list"))

	rec := serve(r, http.MethodPost, "/other")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "" {
		t.Errorf("expected no Allow header, got %q", allow)
	}
}
