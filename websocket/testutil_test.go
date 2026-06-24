package websocket

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
)

// testServer spins up an httptest.Server fronting h and returns the
// ws:// URL plus a cleanup closure.
func testServer(t *testing.T, h *Handler) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	wsURL := toWS(t, srv.URL)
	cleanup := func() { srv.Close() }
	return wsURL, cleanup
}

func toWS(t *testing.T, httpURL string) string {
	t.Helper()
	u, err := url.Parse(httpURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	return u.String()
}

func dialWS(t *testing.T, urlStr string, hdr http.Header) *gws.Conn {
	t.Helper()
	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second
	conn, _, err := dialer.Dial(urlStr, hdr)
	if err != nil {
		t.Fatalf("dial %s: %v", urlStr, err)
	}
	return conn
}

func httpOrigin(t *testing.T, wsURL string) string {
	t.Helper()
	u, err := url.Parse(wsURL)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}
	u.Path = ""
	return u.String()
}
