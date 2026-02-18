package websocket

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// DeliveryEvent represents a real-time delivery update sent to dashboard clients.
type DeliveryEvent struct {
	Type         string    `json:"type"` // "delivery_success", "delivery_failed", "delivery_retrying", "delivery_dlq"
	EventID      string    `json:"event_id"`
	SubscriberID string    `json:"subscriber_id"`
	EndpointURL  string    `json:"endpoint_url"`
	EventType    string    `json:"event_type"`
	Attempt      int       `json:"attempt"`
	StatusCode   *int      `json:"status_code,omitempty"`
	ResponseMs   int64     `json:"response_ms"`
	Error        string    `json:"error,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// Hub manages WebSocket connections and broadcasts events to all connected clients.
type Hub struct {
	clients    map[*client]struct{}
	mu         sync.RWMutex
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
	logger     *slog.Logger
}

type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		logger:     logger,
	}
}

// Run starts the hub's event loop. Should be called as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
			h.logger.Debug("websocket client connected", "total_clients", len(h.clients))

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			h.logger.Debug("websocket client disconnected", "total_clients", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- message:
				default:
					// Client buffer full — drop it
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, c)
					close(c.send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a delivery event to all connected WebSocket clients.
func (h *Hub) Broadcast(event DeliveryEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal websocket event", "error", err)
		return
	}

	select {
	case h.broadcast <- data:
	default:
		h.logger.Warn("websocket broadcast channel full, dropping event")
	}
}

// HandleWebSocket upgrades HTTP connections to WebSocket and registers the client.
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	c := &client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- c

	go c.writePump()
	go c.readPump()
}

// readPump reads messages from the WebSocket connection (handles pings/disconnects).
func (c *client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ClientCount returns the number of connected WebSocket clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
