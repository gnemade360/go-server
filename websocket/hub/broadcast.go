package hub

import "github.com/gnemade360/go-server/websocket/internal/types"

// Broadcast sends m to every connection registered with the hub.
func (h *Hub) Broadcast(m types.Message) {
	select {
	case h.broadcast <- hubBroadcast{msg: m}:
	default:
	}
}

// BroadcastExcept sends m to every connection except sender.
func (h *Hub) BroadcastExcept(sender types.ConnRef, m types.Message) {
	senderID := ""
	if sender != nil {
		senderID = sender.ID()
	}
	h.BroadcastFilter(m, func(c types.ConnRef) bool { return c.ID() != senderID })
}

// BroadcastFilter sends m to every connection where filter returns true.
// filter runs on the hub goroutine; keep it cheap.
func (h *Hub) BroadcastFilter(m types.Message, filter func(types.ConnRef) bool) {
	select {
	case h.broadcast <- hubBroadcast{msg: m, filter: filter}:
	default:
	}
}
