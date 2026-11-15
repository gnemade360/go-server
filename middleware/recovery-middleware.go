package middleware

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

// RecoveryConfig customizes how Recovery reports a recovered panic.
type RecoveryConfig struct {
	// Log receives the request, the recovered value and the stack.
	// Default: log.Printf of the value and the stack.
	Log func(r *http.Request, recovered any, stack []byte)
	// Respond writes the error response. It is called only when nothing has
	// been written yet. Default: 500 with "internal server error" as text/plain.
	Respond func(w http.ResponseWriter, r *http.Request, recovered any)
}

// Recovery recovers handler panics with the default RecoveryConfig.
func Recovery() Middleware {
	return RecoveryWithConfig(RecoveryConfig{})
}

// RecoveryWithConfig recovers handler panics. Before the response has started,
// it logs the panic and calls Respond. Once the response has started, it logs
// the panic and re-panics with http.ErrAbortHandler, so net/http drops the
// connection and the client sees a cut-off response instead of one that looks
// complete. A panic with http.ErrAbortHandler is passed on without logging.
func RecoveryWithConfig(cfg RecoveryConfig) Middleware {
	logPanic := cfg.Log
	if logPanic == nil {
		logPanic = defaultRecoveryLog
	}
	respond := cfg.Respond
	if respond == nil {
		respond = defaultRecoveryRespond
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseRecorder{ResponseWriter: w}
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logPanic(r, rec, debug.Stack())
				if rw.wroteHeader {
					panic(http.ErrAbortHandler)
				}
				respond(rw, r, rec)
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

func defaultRecoveryLog(_ *http.Request, recovered any, stack []byte) {
	log.Printf("panic recovered: %v\n%s", recovered, stack)
}

func defaultRecoveryRespond(w http.ResponseWriter, _ *http.Request, _ any) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("internal server error"))
}

type responseRecorder struct {
	http.ResponseWriter
	wroteHeader bool
}

func (r *responseRecorder) WriteHeader(code int) {
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Flush() {
	r.wroteHeader = true
	_ = http.NewResponseController(r.ResponseWriter).Flush()
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	r.wroteHeader = true
	return http.NewResponseController(r.ResponseWriter).Hijack()
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
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
