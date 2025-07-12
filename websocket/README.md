# WebSocket Package

This package provides WebSocket-like functionality for server applications. It includes two implementations of the `Handler` interface:

1. **LongPollHandler**: Uses HTTP long polling to provide WebSocket-like functionality without requiring actual WebSockets. This makes it compatible with environments where WebSocket connections might be restricted.

2. **SimpleHandler**: Uses a simplified HTTP-based approach for environments that need a lighter implementation.

Both handlers implement a common `Handler` interface, allowing them to be used interchangeably.

## Usage

### Creating a Handler

```go
// Create a LongPollHandler
handler := websocket.NewLongPollHandler("myhandler", logger)

// Or create a SimpleHandler
handler := websocket.NewSimpleHandler("myhandler", logger)
```

### Using the Handler in an HTTP Server

```go
// Create a new HTTP server
mux := http.NewServeMux()
mux.Handle("/ws/", handler)

// Start the server
http.ListenAndServe(":8080", mux)
```

### Sending Messages to Clients

```go
// Broadcast a message to all clients
handler.Broadcast(message)

// Send a message to a specific client
handler.Send(clientID, message)
```

### Receiving Messages from Clients

```go
// Receive messages from clients
for {
    select {
    case msg := <-handler.Receive():
        // Handle the message
    }
}
```

## Handler Interface

The `Handler` interface defines the following methods:

- `ServeHTTP(w http.ResponseWriter, r *http.Request)`: Implements the `http.Handler` interface.
- `Broadcast(message interface{})`: Sends a message to all connected clients.
- `Send(clientID string, message interface{}) error`: Sends a message to a specific client.
- `Receive() <-chan interface{}`: Returns a channel that receives messages from clients.
- `RemoveClient(clientID string)`: Removes a client from the handler.
- `ClientCount() int`: Returns the number of connected clients.

## Client

The `Client` struct represents a client connected to a WebSocket-like handler. It includes:

- `ID`: The client's unique ID.
- `emitter`: A channel for sending messages to the client.
- `receiver`: A channel for receiving messages from the client.
- `logger`: A logger for logging client-related messages.

## Implementation Details

### LongPollHandler

The `LongPollHandler` uses HTTP long polling to simulate WebSocket connections. When a client wants to receive messages, it makes a GET request that is held open until a message is available or a timeout occurs. When a client wants to send a message, it makes a POST request with the message in the request body.

### SimpleHandler

The `SimpleHandler` uses a simplified HTTP-based approach similar to the `LongPollHandler`, but with shorter polling intervals. This provides a lighter implementation for environments that don't need the full long polling functionality.
