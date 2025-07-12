// Package websocket provides WebSocket-like functionality for server applications.
package websocket

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

// SimpleHandler implements the Handler interface using a simplified WebSocket-like approach.
// This provides real-time communication between the server and clients using standard HTTP.
type SimpleHandler struct {
	// Map of connected clients
	clients *sync.Map

	// Channel for errors
	errCh chan error

	// Channel for broadcasting messages to all clients
	emitter chan interface{}

	// Channel for receiving messages from clients
	receiver chan interface{}

	// Name of the handler for logging
	Name string

	// Logger
	logger *log.Logger
}

// NewSimpleHandler creates a new SimpleHandler with the given name and logger.
func NewSimpleHandler(name string, logger *log.Logger) *SimpleHandler {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &SimpleHandler{
		Name:     name,
		clients:  new(sync.Map),
		errCh:    make(chan error, 100),
		emitter:  make(chan interface{}, 100),
		receiver: make(chan interface{}, 100),
		logger:   logger,
	}
}

// ServeHTTP implements the http.Handler interface
func (h *SimpleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if this is a new client or an existing one
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		// New client, generate a client ID
		clientID = generateID()
		h.logger.Printf("New client connected: %s", clientID)

		// Create a new client
		client := NewClient(clientID, h.logger)

		// Store the client
		h.clients.Store(clientID, client)

		// Return the client ID
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"client_id": clientID,
		})
		return
	}

	// Existing client, check if it exists
	clientObj, ok := h.clients.Load(clientID)
	if !ok {
		h.logger.Printf("Client not found: %s", clientID)
		http.Error(w, "Client not found", http.StatusNotFound)
		return
	}
	client := clientObj.(*Client)

	// Check if this is a send or receive request
	if r.Method == http.MethodPost {
		// Client is sending a message
		var message interface{}
		if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
			h.logger.Printf("Error decoding message: %v", err)
			http.Error(w, "Error decoding message", http.StatusBadRequest)
			return
		}

		// Add the message to the client's receiver channel
		client.receiver <- message

		// Also add it to the handler's receiver channel
		h.receiver <- message

		// Return success
		w.WriteHeader(http.StatusOK)
		return
	} else if r.Method == http.MethodGet {
		// Client is receiving messages
		// Use short polling with a timeout to simulate WebSockets
		select {
		case message, ok := <-client.emitter:
			if !ok {
				h.logger.Printf("Client channel closed: %s", clientID)
				http.Error(w, "Client channel closed", http.StatusInternalServerError)
				return
			}

			// Return the message
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(message)
			return
		case <-r.Context().Done():
			// Request was cancelled
			return
		}
	}

	// Invalid method
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Broadcast sends a message to all connected clients
func (h *SimpleHandler) Broadcast(message interface{}) {
	h.logger.Printf("Broadcasting message to all clients")
	var count int
	h.clients.Range(func(key, value interface{}) bool {
		client := value.(*Client)
		select {
		case client.emitter <- message:
			count++
		default:
			// Channel is full, remove the client
			h.clients.Delete(key)
			h.logger.Printf("Removed client %s (channel full)", key)
		}
		return true
	})
	h.logger.Printf("Broadcast message to %d clients", count)
}

// Send sends a message to a specific client
func (h *SimpleHandler) Send(clientID string, message interface{}) error {
	clientObj, ok := h.clients.Load(clientID)
	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}
	client := clientObj.(*Client)

	select {
	case client.emitter <- message:
		return nil
	default:
		// Channel is full, remove the client
		h.clients.Delete(clientID)
		h.logger.Printf("Removed client %s (channel full)", clientID)
		return fmt.Errorf("client channel full: %s", clientID)
	}
}

// Receive returns a channel that receives messages from clients
func (h *SimpleHandler) Receive() <-chan interface{} {
	return h.receiver
}

// RemoveClient removes a client from the handler
func (h *SimpleHandler) RemoveClient(clientID string) {
	h.clients.Delete(clientID)
	h.logger.Printf("Removed client %s", clientID)
}

// ClientCount returns the number of connected clients
func (h *SimpleHandler) ClientCount() int {
	var count int
	h.clients.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
