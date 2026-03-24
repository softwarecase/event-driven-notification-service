package event

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/softwarecase/event-driven-notification-service/internal/port"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second

	redisPubSubChannel = "notifications:events"
)

type Client struct {
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	batchID string // optional filter
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan port.StatusEvent
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	logger     *slog.Logger
	redis      *redis.Client
	subscriber bool // true = API mode (subscribe only), false = Worker mode (publish only)
}

func NewHub(logger *slog.Logger, redisClient *redis.Client, subscriber bool) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan port.StatusEvent, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		redis:      redisClient,
		subscriber: subscriber,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Info("websocket client connected", "total", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("websocket client disconnected", "total", len(h.clients))

		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				h.logger.Error("marshal event", "error", err)
				continue
			}
			h.mu.RLock()
			for client := range h.clients {
				// Filter by batch ID if client specified one
				if client.batchID != "" && client.batchID != event.BatchID {
					continue
				}
				select {
				case client.send <- data:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Publish sends an event to local WebSocket clients and to Redis Pub/Sub
// so that other processes (e.g. API) can relay it to their own clients.
func (h *Hub) Publish(event port.StatusEvent) {
	// Local broadcast (for clients connected to this process)
	select {
	case h.broadcast <- event:
	default:
		h.logger.Warn("event broadcast channel full, dropping event")
	}

	// Publish to Redis only in non-subscriber mode (Worker)
	// Subscriber mode (API) skips this to avoid duplicate events
	if h.redis != nil && !h.subscriber {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		h.redis.Publish(context.Background(), redisPubSubChannel, data)
	}
}

// SubscribeRedis listens for events published by other processes (e.g. Worker)
// and relays them to local WebSocket clients.
func (h *Hub) SubscribeRedis(ctx context.Context) {
	if h.redis == nil {
		return
	}
	sub := h.redis.Subscribe(ctx, redisPubSubChannel)
	ch := sub.Channel()

	h.logger.Info("subscribed to Redis event channel", "channel", redisPubSubChannel)

	for {
		select {
		case <-ctx.Done():
			_ = sub.Close()
			return
		case msg := <-ch:
			var evt port.StatusEvent
			if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
				h.logger.Error("unmarshal redis event", "error", err)
				continue
			}
			// Feed into local broadcast (don't re-publish to Redis)
			select {
			case h.broadcast <- evt:
			default:
			}
		}
	}
}

func (h *Hub) Register(conn *websocket.Conn, batchID string) {
	client := &Client{
		hub:     h,
		conn:    conn,
		send:    make(chan []byte, 256),
		batchID: batchID,
	}
	h.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
