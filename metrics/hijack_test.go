package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHTTPMetrics_UnwrapReturnsInnerWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := NewHTTPMetrics(NewRegistry()).Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			t.Fatal("wrapped writer does not implement Unwrap")
		}
		if wrapped.Unwrap() != rec {
			t.Error("Unwrap did not return the inner writer")
		}
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
}

func TestHTTPMetrics_AllowsWebSocketUpgrade(t *testing.T) {
	upgrader := websocket.Upgrader{}
	httpMetrics := NewHTTPMetrics(NewRegistry())
	srv := httptest.NewServer(httpMetrics.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade through metrics middleware failed: %v", err)
			return
		}
		defer conn.Close()
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(messageType, payload)
	})))
	defer srv.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
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
