package handler

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/softwarecase/event-driven-notification-service/internal/adapter/event"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	hub    *event.Hub
	logger *slog.Logger
}

func NewWebSocketHandler(hub *event.Hub, logger *slog.Logger) *WebSocketHandler {
	return &WebSocketHandler{hub: hub, logger: logger}
}

func (h *WebSocketHandler) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	batchID := r.URL.Query().Get("batch_id")
	h.hub.Register(conn, batchID)
}
