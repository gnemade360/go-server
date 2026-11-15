package router

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/gnemade360/go-server/static"
)

type route struct {
	literal    string         // non-empty for exact paths
	pattern    *regexp.Regexp // non-nil for regex routes
	groupNames []string       // regex subexpression names, set only when one is named
	template   *pathTemplate  // non-nil for paths with {name} segments
	handler    http.Handler
}

type Router struct {
	routes           map[string][]route // method → list of (literal|regex) routes
	notFound         http.Handler
	methodNotAllowed http.Handler
}

func NewRouter() *Router {
	return &Router{
		routes:           make(map[string][]route),
		notFound:         http.NotFoundHandler(),
		methodNotAllowed: http.HandlerFunc(defaultMethodNotAllowed),
	}
}

func defaultMethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// SetNotFoundHandler sets the handler for requests whose path matches no
// route. nil restores the default, http.NotFoundHandler.
func (r *Router) SetNotFoundHandler(h http.Handler) {
	if h == nil {
		h = http.NotFoundHandler()
	}
	r.notFound = h
}

// SetMethodNotAllowedHandler sets the handler for requests whose path matches
// a route only under other methods. The Allow header is already set when it
// runs. nil restores the default, a plain-text 405.
func (r *Router) SetMethodNotAllowedHandler(h http.Handler) {
	if h == nil {
		h = http.HandlerFunc(defaultMethodNotAllowed)
	}
	r.methodNotAllowed = h
}

// Handle registers a full regex (if pattern starts with "^"), a path with
// {name} or a final {name...} segment, or an exact path. Values of {name}
// segments and of named regex groups are available through req.PathValue.
// A {name} or {name...} segment never matches an empty value. Matching uses
// the decoded req.URL.Path, so an encoded slash (%2F) splits a segment.
// The first registered route that matches wins. Handle panics on a malformed
// {name} pattern, including one that does not start with "/".
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
		case rt.template != nil && rt.template.source == pattern:
			r.routes[method][i].handler = h
			return
		}
	}

	r.routes[method] = append(r.routes[method], newRoute(pattern, h))
}

func newRoute(pattern string, h http.Handler) route {
	isRegex := strings.HasPrefix(pattern, "^")
	if isRegex {
		re := regexp.MustCompile(pattern)
		return route{pattern: re, groupNames: namedGroups(re), handler: h}
	}
	hasParams := strings.Contains(pattern, "{")
	if hasParams {
		return route{template: mustParseTemplate(pattern), handler: h}
	}
	return route{literal: pattern, handler: h}
}

func namedGroups(re *regexp.Regexp) []string {
	names := re.SubexpNames()
	for _, name := range names {
		if name != "" {
			return names
		}
	}
	return nil
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

// ServeHTTP runs the first route that matches the request's method and path.
// If the path matches only under other methods, it sets Allow and runs the
// 405 handler; otherwise it runs the 404 handler. HEAD is not served by GET
// routes, so HEAD on a GET-only path gets a 405.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, rt := range r.routes[req.Method] {
		if rt.match(req) {
			rt.handler.ServeHTTP(w, req)
			return
		}
	}
	allowed := r.allowedMethods(req)
	if len(allowed) == 0 {
		r.notFound.ServeHTTP(w, req)
		return
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	r.methodNotAllowed.ServeHTTP(w, req)
}

// allowedMethods lists, sorted, the other methods with a route matching the
// request path. It runs only after no route matched the request's method.
func (r *Router) allowedMethods(req *http.Request) []string {
	var allowed []string
	for method, routes := range r.routes {
		if method == req.Method {
			continue
		}
		for _, rt := range routes {
			if rt.matchesPath(req.URL.Path) {
				allowed = append(allowed, method)
				break
			}
		}
	}
	slices.Sort(allowed)
	return allowed
}

// matchesPath reports whether the route matches path, without setting path
// values on the request.
func (rt route) matchesPath(path string) bool {
	switch {
	case rt.template != nil:
		return rt.template.walk(path, nil)
	case rt.pattern == nil:
		return path == rt.literal
	}
	return rt.pattern.MatchString(path)
}

// match reports whether the route matches req, and on a match sets the
// route's path values on req.
func (rt route) match(req *http.Request) bool {
	path := req.URL.Path
	switch {
	case rt.template != nil:
		return rt.template.match(path, req)
	case rt.pattern == nil:
		return path == rt.literal
	case rt.groupNames == nil:
		return rt.pattern.MatchString(path)
	}
	groups := rt.pattern.FindStringSubmatch(path)
	if groups == nil {
		return false
	}
	for i, name := range rt.groupNames {
		if name != "" {
			req.SetPathValue(name, groups[i])
		}
	}
	return true
}

type templateSegment struct {
	literal string // the exact text, when param is empty
	param   string // the {name}, or the final {name...}, without braces
	rest    bool   // true for a final {name...}
}

type pathTemplate struct {
	source   string
	segments []templateSegment
}

var paramNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func mustParseTemplate(pattern string) *pathTemplate {
	trimmed, hasRoot := strings.CutPrefix(pattern, "/")
	if !hasRoot {
		panic(fmt.Sprintf("router: pattern %q must start with /", pattern))
	}
	parts := strings.Split(trimmed, "/")
	segments := make([]templateSegment, len(parts))
	seenNames := make(map[string]bool, len(parts))
	lastIndex := len(parts) - 1
	for i, part := range parts {
		isParam := strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}")
		if !isParam {
			if strings.ContainsAny(part, "{}") {
				panic(fmt.Sprintf("router: bad segment %q in pattern %q", part, pattern))
			}
			segments[i] = templateSegment{literal: part}
			continue
		}
		name := part[1 : len(part)-1]
		name, isRest := strings.CutSuffix(name, "...")
		if isRest && i != lastIndex {
			panic(fmt.Sprintf("router: {%s...} must be the last segment in pattern %q", name, pattern))
		}
		if !paramNamePattern.MatchString(name) {
			panic(fmt.Sprintf("router: bad parameter name %q in pattern %q", name, pattern))
		}
		if seenNames[name] {
			panic(fmt.Sprintf("router: duplicate parameter %q in pattern %q", name, pattern))
		}
		seenNames[name] = true
		segments[i] = templateSegment{param: name, rest: isRest}
	}
	return &pathTemplate{source: pattern, segments: segments}
}

// match reports whether path fits the template, then walks it a second time
// to set the path values on req, so a failed match allocates nothing.
func (t *pathTemplate) match(path string, req *http.Request) bool {
	if !t.walk(path, nil) {
		return false
	}
	t.walk(path, req)
	return true
}

// walk compares path with the template segment by segment without splitting
// it. When req is not nil, it sets each parameter's value on req.
func (t *pathTemplate) walk(path string, req *http.Request) bool {
	remaining, hasRoot := strings.CutPrefix(path, "/")
	if !hasRoot {
		return false
	}
	lastIndex := len(t.segments) - 1
	for i, segment := range t.segments {
		if segment.rest {
			if remaining == "" {
				return false
			}
			if req != nil {
				req.SetPathValue(segment.param, remaining)
			}
			return true
		}
		part, after, hasMore := strings.Cut(remaining, "/")
		isLast := i == lastIndex
		if hasMore == isLast {
			return false
		}
		remaining = after
		if segment.param == "" {
			if part != segment.literal {
				return false
			}
			continue
		}
		if part == "" {
			return false
		}
		if req != nil {
			req.SetPathValue(segment.param, part)
		}
	}
	return true
}

// StaticOptions re-export to avoid deep import for callers.
type StaticOptions = static.Options
