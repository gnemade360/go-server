package middleware

import (
	"net/http"
)

// ----------------------------------------------------------------------------
// Simple CORS middleware
// ----------------------------------------------------------------------------

func CORS(allowedOrigins ...string) Middleware {
	allowAll := len(allowedOrigins) == 0 || (len(allowedOrigins) == 1 && allowedOrigins[0] == "*")

	isAllowed := func(origin string) bool {
		if allowAll {
			return true
		}
		for _, o := range allowedOrigins {
			if o == origin {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if isAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
