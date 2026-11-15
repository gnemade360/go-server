package middleware

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestRecovery_PassesFlushThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := Recovery()(flushingHandler(t, rec))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
}

func TestRecovery_FlusherTypeAssertion(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Fatal("wrapped writer does not implement http.Flusher")
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stream", nil))
}

func TestRecovery_UnwrapReturnsInnerWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		unwrapper, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			t.Fatal("wrapped writer does not implement Unwrap")
		}
		if unwrapper.Unwrap() != rec {
			t.Error("Unwrap did not return the inner writer")
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestRecovery_StreamsChunksBeforeHandlerReturns(t *testing.T) {
	secondChunkAllowed := make(chan struct{})
	handler := Recovery()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: first\n\n"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through Recovery failed: %v", err)
			return
		}
		<-secondChunkAllowed
		w.Write([]byte("data: second\n\n"))
	}))
	srv := httptest.NewServer(handler)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	firstLine, err := readLineWithin(reader, 2*time.Second)
	close(secondChunkAllowed)
	if err != nil {
		t.Fatalf("first chunk did not arrive before the handler finished: %v", err)
	}
	if firstLine != "data: first\n" {
		t.Errorf("unexpected first line %q", firstLine)
	}
}

func readLineWithin(reader *bufio.Reader, timeout time.Duration) (string, error) {
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		done <- result{line, err}
	}()
	select {
	case res := <-done:
		return res.line, res.err
	case <-time.After(timeout):
		return "", io.ErrNoProgress
	}
}

func TestWrappers_AllowWebSocketUpgrade(t *testing.T) {
	quietLogging := DefaultLoggingConfig()
	quietLogging.LogHandler = func(LogEntry) {}
	cases := map[string]Middleware{
		"Recovery":             Recovery(),
		"StructuredLogging":    StructuredLogging(quietLogging),
		"RequestLoggingSimple": RequestLoggingSimple(),
	}
	for name, mw := range cases {
		t.Run(name, func(t *testing.T) {
			assertWebSocketEcho(t, mw(echoWebSocketHandler(t)))
		})
	}
}

func TestGzip_HijackReturnsError(t *testing.T) {
	srv := httptest.NewServer(Gzip()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := http.NewResponseController(w).Hijack()
		if err == nil {
			conn.Close()
			t.Error("expected Hijack through gzip to fail")
			return
		}
		if !strings.Contains(err.Error(), "gzip") {
			t.Errorf("expected an error naming gzip, got %v", err)
		}
	})))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
}

func echoWebSocketHandler(t *testing.T) http.Handler {
	upgrader := websocket.Upgrader{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(messageType, payload)
	})
}

func assertWebSocketEcho(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(payload) != "ping" {
		t.Errorf("expected echo %q, got %q", "ping", payload)
	}
}
