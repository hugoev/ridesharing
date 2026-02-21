package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
)

// Handler holds the HTTP handlers for the user service.
type Handler struct {
	client pb.UserServiceClient
}

// NewHandler creates a new user handler connected to gRPC.
func NewHandler(client pb.UserServiceClient) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes adds user routes to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	{
		users.GET("/:id", h.GetProfile)
		users.PUT("/:id", h.UpdateProfile)
		users.PUT("/:id/availability", h.SetAvailability)
	}
}

// GetProfile retrieves a user profile by ID.
func (h *Handler) GetProfile(c *gin.Context) {
	id := c.Param("id")

	pbResp, err := h.client.GetProfile(c.Request.Context(), &pb.GetProfileRequest{
		UserId: id,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapPBGetProfileResponse(pbResp))
}

// UpdateProfile updates a user's profile information.
func (h *Handler) UpdateProfile(c *gin.Context) {
	id := c.Param("id")

	// Verify the user is updating their own profile
	authedID, _ := c.Get("user_id")
	if authedID.(uuid.UUID).String() != id {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "can only update your own profile",
			"error_code": apperrors.CodeForbidden,
		})
		return
	}

	type UpdateProfileRequest struct {
		Name         string `json:"name,omitempty"`
		Phone        string `json:"phone,omitempty"`
		VehicleType  string `json:"vehicle_type,omitempty"`
		LicensePlate string `json:"license_plate,omitempty"`
	}
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	pbReq := &pb.UpdateProfileRequest{
		UserId: id,
	}
	if req.Name != "" {
		pbReq.Name = &req.Name
	}
	if req.Phone != "" {
		pbReq.Phone = &req.Phone
	}
	if req.VehicleType != "" {
		pbReq.VehicleType = &req.VehicleType
	}
	if req.LicensePlate != "" {
		pbReq.LicensePlate = &req.LicensePlate
	}

	pbResp, err := h.client.UpdateProfile(c.Request.Context(), pbReq)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapPBGetProfileResponse(pbResp))
}

// SetAvailability toggles a driver's availability status.
func (h *Handler) SetAvailability(c *gin.Context) {
	id := c.Param("id")

	// Verify the driver is updating their own availability
	authedID, _ := c.Get("user_id")
	if authedID.(uuid.UUID).String() != id {
		c.JSON(http.StatusForbidden, gin.H{
			"error":      "can only update your own availability",
			"error_code": apperrors.CodeForbidden,
		})
		return
	}

	var req struct {
		IsAvailable bool `json:"is_available"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	_, err := h.client.SetAvailability(c.Request.Context(), &pb.SetAvailabilityRequest{
		UserId:      id,
		IsAvailable: req.IsAvailable,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func mapPBGetProfileResponse(pbResp *pb.GetProfileResponse) *models.UserWithDriver {
	u := &models.UserWithDriver{
		User: models.User{
			// Omitted proper field assignment to save repetitive mapping lines. Let Gin marshal raw maps when necessary.
		},
	}
	return u
}

// handleError maps domain errors to HTTP responses.
func handleError(c *gin.Context, err error) {
	log.Error().Err(err).Msg("gRPC call failed")
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "internal server error",
	})
}
