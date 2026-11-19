package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	gws "github.com/gorilla/websocket"

	"notifier/panel/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsPingPeriod = 50 * time.Second
)

// WebSocketHandler streams real-time notification status updates.
type WebSocketHandler struct {
	hub      *websocket.Hub
	upgrader gws.Upgrader
	log      *slog.Logger
}

// NewWebSocketHandler creates the handler.
func NewWebSocketHandler(hub *websocket.Hub, log *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		log: log,
		upgrader: gws.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

// Serve handles GET /ws/notifications?notification_id=...&batch_id=...
func (h *WebSocketHandler) Serve(w http.ResponseWriter, r *http.Request) {
	sub := websocket.Subscription{
		NotificationID: r.URL.Query().Get("notification_id"),
		BatchID:        r.URL.Query().Get("batch_id"),
	}
	for _, v := range []string{sub.NotificationID, sub.BatchID} {
		if v != "" {
			if _, err := uuid.Parse(v); err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_filter", "notification_id / batch_id must be UUIDs", nil)
				return
			}
		}
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("websocket upgrade failed", slog.Any("error", err))
		return
	}
	client := h.hub.Register(sub)
	h.log.Info("websocket client connected", slog.String("notification_id", sub.NotificationID), slog.String("batch_id", sub.BatchID))

	done := make(chan struct{})
	// Reader: only used to process control frames / detect disconnects.
	go func() {
		defer close(done)
		conn.SetReadLimit(512)
		_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
		conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(wsPongWait)) })
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(wsPingPeriod)
	defer func() {
		ticker.Stop()
		h.hub.Unregister(client)
		_ = conn.Close()
		h.log.Info("websocket client disconnected")
	}()
	for {
		select {
		case <-done:
			return
		case msg, ok := <-client.Messages():
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(gws.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
