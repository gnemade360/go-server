# Go Server

A lightweight, extensible HTTP server library for Go with built-in middleware support, routing, and static file serving.

## Features

- Simple and intuitive API
- Built-in middleware system (CORS, Gzip, Cache Control, Recovery, Timeout)
- Flexible routing with regex support
- Static file serving with SPA support
- WebSocket support
- Configurable timeouts and server options
- Production-ready with graceful shutdown

## Installation

```bash
go get github.com/yourorg/go-server
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    "net/http"
    
    "github.com/yourorg/go-server"
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
WebSocket support for real-time applications.

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

## Middleware

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

## Static Files

```go
// Serve static files from ./public directory
server.Router().Static(goserver.StaticOptions{
    Dir:    "./public",
    Prefix: "/static/",
})
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.