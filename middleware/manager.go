package middleware

import "net/http"

// MiddlewareManager embeds middleware logic.
type MiddlewareManager struct {
	middleware []Middleware
}

// AddMiddleware adds one or more middleware to the manager.
func (m *MiddlewareManager) AddMiddleware(mw ...Middleware) {
	m.middleware = append(m.middleware, mw...)
}

// ApplyMiddleware applies all middleware in reverse order to the given handler.
func (m *MiddlewareManager) ApplyMiddleware(h http.Handler) http.Handler {
	for i := len(m.middleware) - 1; i >= 0; i-- {
		h = m.middleware[i](h)
	}
	return h
}
