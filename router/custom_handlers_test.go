package router

import (
	"net/http"
	"testing"
)

func writeJSON(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}
}

func TestSetNotFoundHandler_IsCalledForUnknownPath(t *testing.T) {
	r := NewRouter()
	r.Get("/items", writeText("list"))
	r.SetNotFoundHandler(writeJSON(http.StatusNotFound, `{"error":"not found"}`))

	rec := serve(r, http.MethodGet, "/missing")

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected application/json, got %q", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != `{"error":"not found"}` {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
}

func TestSetMethodNotAllowedHandler_IsCalledWithAllowSet(t *testing.T) {
	r := NewRouter()
	r.Get("/items", writeText("list"))
	var allowSeenByHandler string
	r.SetMethodNotAllowedHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		allowSeenByHandler = w.Header().Get("Allow")
		writeJSON(http.StatusMethodNotAllowed, `{"error":"method not allowed"}`)(w, req)
	}))

	rec := serve(r, http.MethodPost, "/items")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
	if allowSeenByHandler != "GET" {
		t.Errorf("expected the handler to see Allow %q, got %q", "GET", allowSeenByHandler)
	}
	if rec.Body.String() != `{"error":"method not allowed"}` {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
}

func TestSetHandlers_NilRestoresDefaults(t *testing.T) {
	r := NewRouter()
	r.Get("/items", writeText("list"))
	r.SetNotFoundHandler(writeText("custom 404"))
	r.SetMethodNotAllowedHandler(writeText("custom 405"))
	r.SetNotFoundHandler(nil)
	r.SetMethodNotAllowedHandler(nil)

	if rec := serve(r, http.MethodGet, "/missing"); rec.Code != http.StatusNotFound || rec.Body.String() != "404 page not found\n" {
		t.Errorf("expected the default 404, got %d %q", rec.Code, rec.Body.String())
	}
	if rec := serve(r, http.MethodPost, "/items"); rec.Code != http.StatusMethodNotAllowed || rec.Body.String() != "method not allowed\n" {
		t.Errorf("expected the default 405, got %d %q", rec.Code, rec.Body.String())
	}
}
