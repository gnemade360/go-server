package goserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	return addr
}

func waitUntilListening(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s", addr)
}

// startWithBlockingRoute starts server on a free port with a /block route that
// sends its headers and then waits until release is closed.
func startWithBlockingRoute(t *testing.T, server *Server, ctx context.Context) (startErr chan error, release chan struct{}) {
	t.Helper()
	addr := freeAddr(t)
	server.Configure(addr)
	requestStarted := make(chan struct{})
	release = make(chan struct{})
	server.Router().Get("/block", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		http.NewResponseController(w).Flush()
		close(requestStarted)
		<-release
	})
	startErr = make(chan error, 1)
	go func() { startErr <- server.Start(ctx) }()
	waitUntilListening(t, addr)
	go func() {
		resp, err := http.Get("http://" + addr + "/block")
		if err == nil {
			resp.Body.Close()
		}
	}()
	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("blocking request did not reach the handler")
	}
	t.Cleanup(func() { server.httpServer.Close() })
	return startErr, release
}

func TestNewServer_DefaultShutdownTimeoutIs10Seconds(t *testing.T) {
	if got := NewServer().shutdownTimeout; got != 10*time.Second {
		t.Errorf("expected 10s, got %v", got)
	}
	if got := NewServerWithConfig(ServerConfig{}).shutdownTimeout; got != 10*time.Second {
		t.Errorf("NewServerWithConfig: expected 10s, got %v", got)
	}
}

func TestStart_ReturnsWithinShutdownTimeoutWhileARequestIsOpen(t *testing.T) {
	server := NewServer()
	WithShutdownTimeout(100 * time.Millisecond)(server)
	ctx, cancel := context.WithCancel(context.Background())
	startErr, release := startWithBlockingRoute(t, server, ctx)
	defer close(release)

	cancel()

	select {
	case err := <-startErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return within the shutdown timeout")
	}
}

func TestStart_ZeroShutdownTimeoutWaitsForOpenRequests(t *testing.T) {
	server := NewServer()
	WithShutdownTimeout(0)(server)
	ctx, cancel := context.WithCancel(context.Background())
	startErr, release := startWithBlockingRoute(t, server, ctx)

	cancel()

	select {
	case err := <-startErr:
		t.Fatalf("expected Start to wait for the open request, it returned %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-startErr:
		if err != nil {
			t.Errorf("expected nil after the request finished, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after the open request finished")
	}
}

func TestStart_ReturnsNilWhenShutdownIsCalledElsewhere(t *testing.T) {
	server := NewServer()
	addr := freeAddr(t)
	server.Configure(addr)
	startErr := make(chan error, 1)
	go func() { startErr <- server.Start(context.Background()) }()
	waitUntilListening(t, addr)

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	select {
	case err := <-startErr:
		if err != nil {
			t.Errorf("expected nil instead of http.ErrServerClosed, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after Shutdown")
	}
}
