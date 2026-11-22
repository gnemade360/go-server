# Go Server

A lightweight, extensible HTTP server library for Go with built-in middleware support, routing, and static file serving.

## Features

- Simple and intuitive API
- Built-in middleware system (CORS, Gzip, Cache Control, Recovery, Timeout)
- Flexible routing with regex support
- Static file serving with SPA support
- **Secure WebSocket support (RFC 6455)** with origin checking, pluggable auth, connection limits, rate limiting, ping/pong, hub/rooms, and graceful shutdown
- **Health monitoring and checks** with built-in endpoints
- Configurable timeouts and server options
- Production-ready with graceful shutdown

## Installation

```bash
go get github.com/gnemade360/go-server
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    
    "github.com/gnemade360/go-server"
)

func main() {
    // Create a new server
    server := goserver.NewServer()
    
    // Add routes
    server.Router().Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })
    
    server.Router().Get("/api/users", handleUsers)
    
    // Configure the server
    server.Configure(":8080")
    
    // Start the server
    log.Println("Server starting on :8080")
    if err := server.Start(context.Background()); err != nil {
        log.Fatal(err)
    }
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"users": []}`))
}
```

## Core Components

### Server
The main server struct that orchestrates all components.

### Router
Handles HTTP routing with support for exact paths and regex patterns.

### Middleware
Extensible middleware system for request/response processing.

### Filter
Pre-processing filters for requests before they reach handlers.

### Static
Static file serving with template support and SPA compatibility.

### WebSocket
RFC 6455 WebSocket handler with pluggable origin checking, authentication, connection limits, message rate limiting, codec, hub, and rooms. See the [WebSocket](#websocket) section below.

### Health Monitoring
Comprehensive health checking system with built-in and custom checks.

## Health Monitoring

The server includes a built-in health monitoring system with multiple endpoints:

```go
server := goserver.NewServer()

// Add health routes
server.AddHealthRoutes()

// Add custom health checks
server.Health().AddDatabaseCheck("primary", func() error {
    return db.Ping()
})

server.Health().AddExternalServiceCheck("api", "https://api.example.com/health")
```

### Health Endpoints

- `GET /health` - Comprehensive health status with all checks
- `GET /health/ready` - Readiness check for load balancers
- `GET /health/live` - Simple liveness check

### Built-in Health Checks

- **Memory Usage**: Monitor memory consumption with configurable limits
- **Goroutine Count**: Track goroutine leaks and resource usage
- **HTTP Dependencies**: Check external service availability
- **Database Connectivity**: Monitor database connection health
- **Custom Checks**: Add application-specific health logic

Example health response:
```json
{
  "status": "UP",
  "timestamp": "2023-12-07T10:30:00Z",
  "checks": {
    "memory": {
      "status": "UP",
      "message": "Memory usage: 45 MB",
      "details": {"alloc_mb": 45, "max_memory_mb": 512}
    }
  },
  "service_info": {
    "version": "1.0.0",
    "environment": "production"
  }
}
```

## Configuration

```go
config := goserver.ServerConfig{
    Addr:         ":8080",
    IdleTimeout:  60 * time.Second,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 15 * time.Second,
    CORSOrigin:   "*",
    EnableGzip:   true,
}

server := goserver.NewServerWithConfig(config)
```

## Routing

A route pattern is an exact path, a path with `{name}` segments, or a regex
that starts with `^`.

```go
r := server.Router()
r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
    id := req.PathValue("id")
    w.Write([]byte(id))
})
r.Get("/files/{path...}", func(w http.ResponseWriter, req *http.Request) {
    w.Write([]byte(req.PathValue("path"))) // "a/b.txt" for /files/a/b.txt
})
r.Get(`^/orders/(?P<num>[0-9]+)$`, func(w http.ResponseWriter, req *http.Request) {
    w.Write([]byte(req.PathValue("num")))
})
```

- `{name}` matches one non-empty path segment. `{name...}` must be the last
  segment and matches the rest of the path, which must not be empty.
- Named regex groups are also read with `req.PathValue`.
- When several routes match, the one registered first wins.
- If the path matches a route only under other methods, the router answers
  `405 Method Not Allowed` with an `Allow` header listing them. Otherwise it
  answers `404`. HEAD is not served by GET routes, so a HEAD request to a
  GET-only route gets a 405.

To change those responses:

```go
r.SetNotFoundHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
    http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
}))
r.SetMethodNotAllowedHandler(myHandler) // Allow is already set when it runs
```

Passing `nil` to either setter restores the default.

## Errors

Package `httperr` gives handlers one error shape. Each error carries an HTTP status, a stable code and a message that is safe to show the client:

```json
{"error": {"code": "not_found", "message": "order 42 not found"}}
```

| Constructor | Status | Code |
|---|---|---|
| `BadRequest(msg)` | 400 | `bad_request` (the request cannot be read) |
| `Invalid(msg)` | 400 | `invalid` (a domain rule is broken) |
| `Unauthorized(msg)` | 401 | `unauthorized` |
| `Forbidden(msg)` | 403 | `forbidden` |
| `NotFound(msg)` | 404 | `not_found` |
| `Conflict(msg)` | 409 | `conflict` |
| `Internal(cause)` | 500 | `internal` |

`New(status, code, msg)` adds your own code. `WithCause(err)` keeps the underlying error for logs and `errors.Is`; it is never sent. `httperr.Write(w, err)` finds the `*httperr.Error` in err's chain and writes it. Any other error becomes a 500, and every 5xx answers with `"internal error"`, so no internal detail reaches the client.

Domain code keeps its own error types and does not import `httperr`. Map them in one place in the HTTP adapter:

```go
func (h *Handlers) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		err = httperr.NotFound("order not found").WithCause(err)
	case errors.Is(err, domain.ErrAlreadyPlaced):
		err = httperr.Conflict("order already placed").WithCause(err)
	}
	if e := httperr.From(err); e.Status >= http.StatusInternalServerError {
		h.log.Error("request failed", "error", err)
	}
	httperr.Write(w, err)
}
```

## Middleware

No middleware is on by default: `New`, `NewServer` and `DefaultServerConfig`
add no CORS, cache or gzip handling. Turn each one on through the
`CORSOrigin`, `CacheDuration` and `EnableGzip` config fields or with
`WithMiddleware`.

### Order

The first middleware added is the outermost one: it sees the request first
and the response last. `Configure` adds the middleware from config fields
(CORS, cache, gzip, timeout) before anything passed with `WithMiddleware`, so
those sit outside it.

Put `Recovery` before `StructuredLogging`. `StructuredLogging` logs a panic
and then panics again, so something outside it has to recover.

### Recovery

`middleware.Recovery()` turns a handler panic into a plain-text
`500 internal server error` and logs the value and stack. If the response has
already started, it logs the panic and aborts the connection instead, so the
client sees a cut-off response rather than one that looks complete.
`RecoveryWithConfig` replaces the log or the response:

```go
import "github.com/gnemade360/go-server/middleware"

server.Configure(":8080",
    goserver.WithMiddleware(middleware.RecoveryWithConfig(middleware.RecoveryConfig{
        Log: func(r *http.Request, recovered any, stack []byte) {
            slog.Error("panic", "path", r.URL.Path, "value", recovered)
        },
        Respond: func(w http.ResponseWriter, r *http.Request, recovered any) {
            w.Header().Set("Content-Type", "application/json")
            w.WriteHeader(http.StatusInternalServerError)
            w.Write([]byte(`{"error":"internal"}`))
        },
    })),
    goserver.WithMiddleware(goserver.StructuredLogging(goserver.DefaultLoggingConfig())),
)
```

`Respond` runs only when nothing has been written yet.

Built-in middleware includes:

- **CORS**: Cross-origin resource sharing
- **Gzip**: Response compression
- **Cache Control**: HTTP caching headers
- **Recovery**: Panic recovery and logging
- **Timeout**: Request timeout handling

```go
server := goserver.NewServer()
server.Configure(":8080",
    goserver.WithMiddleware(goserver.CORS("*")),
    goserver.WithMiddleware(goserver.Gzip()),
    goserver.WithTimeout(30*time.Second),
)
```

## Shutdown

`Start` serves until its context is cancelled. It then shuts down
gracefully, waiting up to 10 seconds for open requests, and returns the
result of `Shutdown`: `nil`, or `context.DeadlineExceeded` if requests were
still running when time ran out. `WithShutdownTimeout` changes the wait; zero
or less means no deadline.

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
defer stop()

server.Configure(":8080", goserver.WithShutdownTimeout(30*time.Second))
if err := server.Start(ctx); err != nil {
    log.Fatal(err)
}
```

If you call `server.Shutdown` yourself, `Start` returns `nil` as soon as the
shutdown begins, before open requests finish. Wait for your own `Shutdown`
call to return before exiting. Any other error from listening, such as a port
already in use, is returned as is.

## Static Files

```go
// Serve static files from ./public directory
server.Router().Static(goserver.StaticOptions{
    Dir:    "./public",
    Prefix: "/static/",
})
```

## WebSocket

The `websocket` package implements RFC 6455 on top of
[`github.com/gorilla/websocket`](https://github.com/gorilla/websocket).
Every concern lives in its own file and is hookable through `Config`;
new behaviour is added by satisfying one of the small interfaces
(`OriginChecker`, `Authenticator`, `ConnectionLimiter`,
`MessageCodec`, `MessageRateLimiter`) without touching the upgrade
flow. The handler is re-exported from the root package for a single-
import setup.

```go
import goserver "github.com/gnemade360/go-server"

ws := goserver.NewWebSocketHandler(goserver.WebSocketConfig{
    Origin: goserver.WebSocketAllowOrigins("https://app.example.com"),
    Auth:   goserver.WebSocketBearerTokenAuth(verifyFn),
    OnConnect: func(c *goserver.WebSocketConn) error {
        c.Handler().Hub().Room("lobby").Join(c)
        return nil
    },
    OnMessage: func(c *goserver.WebSocketConn, m goserver.WebSocketMessage) error {
        c.Handler().Hub().Room("lobby").PublishExcept(c, m)
        return nil
    },
})

server.Router().Get("/ws", ws.ServeHTTP)
server.Health().AddCheck("websocket", ws.HealthCheck())
ws.RegisterMetrics(server.Metrics())
```

### Security defaults

Every default is restrictive; opt-out is explicit.

| Concern                    | Default                          |
|----------------------------|----------------------------------|
| Origin check               | Same-origin (defeats CSWSH)      |
| Authentication             | `DenyAll` — must set explicitly  |
| Per-frame read limit       | 1 MiB                            |
| Reassembly cap (cumulative)| 1 MiB                            |
| Ping interval              | 30 s                             |
| Pong deadline              | 60 s                             |
| Handshake timeout          | 10 s                             |
| Per-IP connection cap      | 10                               |
| Global connection cap      | 1000                             |
| permessage-deflate         | Disabled                         |
| UTF-8 validation           | Enforced on text frames          |
| JSON decoder depth         | 64 levels                        |
| Concurrent writes          | Serialized under a mutex         |

### Authentication

`BearerTokenAuth` reads the token from `Authorization: Bearer …`, a
`bearer.<token>` subprotocol entry, or `?access_token=` — in that
order. The subprotocol path is the only browser-friendly way to
carry a bearer header through the WebSocket API; pair it with a
non-secret subprotocol name so the server can echo a non-secret
value back:

```js
new WebSocket("wss://host/ws", ["chat.v1", "bearer." + token])
```

`CookieSessionAuth` reads a named cookie and resolves it through a
lookup function. Pair it with `SameOriginChecker` (the default) and
issue session cookies with the `__Host-` prefix to scope them to a
single origin.

### Hub, rooms, broadcast

Each handler exposes a single in-process hub (lazy-created on first
`Hub()` call) and any number of rooms.

```go
hub := ws.Hub()
hub.Broadcast(websocket.Message{Type: websocket.TextMessage, Data: []byte("notice")})
hub.Room("lobby").PublishExcept(sender, msg)
hub.BroadcastFilter(msg, func(c *websocket.Conn) bool {
    return c.Principal().ID != ""
})
```

### Graceful shutdown

`ws.Shutdown(ctx)` sends close code 1001 to every open connection,
waits for them to drain (bounded by the context deadline), and
stops the hub goroutine. Call it from your signal handler before
`server.Shutdown`.

### Example

See [`examples/websocket-example`](examples/websocket-example/) for
a runnable chat-style demo wired through router, health, and
metrics.



## License

This project is licensed under the MIT License - see the LICENSE file for details.