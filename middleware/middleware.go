package middleware

import "net/http"

// Middleware defines a function type for HTTP middleware.
type Middleware func(http.Handler) http.Handler
