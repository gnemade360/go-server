# WebSocket example

End-to-end demo of the framework's `websocket` package: a chat-style
endpoint where each connection joins one shared room and every text
message broadcast is relayed to the other peers. Health and metrics
for the WebSocket subsystem are exposed alongside the standard
framework endpoints.

## Run

```
go run ./examples/websocket-example
```

Then open <http://localhost:8080/> in two browser tabs. Anything
typed in one tab appears in the other (within the same lobby).

## Endpoints

| Path                  | Purpose                                  |
|-----------------------|------------------------------------------|
| `/`                   | Minimal HTML chat client                 |
| `/ws`                 | WebSocket upgrade endpoint               |
| `/health`             | Overall health, includes the `websocket` check |
| `/health/live`        | Liveness probe                           |
| `/health/ready`       | Readiness probe                          |
| `/metrics`            | JSON metrics                             |
| `/metrics/prometheus` | Prometheus exposition format             |

## What this example demonstrates

- `Config` defaults are conservative; the example overrides `Origin`
  for local testing and pairs `Auth: AllowAnonymous` with a token
  bucket rate limiter.
- `OnConnect` joins each conn to the `lobby` room and sends a
  welcome.
- `OnMessage` relays the inbound frame to the rest of the room with
  `Hub().Room("lobby").PublishExcept`.
- `ws.HealthCheck()` is registered with the server's health
  checker, so `/health` reports `websocket` alongside the built-in
  memory and goroutine probes.
- `ws.RegisterMetrics(server.Metrics())` wires connection,
  message, byte, and close-code metrics into the shared registry.
- `ws.Shutdown(ctx)` is invoked on SIGINT/SIGTERM before
  `server.Shutdown`, sending close frames (1001 Going Away) to
  every open connection.

## Production hardening checklist

The example uses `AllowAllOrigins` and `AllowAnonymous` because it
ships with no separate frontend host or auth provider. For
production:

1. Replace `AllowAllOrigins()` with `AllowOrigins("https://your.app")`.
2. Replace `AllowAnonymous{}` with `BearerTokenAuth(verify)` or
   `CookieSessionAuth("__Host-Session", lookup)`.
3. Serve over TLS (`wss://`); set `Content-Security-Policy:
   connect-src wss://your.host` explicitly — `'self'` does not
   reliably match `wss://` in every browser.
4. Tune `Limits` for your expected load.
5. Leave `EnableCompression` off unless you control all clients and
   have considered compression-bomb risk.
