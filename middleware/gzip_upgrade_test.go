package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestGzip_PassesWebSocketUpgradeThroughWhenGzipIsAccepted(t *testing.T) {
	srv := httptest.NewServer(Gzip()(echoWebSocketHandler(t)))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, http.Header{"Accept-Encoding": {"gzip"}})
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
