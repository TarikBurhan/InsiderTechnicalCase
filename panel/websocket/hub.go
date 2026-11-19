// Package websocket pushes real-time notification status changes to
// subscribed WebSocket clients. Status changes come from PostgreSQL
// LISTEN/NOTIFY (see listener.go), so any service updating a notification
// automatically reaches the clients without extra coupling.
package websocket

import (
	"encoding/json"
	"sync"

	"notifier/panel/dto"
)

// Subscription filters which events a client receives. Empty fields match all.
type Subscription struct {
	NotificationID string
	BatchID        string
}

func (s Subscription) matches(e dto.StatusEvent) bool {
	if s.NotificationID != "" && s.NotificationID != e.ID {
		return false
	}
	if s.BatchID != "" && (e.BatchID == nil || *e.BatchID != s.BatchID) {
		return false
	}
	return true
}

// Client is a connected subscriber.
type Client struct {
	sub  Subscription
	send chan []byte
}

// Hub fans out status events to clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// NewHub creates a hub.
func NewHub() *Hub { return &Hub{clients: map[*Client]struct{}{}} }

// Register adds a client and returns it.
func (h *Hub) Register(sub Subscription) *Client {
	c := &Client{sub: sub, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	return c
}

// Unregister removes a client.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// Broadcast sends an event to every matching client. Slow clients whose
// buffer is full drop the event rather than blocking everybody.
func (h *Hub) Broadcast(e dto.StatusEvent) {
	payload, err := json.Marshal(map[string]any{"type": "notification.status", "data": e})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if !c.sub.matches(e) {
			continue
		}
		select {
		case c.send <- payload:
		default:
		}
	}
}

// Count returns the number of connected clients.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Messages returns the client's outgoing message channel.
func (c *Client) Messages() <-chan []byte { return c.send }
