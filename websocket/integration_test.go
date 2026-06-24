package websocket

import (
	"net/http"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
)
func TestEchoRoundTrip(t *testing.T) {
	h := NewHandler(Config{
		Auth: AllowAnonymous{},
		OnMessage: func(c ConnRef, m Message) error {
			return c.SendMessage(m)
		},
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	conn := dialWS(t, wsURL, http.Header{"Origin": []string{httpOrigin(t, wsURL)}})
	defer conn.Close()

	want := []byte(`{"hello":"world"}`)
	if err := conn.WriteMessage(gws.TextMessage, want); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("echo mismatch: got %q want %q", got, want)
	}
}

func TestOriginRejected(t *testing.T) {
	h := NewHandler(Config{
		Auth: AllowAnonymous{},
		// Default Origin is SameOriginChecker.
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second
	_, resp, err := dialer.Dial(wsURL, http.Header{"Origin": []string{"https://evil.example.com"}})
	if err == nil {
		t.Fatal("expected dial to fail for foreign origin")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %v", resp)
	}
}

func TestUnauthorizedByDefault(t *testing.T) {
	// DenyAll is the default — no Auth set explicitly.
	h := NewHandler(Config{})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second
	_, resp, err := dialer.Dial(wsURL, http.Header{"Origin": []string{httpOrigin(t, wsURL)}})
	if err == nil {
		t.Fatal("expected dial to fail with default DenyAll")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %v", resp)
	}
}

func TestBearerTokenAuth(t *testing.T) {
	verifyCalled := ""
	auth := BearerTokenAuth(func(token string) (Principal, error) {
		verifyCalled = token
		if token != "good" {
			return Principal{}, ErrUnauthorized
		}
		return Principal{ID: "user-1"}, nil
	})

	h := NewHandler(Config{
		Auth:   auth,
		Origin: AllowAllOrigins(),
		OnMessage: func(c ConnRef, m Message) error {
			return c.SendMessage(Message{Type: TextMessage, Data: []byte(c.Principal().ID)})
		},
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	// Reject bad token.
	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second
	_, resp, _ := dialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer bad"},
	})
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401 for bad token, got %v", resp)
	}
	if verifyCalled != "bad" {
		t.Fatalf("verifier not invoked with bad token: %q", verifyCalled)
	}

	// Accept good token.
	conn := dialWS(t, wsURL, http.Header{"Authorization": []string{"Bearer good"}})
	defer conn.Close()

	if err := conn.WriteMessage(gws.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "user-1" {
		t.Fatalf("want principal echo 'user-1', got %q", got)
	}
}

func TestBearerViaSubprotocol(t *testing.T) {
	auth := BearerTokenAuth(func(token string) (Principal, error) {
		if token != "secret" {
			return Principal{}, ErrUnauthorized
		}
		return Principal{ID: "u"}, nil
	})

	h := NewHandler(Config{
		Auth:         auth,
		Origin:       AllowAllOrigins(),
		Subprotocols: []string{"chat.v1"},
		OnMessage: func(c ConnRef, m Message) error {
			return c.SendMessage(Message{Type: TextMessage, Data: []byte(c.Subprotocol())})
		},
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 2 * time.Second
	dialer.Subprotocols = []string{"chat.v1", "bearer.secret"}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if conn.Subprotocol() != "chat.v1" {
		t.Fatalf("server echoed wrong subprotocol: %q", conn.Subprotocol())
	}
	if err := conn.WriteMessage(gws.TextMessage, []byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, got, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "chat.v1" {
		t.Fatalf("want chat.v1, got %q", got)
	}
}

func TestReadLimitClosesWithMessageTooBig(t *testing.T) {
	h := NewHandler(Config{
		Auth:                   AllowAnonymous{},
		Origin:                 AllowAllOrigins(),
		ReadLimit:              64,
		MessageReassemblyLimit: 64,
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	conn := dialWS(t, wsURL, nil)
	defer conn.Close()

	// 200 bytes of valid UTF-8 — exceeds the 64-byte cap.
	payload := strings.Repeat("a", 200)
	if err := conn.WriteMessage(gws.TextMessage, []byte(payload)); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("expected close on oversize message")
	}
	if !gws.IsCloseError(err, CloseMessageTooBig) {
		t.Fatalf("want close 1009, got %v", err)
	}
}

func TestInvalidUTF8RejectedOnTextFrame(t *testing.T) {
	h := NewHandler(Config{
		Auth:   AllowAnonymous{},
		Origin: AllowAllOrigins(),
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	conn := dialWS(t, wsURL, nil)
	defer conn.Close()

	// Lone continuation byte — not valid UTF-8.
	if err := conn.WriteMessage(gws.TextMessage, []byte{0x80}); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if !gws.IsCloseError(err, CloseInvalidFramePayload) {
		t.Fatalf("want close 1007, got %v", err)
	}
}

func TestConnectionLimitExceeded(t *testing.T) {
	h := NewHandler(Config{
		Auth:   AllowAnonymous{},
		Origin: AllowAllOrigins(),
		Limits: NewDefaultLimiter(1, 0, 0),
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	c1 := dialWS(t, wsURL, nil)
	defer c1.Close()

	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 1 * time.Second
	_, resp, err := dialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected second dial to fail")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %v", resp)
	}
}

func TestBroadcastAcrossConnections(t *testing.T) {
	h := NewHandler(Config{
		Auth:   AllowAnonymous{},
		Origin: AllowAllOrigins(),
	})
	hub := h.Hub()
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	a := dialWS(t, wsURL, nil)
	defer a.Close()
	b := dialWS(t, wsURL, nil)
	defer b.Close()

	// Wait for both registrations to land in the hub.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if hub.Active() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if hub.Active() < 2 {
		t.Fatalf("hub Active = %d, want >= 2", hub.Active())
	}

	hub.Broadcast(Message{Type: TextMessage, Data: []byte("ping")})

	for i, c := range []*gws.Conn{a, b} {
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, got, err := c.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read: %v", i, err)
		}
		if string(got) != "ping" {
			t.Fatalf("client %d got %q want %q", i, got, "ping")
		}
	}
}

func TestRoomPublishExcept(t *testing.T) {
	var h *Handler
	h = NewHandler(Config{
		Auth:   AllowAnonymous{},
		Origin: AllowAllOrigins(),
		OnConnect: func(c ConnRef) error {
			h.Hub().Room("lobby").Join(c)
			return nil
		},
	})
	hub := h.Hub()
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	a := dialWS(t, wsURL, nil)
	defer a.Close()
	b := dialWS(t, wsURL, nil)
	defer b.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if hub.Room("lobby").Size() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	room := hub.Room("lobby")
	members := room.Members()
	if len(members) < 2 {
		t.Fatalf("room size = %d", len(members))
	}

	sender := members[0]
	room.PublishExcept(sender, Message{Type: TextMessage, Data: []byte("hi")})

	// Exactly one of the two test clients corresponds to the
	// non-sender server-side conn; that one receives "hi". The
	// other should time out. We run both reads in parallel so a
	// blocking sender-side read doesn't starve the receiver.
	type result struct {
		data []byte
		err  error
	}
	readWith := func(c *gws.Conn) <-chan result {
		ch := make(chan result, 1)
		go func() {
			c.SetReadDeadline(time.Now().Add(time.Second))
			_, d, err := c.ReadMessage()
			ch <- result{data: d, err: err}
		}()
		return ch
	}
	chA, chB := readWith(a), readWith(b)
	resA, resB := <-chA, <-chB

	got := 0
	if resA.err == nil && string(resA.data) == "hi" {
		got++
	}
	if resB.err == nil && string(resB.data) == "hi" {
		got++
	}
	if got != 1 {
		t.Fatalf("want exactly one receiver of 'hi'; resA={%q,%v} resB={%q,%v}",
			resA.data, resA.err, resB.data, resB.err)
	}
}

func TestJSONDepthLimit(t *testing.T) {
	codec := JSONCodec{MaxDepth: 4}
	deep := []byte(`[[[[[1]]]]]`) // 5 levels
	if err := codec.Decode(Message{Data: deep}, new(any)); err == nil {
		t.Fatal("expected depth-limit error")
	}
	shallow := []byte(`[[[1]]]`) // 3 levels
	if err := codec.Decode(Message{Data: shallow}, new(any)); err != nil {
		t.Fatalf("unexpected error on shallow payload: %v", err)
	}
}

func TestShutdownEvictsConnections(t *testing.T) {
	h := NewHandler(Config{
		Auth:   AllowAnonymous{},
		Origin: AllowAllOrigins(),
	})
	wsURL, cleanup := testServer(t, h)
	defer cleanup()

	conn := dialWS(t, wsURL, nil)
	defer conn.Close()

	// Let the upgrade settle.
	time.Sleep(50 * time.Millisecond)

	done := make(chan error, 1)
	go func() {
		done <- h.Shutdown(timeoutCtx(2 * time.Second))
	}()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	if !gws.IsCloseError(err, CloseGoingAway) {
		t.Fatalf("want close 1001, got %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("shutdown err: %v", err)
	}

	// New upgrades must be rejected after shutdown.
	dialer := *gws.DefaultDialer
	dialer.HandshakeTimeout = 1 * time.Second
	_, resp, _ := dialer.Dial(wsURL, nil)
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503 after shutdown, got %v", resp)
	}
}

func TestTokenBucketRateLimiter(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(2, 0)
	// Burst of 2 allowed.
	if !limiter.Allow(1) {
		t.Fatal("first message should be allowed")
	}
	if !limiter.Allow(1) {
		t.Fatal("second message should be allowed")
	}
	if limiter.Allow(1) {
		t.Fatal("third message should be denied")
	}
	time.Sleep(600 * time.Millisecond)
	if !limiter.Allow(1) {
		t.Fatal("after refill, message should be allowed")
	}
}
