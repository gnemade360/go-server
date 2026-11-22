package goserver

import (
	"context"
	"net/http"
	"testing"
)

func TestListenAndServeServesRouterRoutes(t *testing.T) {
	addr := freeAddr(t)
	server := New(addr)
	server.Router().Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.ListenAndServe() }()
	waitUntilListening(t, addr, serveErr)
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	resp, err := http.Get("http://" + addr + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusTeapot {
		t.Errorf("GET /ping status = %d, want %d", resp.StatusCode, http.StatusTeapot)
	}
}
