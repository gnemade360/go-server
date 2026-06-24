package hub

import (
	"sync"

	"github.com/gnemade360/go-server/websocket/internal/types"
)

// Room is a named subset of a Hub's connections.
type Room struct {
	name string

	mu      sync.RWMutex
	members map[string]types.ConnRef
}

func (r *Room) Name() string { return r.name }

func (r *Room) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.members)
}

// Join adds c to the room. Idempotent.
func (r *Room) Join(c types.ConnRef) {
	r.mu.Lock()
	r.members[c.ID()] = c
	r.mu.Unlock()
}

// Leave removes c from the room. Idempotent.
func (r *Room) Leave(c types.ConnRef) {
	r.mu.Lock()
	delete(r.members, c.ID())
	r.mu.Unlock()
}

// Publish sends m to every current member.
func (r *Room) Publish(m types.Message) {
	r.mu.RLock()
	members := make([]types.ConnRef, 0, len(r.members))
	for _, c := range r.members {
		members = append(members, c)
	}
	r.mu.RUnlock()
	for _, c := range members {
		_ = c.SendMessage(m)
	}
}

// PublishExcept sends m to every member except sender (compared by ID).
func (r *Room) PublishExcept(sender types.ConnRef, m types.Message) {
	senderID := ""
	if sender != nil {
		senderID = sender.ID()
	}
	r.mu.RLock()
	members := make([]types.ConnRef, 0, len(r.members))
	for _, c := range r.members {
		if c.ID() == senderID {
			continue
		}
		members = append(members, c)
	}
	r.mu.RUnlock()
	for _, c := range members {
		_ = c.SendMessage(m)
	}
}

// Members returns a snapshot of current members.
func (r *Room) Members() []types.ConnRef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]types.ConnRef, 0, len(r.members))
	for _, c := range r.members {
		out = append(out, c)
	}
	return out
}

type roomRegistry struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func newRoomRegistry() *roomRegistry {
	return &roomRegistry{rooms: make(map[string]*Room)}
}

func (rr *roomRegistry) get(name string) *Room {
	rr.mu.RLock()
	r, ok := rr.rooms[name]
	rr.mu.RUnlock()
	if ok {
		return r
	}
	rr.mu.Lock()
	defer rr.mu.Unlock()
	if r, ok := rr.rooms[name]; ok {
		return r
	}
	r = &Room{name: name, members: make(map[string]types.ConnRef)}
	rr.rooms[name] = r
	return r
}

func (rr *roomRegistry) leaveAll(c types.ConnRef) {
	rr.mu.RLock()
	rooms := make([]*Room, 0, len(rr.rooms))
	for _, r := range rr.rooms {
		rooms = append(rooms, r)
	}
	rr.mu.RUnlock()
	for _, r := range rooms {
		r.Leave(c)
	}
}
