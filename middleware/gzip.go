package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// ----------------------------------------------------------------------------
// Gzip – transparent response compression
// ----------------------------------------------------------------------------

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

func Gzip() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w.Header().Set("Content-Encoding", "gzip")
			next.ServeHTTP(gzipResponseWriter{Writer: gz, ResponseWriter: w}, r)
		})
	}
}
