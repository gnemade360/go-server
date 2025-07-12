package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

func Recovery() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// wrap the ResponseWriter so we can detect WriteHeader calls:
			rw := &responseRecorder{ResponseWriter: w}
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("‼️ panic recovered: %v\n%s", rec, debug.Stack())
					if !rw.wroteHeader {
						rw.WriteHeader(http.StatusInternalServerError)
					}
					rw.Write([]byte("internal server error"))
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			log.Printf("Request started: %s %s", r.Method, r.URL.Path)

			// Call the next handler in the chain
			next.ServeHTTP(w, r)

			log.Printf("Request completed: %s %s in %v", r.Method, r.URL.Path, time.Since(start))
		})
	}
}
