package middleware

import (
	"bufio"
	"compress/gzip"
	"errors"
	"net"
	"net/http"
	"strings"
)

// ----------------------------------------------------------------------------
// Gzip – transparent response compression
// ----------------------------------------------------------------------------

var errGzipHijack = errors.New("middleware: cannot hijack a gzip-compressed response")

// gzipResponseWriter creates its gzip.Writer, and sets Content-Encoding, on
// the first Write, final WriteHeader or Flush. A handler that panics before
// writing leaves the response untouched, so an outer Recovery can still send
// a plain 500.
type gzipResponseWriter struct {
	Writer *gzip.Writer
	http.ResponseWriter
}

func (g *gzipResponseWriter) start() {
	if g.Writer != nil {
		return
	}
	g.Header().Set("Content-Encoding", "gzip")
	g.Writer = gzip.NewWriter(g.ResponseWriter)
}

func (g *gzipResponseWriter) close() {
	if g.Writer == nil {
		return
	}
	_ = g.Writer.Close()
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	isInformational := code < http.StatusOK
	if !isInformational {
		g.start()
	}
	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	g.start()
	return g.Writer.Write(b)
}

// Flush pushes buffered compressed bytes to the client before flushing the
// underlying writer, so streamed responses (SSE) are not held back by gzip.
func (g *gzipResponseWriter) Flush() {
	g.start()
	_ = g.Writer.Flush()
	_ = http.NewResponseController(g.ResponseWriter).Flush()
}

// Hijack refuses because the deferred gzip Close would write onto a connection
// the handler now owns.
func (g *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, errGzipHijack
}

func (g *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return g.ResponseWriter
}

func Gzip() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
			isUpgrade := r.Header.Get("Upgrade") != ""
			if !acceptsGzip || isUpgrade {
				next.ServeHTTP(w, r)
				return
			}
			gw := &gzipResponseWriter{ResponseWriter: w}
			defer gw.close()
			next.ServeHTTP(gw, r)
		})
	}
}
