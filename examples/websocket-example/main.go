package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	goserver "github.com/gnemade360/go-server"
)

func main() {
	server := goserver.NewServer()

	var ws *goserver.WebSocketHandler
	ws = goserver.NewWebSocketHandler(goserver.WebSocketConfig{
		Origin:    goserver.WebSocketAllowAllOrigins(),
		Auth:      goserver.WebSocketAllowAnonymous,
		Logger:    log.New(os.Stdout, "ws ", log.LstdFlags|log.Lmsgprefix),
		RateLimit: goserver.NewWebSocketTokenBucketLimiter(20, 64*1024),
		OnConnect: func(c goserver.WebSocketConnRef) error {
			ws.Hub().Room("lobby").Join(c)
			return c.SendMessage(goserver.WebSocketMessage{
				Type: goserver.WebSocketTextMessage,
				Data: []byte(fmt.Sprintf("welcome %s", c.ID()[:8])),
			})
		},
		OnMessage: func(c goserver.WebSocketConnRef, m goserver.WebSocketMessage) error {
			ws.Hub().Room("lobby").PublishExcept(c, m)
			return nil
		},
	})

	server.Router().Get("/ws", ws.ServeHTTP)
	server.Router().Get("/", indexHandler)

	server.Health().AddCheck("websocket", ws.HealthCheck())
	ws.RegisterMetrics(server.Metrics())

	server.AddHealthRoutes()
	server.AddMetricsRoutes()

	addr := ":8080"
	server.Configure(addr)
	log.Printf("listening on %s (open http://localhost%s/)", addr, addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- server.Start(context.Background()) }()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Fatalf("server error: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ws.Shutdown(shutdownCtx); err != nil {
		log.Printf("websocket shutdown: %v", err)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

const indexHTML = `<!doctype html>
<html>
<head><title>WebSocket Example</title></head>
<body>
<h1>WebSocket chat</h1>
<div id="log" style="font-family:monospace;white-space:pre-wrap;border:1px solid #ccc;padding:8px;height:300px;overflow:auto;"></div>
<input id="msg" style="width:80%" placeholder="type and press enter">
<script>
const log = document.getElementById('log');
const append = (line) => { log.textContent += line + "\n"; log.scrollTop = log.scrollHeight; };
const ws = new WebSocket("ws://" + location.host + "/ws");
ws.onopen = () => append("[open]");
ws.onmessage = (e) => append("< " + e.data);
ws.onclose = (e) => append("[close " + e.code + "]");
ws.onerror = () => append("[error]");
document.getElementById('msg').addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && ws.readyState === 1) {
    const v = e.target.value; ws.send(v); append("> " + v); e.target.value = '';
  }
});
</script>
</body>
</html>`
