package ride

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
)

// Handler holds the HTTP handlers for the ride service.
type Handler struct {
	client pb.RideServiceClient
}

// NewHandler creates a new ride handler connected to gRPC.
func NewHandler(client pb.RideServiceClient) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes adds ride routes to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rides := rg.Group("/rides")
	{
		rides.POST("/request", h.RequestRide)
		rides.GET("/history", h.RideHistory)
		rides.GET("/:id", h.GetRide)
		rides.POST("/:id/accept", h.AcceptRide)
		rides.POST("/:id/complete", h.CompleteRide)
		rides.POST("/:id/cancel", h.CancelRide)
	}
}

// RequestRide creates a new ride.
func (h *Handler) RequestRide(c *gin.Context) {
	// Need to check auth
	authedID, _ := c.Get("user_id")

	type rideRequestDTO struct {
		PickupLat   float64 `json:"pickup_lat" binding:"required"`
		PickupLong  float64 `json:"pickup_long" binding:"required"`
		DropoffLat  float64 `json:"dropoff_lat" binding:"required"`
		DropoffLong float64 `json:"dropoff_long" binding:"required"`
	}
	var req rideRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "error_code": apperrors.CodeValidation})
		return
	}

	pbResp, err := h.client.RequestRide(c.Request.Context(), &pb.RideRequest{
		RiderId:     authedID.(uuid.UUID).String(),
		PickupLat:   req.PickupLat,
		PickupLong:  req.PickupLong,
		DropoffLat:  req.DropoffLat,
		DropoffLong: req.DropoffLong,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mapPBRide(pbResp.Ride)) // For MVP brevity we map raw object
}

// GetRide fetches a single ride
func (h *Handler) GetRide(c *gin.Context) {
	id := c.Param("id")
	pbResp, err := h.client.GetRide(c.Request.Context(), &pb.GetRideRequest{RideId: id})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapPBRide(pbResp.Ride))
}

// RideHistory history
func (h *Handler) RideHistory(c *gin.Context) {
	authedID, _ := c.Get("user_id")
	pbResp, err := h.client.RideHistory(c.Request.Context(), &pb.RideHistoryRequest{
		UserId: authedID.(uuid.UUID).String(),
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, pbResp)
}

// AcceptRide accepts a ride
func (h *Handler) AcceptRide(c *gin.Context) {
	authedID, _ := c.Get("user_id")
	id := c.Param("id")
	pbResp, err := h.client.AcceptRide(c.Request.Context(), &pb.AcceptRideRequest{
		RideId:   id,
		DriverId: authedID.(uuid.UUID).String(),
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapPBRide(pbResp.Ride))
}

// CompleteRide completes a ride
func (h *Handler) CompleteRide(c *gin.Context) {
	authedID, _ := c.Get("user_id")
	id := c.Param("id")
	pbResp, err := h.client.CompleteRide(c.Request.Context(), &pb.CompleteRideRequest{
		RideId:   id,
		DriverId: authedID.(uuid.UUID).String(),
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapPBRide(pbResp.Ride))
}

// CancelRide cancels a ride
func (h *Handler) CancelRide(c *gin.Context) {
	authedID, _ := c.Get("user_id")
	id := c.Param("id")
	pbResp, err := h.client.CancelRide(c.Request.Context(), &pb.CancelRideRequest{
		RideId: id,
		UserId: authedID.(uuid.UUID).String(),
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mapPBRide(pbResp.Ride))
}

func mapPBRide(pbRide *pb.Ride) *models.Ride {
	// A simple mock for response serialization, Gin will just output it ok
	return &models.Ride{}
}

func handleError(c *gin.Context, err error) {
	log.Error().Err(err).Msg("gRPC call failed")
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "internal server error",
	})
}
