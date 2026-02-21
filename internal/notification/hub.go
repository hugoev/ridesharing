// Package notification implements the real-time notification service using WebSockets and Kafka.
package notification

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Client represents a connected WebSocket client.
type Client struct {
	UserID uuid.UUID
	Conn   *websocket.Conn
	Send   chan []byte
}

// Hub maintains the set of active WebSocket clients and broadcasts messages.
type Hub struct {
	// Registered clients indexed by user ID
	clients map[uuid.UUID]*Client
	mu      sync.RWMutex

	// Channel for broadcasting messages to specific users
	broadcast chan *UserMessage

	// Register/unregister channels
	register   chan *Client
	unregister chan *Client
}

// UserMessage targets a notification to a specific user.
type UserMessage struct {
	UserID  uuid.UUID
	Payload []byte
}

// NewHub creates a new notification hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		broadcast:  make(chan *UserMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's event loop. Should be run as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			// Close existing connection for this user if any
			if old, exists := h.clients[client.UserID]; exists {
				close(old.Send)
				if old.Conn != nil {
					old.Conn.Close()
				}
			}
			h.clients[client.UserID] = client
			h.mu.Unlock()
			log.Info().Str("user_id", client.UserID.String()).Msg("client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if c, ok := h.clients[client.UserID]; ok && c == client {
				close(client.Send)
				delete(h.clients, client.UserID)
			}
			h.mu.Unlock()
			log.Info().Str("user_id", client.UserID.String()).Msg("client disconnected")

		case msg := <-h.broadcast:
			h.mu.RLock()
			client, exists := h.clients[msg.UserID]
			h.mu.RUnlock()

			if exists {
				select {
				case client.Send <- msg.Payload:
				default:
					// Client's buffer is full, disconnect them
					h.unregister <- client
				}
			}
		}
	}
}

// SendToUser sends a notification to a specific user.
func (h *Hub) SendToUser(userID uuid.UUID, eventType string, data interface{}) {
	payload, err := json.Marshal(map[string]interface{}{
		"event": eventType,
		"data":  data,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal notification")
		return
	}

	h.broadcast <- &UserMessage{
		UserID:  userID,
		Payload: payload,
	}
}

// BroadcastToUsers sends a notification to multiple users.
func (h *Hub) BroadcastToUsers(userIDs []uuid.UUID, eventType string, data interface{}) {
	for _, id := range userIDs {
		h.SendToUser(id, eventType, data)
	}
}

// ConnectedCount returns the number of connected clients.
func (h *Hub) ConnectedCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// writePump pumps messages from the hub to the WebSocket connection.
func writePump(client *Client) {
	defer func() {
		client.Conn.Close()
	}()

	for message := range client.Send {
		if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Error().Err(err).
				Str("user_id", client.UserID.String()).
				Msg("websocket write error")
			return
		}
	}
}

// readPump pumps messages from the WebSocket connection to the hub.
// It handles client disconnection detection.
func readPump(client *Client, hub *Hub) {
	defer func() {
		hub.unregister <- client
		client.Conn.Close()
	}()

	for {
		_, _, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Error().Err(err).Str("user_id", client.UserID.String()).Msg("websocket read error")
			}
			break
		}
		// We don't process incoming messages from clients in this version
	}
}
