package middleware

import (
	"fmt"
	"net/http"
	"time"
)

// ----------------------------------------------------------------------------
// Cache-Control middleware – sets max-age and Expires
// ----------------------------------------------------------------------------

func CacheControl(maxAge time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			maxAgeSecs := int(maxAge.Seconds())
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAgeSecs))
			w.Header().Set("Expires", time.Now().Add(maxAge).UTC().Format(http.TimeFormat))
			next.ServeHTTP(w, r)
		})
	}
}
