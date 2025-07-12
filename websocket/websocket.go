// Package websocket provides WebSocket-like functionality for server applications.
package websocket

import (
	"log"
	"net/http"
)

// Handler is an interface for WebSocket-like handlers.
// It can be implemented using actual WebSockets or HTTP long polling.
type Handler interface {
	// ServeHTTP implements the http.Handler interface
	ServeHTTP(w http.ResponseWriter, r *http.Request)

	// Broadcast sends a message to all connected clients
	Broadcast(message interface{})

	// Send sends a message to a specific client
	Send(clientID string, message interface{}) error

	// Receive returns a channel that receives messages from clients
	Receive() <-chan interface{}

	// RemoveClient removes a client from the handler
	RemoveClient(clientID string)

	// ClientCount returns the number of connected clients
	ClientCount() int
}

// Client represents a client connected to a WebSocket-like handler.
type Client struct {
	// ID of the client
	ID string

	// Channel for sending messages to the client
	emitter chan interface{}

	// Channel for receiving messages from the client
	receiver chan interface{}

	// Logger
	logger *log.Logger
}

// NewClient creates a new client with the given ID and logger.
func NewClient(id string, logger *log.Logger) *Client {
	return &Client{
		ID:       id,
		emitter:  make(chan interface{}, 100),
		receiver: make(chan interface{}, 100),
		logger:   logger,
	}
}

// Emitter returns the channel for sending messages to the client.
func (c *Client) Emitter() chan interface{} {
	return c.emitter
}

// Receiver returns the channel for receiving messages from the client.
func (c *Client) Receiver() chan interface{} {
	return c.receiver
}
