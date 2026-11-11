package middleware

import (
	"compress/gzip"
	"net/http"
	"strings"
)

// ----------------------------------------------------------------------------
// Gzip – transparent response compression
// ----------------------------------------------------------------------------

type gzipResponseWriter struct {
	Writer *gzip.Writer
	http.ResponseWriter
}

func (g gzipResponseWriter) Write(b []byte) (int, error) {
	return g.Writer.Write(b)
}

// Flush pushes buffered compressed bytes to the client before flushing the
// underlying writer, so streamed responses (SSE) are not held back by gzip.
func (g gzipResponseWriter) Flush() {
	_ = g.Writer.Flush()
	_ = http.NewResponseController(g.ResponseWriter).Flush()
}

func (g gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
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
