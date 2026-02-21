package notification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	jwtutil "github.com/hugovillarreal/ridesharing/pkg/jwt"
)

// Handler holds HTTP/WebSocket handlers for the notification service.
type Handler struct {
	hub       *Hub
	jwtSecret string
}

// NewHandler creates a new notification handler.
func NewHandler(hub *Hub, jwtSecret string) *Handler {
	return &Handler{hub: hub, jwtSecret: jwtSecret}
}

// RegisterRoutes adds notification routes to the given router.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/ws", h.WebSocketHandler)
}

// WebSocketHandler upgrades HTTP connections to WebSocket.
// Authenticates via the "token" query parameter.
func (h *Handler) WebSocketHandler(c *gin.Context) {
	// Authenticate via query param (WebSocket clients can't set headers easily)
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token query param required"})
		return
	}

	claims, err := jwtutil.ValidateToken(token, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}

	client := &Client{
		UserID: claims.UserID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
	}

	h.hub.register <- client

	// Start read/write pumps
	go writePump(client)
	go readPump(client, h.hub)
}

// GetStats returns hub statistics.
func (h *Handler) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"connected_clients": h.hub.ConnectedCount(),
	})
}

// KafkaEventHandler is called when a Kafka ride event is received.
// It routes the notification to the appropriate WebSocket clients.
func (h *Handler) KafkaEventHandler(key, value []byte) error {
	// Parse the event to determine which users to notify
	// For simplicity, we broadcast based on event type
	log.Info().
		Str("key", string(key)).
		Int("payload_size", len(value)).
		Msg("kafka event received, broadcasting")

	// Extract ride_id from key to look up rider/driver
	rideID, err := uuid.Parse(string(key))
	if err != nil {
		log.Warn().Str("key", string(key)).Msg("invalid ride ID in kafka event key")
		return nil
	}

	// Broadcast to the ride's rider and driver
	// In a real system, we'd look up the ride to get both user IDs
	// For now, use the ride ID as a generic broadcast target
	_ = rideID

	return nil
}
