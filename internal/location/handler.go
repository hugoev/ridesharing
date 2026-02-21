package location

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
)

// Handler holds the HTTP handlers for the location service.
type Handler struct {
	client pb.LocationServiceClient
}

// NewHandler creates a new location handler.
func NewHandler(client pb.LocationServiceClient) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes adds location routes to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	loc := rg.Group("/locations")
	{
		loc.POST("/update", h.UpdateLocation)
		loc.GET("/nearby", h.NearbyDrivers)
	}
}

// UpdateLocationRequest represents a location update payload.
type UpdateLocationRequest struct {
	Lat  float64 `json:"lat" binding:"required,min=-90,max=90"`
	Long float64 `json:"long" binding:"required,min=-180,max=180"`
}

// UpdateLocation updates a driver's GPS position.
func (h *Handler) UpdateLocation(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":      "unauthorized",
			"error_code": apperrors.CodeUnauthorized,
		})
		return
	}

	role, _ := c.Get("user_role")
	if role.(string) != "driver" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "only drivers can update location",
			"error_code": apperrors.CodeForbidden,
		})
		return
	}

	var req UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	_, err := h.client.UpdateLocation(c.Request.Context(), &pb.UpdateLocationRequest{
		UserId: userID.(uuid.UUID).String(),
		Lat:    req.Lat,
		Long:   req.Long,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to update location")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "failed to update location",
			"error_code": apperrors.CodeInternal,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "location updated"})
}

// NearbyDrivers finds available drivers near a point.
func (h *Handler) NearbyDrivers(c *gin.Context) {
	lat, err := parseFloat(c.Query("lat"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "invalid lat parameter",
			"error_code": apperrors.CodeInvalidCoordinates,
		})
		return
	}

	long, err := parseFloat(c.Query("long"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "invalid long parameter",
			"error_code": apperrors.CodeInvalidCoordinates,
		})
		return
	}

	radius, err := parseFloat(c.Query("radius"))
	if err != nil || radius <= 0 {
		radius = 5.0 // default 5km
	}

	pbResp, err := h.client.GetNearbyDrivers(c.Request.Context(), &pb.GetNearbyDriversRequest{
		Lat:      lat,
		Long:     long,
		RadiusKm: radius,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to get nearby drivers")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "failed to get nearby drivers",
			"error_code": apperrors.CodeInternal,
		})
		return
	}

	var drivers []models.NearbyDriver
	for _, d := range pbResp.Drivers {
		driverID, _ := uuid.Parse(d.DriverId)
		drivers = append(drivers, models.NearbyDriver{
			DriverID:     driverID,
			Lat:          d.Lat,
			Long:         d.Long,
			DistanceKm:   d.DistanceKm,
			EstimatedETA: int(d.EstimatedEta),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data": drivers,
	})
}

// Helper to parse floats from query params
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty parameter")
	}
	return strconv.ParseFloat(s, 64)
}
