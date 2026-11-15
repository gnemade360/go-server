package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(r *Router, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func writePathValues(names ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		for i, name := range names {
			if i > 0 {
				w.Write([]byte("|"))
			}
			w.Write([]byte(req.PathValue(name)))
		}
	}
}

func writeText(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(text))
	}
}

func TestPathParam_MatchesOneSegment(t *testing.T) {
	r := NewRouter()
	r.Get("/users/{id}", writePathValues("id"))

	cases := map[string]struct {
		code int
		body string
	}{
		"/users/42":   {http.StatusOK, "42"},
		"/users/42/x": {http.StatusNotFound, ""},
		"/users/":     {http.StatusNotFound, ""},
		"/users":      {http.StatusNotFound, ""},
		"/accounts/1": {http.StatusNotFound, ""},
	}
	for path, want := range cases {
		rec := serve(r, http.MethodGet, path)
		if rec.Code != want.code {
			t.Errorf("%s: expected %d, got %d", path, want.code, rec.Code)
		}
		if want.code == http.StatusOK && rec.Body.String() != want.body {
			t.Errorf("%s: expected body %q, got %q", path, want.body, rec.Body.String())
		}
	}
}

func TestPathParam_RestMatchesRemainingPath(t *testing.T) {
	r := NewRouter()
	r.Get("/api/v1/models/{id...}", writePathValues("id"))

	rec := serve(r, http.MethodGet, "/api/v1/models/qwen/qwen3.8-27b")
	if rec.Code != http.StatusOK || rec.Body.String() != "qwen/qwen3.8-27b" {
		t.Errorf("expected 200 %q, got %d %q", "qwen/qwen3.8-27b", rec.Code, rec.Body.String())
	}

	rec = serve(r, http.MethodGet, "/api/v1/models/")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected an empty rest to be a 404, got %d", rec.Code)
	}
}

func TestPathParam_SeveralParams(t *testing.T) {
	r := NewRouter()
	r.Get("/orgs/{org}/repos/{repo}", writePathValues("org", "repo"))

	rec := serve(r, http.MethodGet, "/orgs/acme/repos/server")
	if rec.Body.String() != "acme|server" {
		t.Errorf("expected %q, got %q", "acme|server", rec.Body.String())
	}
}

func TestPathParam_TrailingSlashIsLiteral(t *testing.T) {
	r := NewRouter()
	r.Get("/users/{id}/", writePathValues("id"))

	if rec := serve(r, http.MethodGet, "/users/7/"); rec.Code != http.StatusOK || rec.Body.String() != "7" {
		t.Errorf("expected 200 %q, got %d %q", "7", rec.Code, rec.Body.String())
	}
	if rec := serve(r, http.MethodGet, "/users/7"); rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 without the trailing slash, got %d", rec.Code)
	}
}

func TestPathParam_RegexNamedGroups(t *testing.T) {
	r := NewRouter()
	r.Get(`^/admin/api/models/(?P<id>.+)/(?P<state>on|off)$`, writePathValues("id", "state"))

	rec := serve(r, http.MethodGet, "/admin/api/models/qwen/qwen3.8-27b/on")
	if rec.Body.String() != "qwen/qwen3.8-27b|on" {
		t.Errorf("expected %q, got %q", "qwen/qwen3.8-27b|on", rec.Body.String())
	}
}

func TestPathParam_FirstRegisteredWins(t *testing.T) {
	r := NewRouter()
	r.Get("/users/me", writeText("literal"))
	r.Get("/users/{id}", writeText("param"))
	r.Get("/items/{id}", writeText("param"))
	r.Get("/items/new", writeText("literal"))

	if body := serve(r, http.MethodGet, "/users/me").Body.String(); body != "literal" {
		t.Errorf("/users/me: expected the earlier literal route, got %q", body)
	}
	if body := serve(r, http.MethodGet, "/users/9").Body.String(); body != "param" {
		t.Errorf("/users/9: expected the param route, got %q", body)
	}
	if body := serve(r, http.MethodGet, "/items/new").Body.String(); body != "param" {
		t.Errorf("/items/new: expected the earlier param route, got %q", body)
	}
}

func TestPathParam_ReRegisteringReplacesHandler(t *testing.T) {
	r := NewRouter()
	r.Get("/users/{id}", writeText("old"))
	r.Get("/users/{id}", writeText("new"))

	if body := serve(r, http.MethodGet, "/users/1").Body.String(); body != "new" {
		t.Errorf("expected the replacement handler, got %q", body)
	}
}

func TestPathParam_InvalidPatternsPanic(t *testing.T) {
	invalid := []string{
		"/a/{",
		"/a/{}",
		"/a/{id",
		"/a/x{id}",
		"/a/{id}x",
		"/a/{id...}/b",
		"/a/{id}/{id}",
		"/a/{1x}",
		"/a/{...}",
	}
	for _, pattern := range invalid {
		t.Run(pattern, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected registering %q to panic", pattern)
				}
			}()
			NewRouter().Get(pattern, writeText(""))
		})
	}
}

func TestPathParam_RegexNamedGroupsNoMatch(t *testing.T) {
	r := NewRouter()
	r.Get(`^/files/(?P<name>[^/]+)\.txt$`, writePathValues("name"))

	if rec := serve(r, http.MethodGet, "/files/report.pdf"); rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestPathParam_PathWithoutLeadingSlashDoesNotMatch(t *testing.T) {
	r := NewRouter()
	r.Handle(http.MethodOptions, "/{any...}", writeText("matched"))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "*", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected OPTIONS * to be a 404, got %d", rec.Code)
	}
}
