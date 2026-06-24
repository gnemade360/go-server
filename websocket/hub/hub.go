// Package hub provides the in-process fan-out plane: a Hub that
// registers connections, rooms that group them, and broadcast helpers.
//
// Hub stores connections behind the types.ConnRef interface so it
// remains independent of the conn package's concrete type.
package hub

import (
	"sync"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// Hub is the in-process fan-out point for a Handler. It owns its own
// goroutine; mutations happen through channels and snapshots.
type Hub struct {
	register   chan types.ConnRef
	unregister chan types.ConnRef

	broadcast chan hubBroadcast

	stop chan struct{}
	done chan struct{}

	mu    sync.RWMutex
	conns map[string]types.ConnRef

	rooms *roomRegistry
}

type hubBroadcast struct {
	msg    types.Message
	filter func(types.ConnRef) bool
}

// New starts the hub goroutine and returns the Hub.
func New() *Hub {
	h := &Hub{
		register:   make(chan types.ConnRef, 64),
		unregister: make(chan types.ConnRef, 64),
		broadcast:  make(chan hubBroadcast, 256),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
		conns:      make(map[string]types.ConnRef),
		rooms:      newRoomRegistry(),
	}
	go h.run()
	return h
}

// Register adds c to the hub. Implements types.HubRegistrar.
func (h *Hub) Register(c types.ConnRef) {
	select {
	case h.register <- c:
	case <-h.stop:
	}
}

// Unregister removes c from the hub. Implements types.HubRegistrar.
func (h *Hub) Unregister(c types.ConnRef) {
	select {
	case h.unregister <- c:
	case <-h.stop:
	}
}

func (h *Hub) run() {
	defer close(h.done)
	for {
		select {
		case <-h.stop:
			return
		case c := <-h.register:
			h.mu.Lock()
			h.conns[c.ID()] = c
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			delete(h.conns, c.ID())
			h.mu.Unlock()
			h.rooms.leaveAll(c)
		case b := <-h.broadcast:
			h.fanout(b)
		}
	}
}

func (h *Hub) fanout(b hubBroadcast) {
	h.mu.RLock()
	targets := make([]types.ConnRef, 0, len(h.conns))
	for _, c := range h.conns {
		if b.filter == nil || b.filter(c) {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range targets {
		_ = c.SendMessage(b.msg)
	}
}

// Active returns the current registered connection count.
func (h *Hub) Active() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// Conn looks up a connection by ID. Returns nil if unknown.
func (h *Hub) Conn(id string) types.ConnRef {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.conns[id]
}

// Snapshot returns a copy of the current connection set.
func (h *Hub) Snapshot() []types.ConnRef {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]types.ConnRef, 0, len(h.conns))
	for _, c := range h.conns {
		out = append(out, c)
	}
	return out
}

// Room returns the named Room, creating it on first access.
func (h *Hub) Room(name string) *Room {
	return h.rooms.get(name)
}

// Stop terminates the hub goroutine.
func (h *Hub) Stop() {
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}
	<-h.done
}

// IsRunning reports whether the hub goroutine is still alive.
func (h *Hub) IsRunning() bool {
	select {
	case <-h.done:
		return false
	default:
		return true
	}
}
