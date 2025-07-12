package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout returns a middleware that adds a timeout to the request context.
// If the handler doesn't complete within the specified duration, the request is cancelled.
func Timeout(duration time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If no timeout is set, just call the next handler
			if duration <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Create a new context with timeout
			ctx, cancel := context.WithTimeout(r.Context(), duration)
			defer cancel()

			// Create a new request with the timeout context
			r = r.WithContext(ctx)

			// Call the next handler with the new request
			next.ServeHTTP(w, r)
		})
	}
}
