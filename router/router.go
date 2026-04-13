package router

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gnemade360/go-server/static"
)

type route struct {
	literal string         // non-empty for exact paths
	pattern *regexp.Regexp // non-nil for regex routes
	handler http.Handler
}

type Router struct {
	routes   map[string][]route // method → list of (literal|regex) routes
	notFound http.Handler
}

func NewRouter() *Router {
	return &Router{
		routes:   make(map[string][]route),
		notFound: http.NotFoundHandler(),
	}
}

// Handle registers either an exact path (if pattern doesn’t start with "^")
// or a full regex (if it does).
func (r *Router) Handle(method, pattern string, h http.Handler) {
	// override existing
	for i, rt := range r.routes[method] {
		switch {
		case rt.literal != "" && rt.literal == pattern:
			r.routes[method][i].handler = h
			return
		case rt.pattern != nil && rt.pattern.String() == pattern:
			r.routes[method][i].handler = h
			return
		}
	}

	// new route
	if strings.HasPrefix(pattern, "^") {
		re := regexp.MustCompile(pattern)
		r.routes[method] = append(r.routes[method], route{"", re, h})
	} else {
		r.routes[method] = append(r.routes[method], route{pattern, nil, h})
	}
}

func (r *Router) HandleFunc(method, pattern string, fn http.HandlerFunc) {
	r.Handle(method, pattern, fn)
}

func (r *Router) Get(pattern string, h http.HandlerFunc) {
	r.Handle(http.MethodGet, pattern, h)
}

func (r *Router) Post(pattern string, h http.HandlerFunc) {
	r.Handle(http.MethodPost, pattern, h)
}

func (r *Router) Put(pattern string, h http.HandlerFunc) {
	r.Handle(http.MethodPut, pattern, h)
}

func (r *Router) Delete(pattern string, h http.HandlerFunc) {
	r.Handle(http.MethodDelete, pattern, h)
}

// Static mounts an SPA-aware file handler under a regex that matches
// the given prefix and anything below it.
func (r *Router) Static(opts StaticOptions) {
	if opts.Prefix == "" {
		opts.Prefix = "/"
	}
	handler := static.NewStaticHandler(opts)
	pat := "^" + regexp.QuoteMeta(opts.Prefix) + "(.*)?$"
	r.Handle(http.MethodGet, pat, handler)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, rt := range r.routes[req.Method] {
		if rt.literal != "" {
			if req.URL.Path == rt.literal {
				rt.handler.ServeHTTP(w, req)
				return
			}
		} else {
			if rt.pattern.MatchString(req.URL.Path) {
				rt.handler.ServeHTTP(w, req)
				return
			}
		}
	}
	r.notFound.ServeHTTP(w, req)
}

// StaticOptions re-export to avoid deep import for callers.
type StaticOptions = static.Options
