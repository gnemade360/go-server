package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecovery_PanicAfterResponseStartCutsTheConnection(t *testing.T) {
	cases := map[string]bool{"buffered": false, "flushed": true}
	for name, flushFirst := range cases {
		t.Run(name, func(t *testing.T) {
			captureLog(t)
			srv := httptest.NewServer(Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("data: x\n\n"))
				if flushFirst {
					http.NewResponseController(w).Flush()
				}
				panic("mid-stream")
			})))
			defer srv.Close()

			resp, err := http.Get(srv.URL)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)

			if err == nil {
				t.Errorf("expected the client to see a cut-off response, got a complete %d with body %q", resp.StatusCode, body)
			}
		})
	}
}
