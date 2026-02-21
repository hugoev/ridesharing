package payment

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
)

// Handler holds the HTTP handlers for the payment service.
type Handler struct {
	client pb.PaymentServiceClient
}

// NewHandler creates a new payment handler connected to gRPC.
func NewHandler(client pb.PaymentServiceClient) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes adds payment routes to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	payments := rg.Group("/payments")
	{
		payments.POST("/charge", h.ChargeRide)
		payments.GET("/:ride_id", h.GetPayment)
	}
}

// ChargeRide is a test endpoint to manually trigger a charge.
func (h *Handler) ChargeRide(c *gin.Context) {
	type chargeRequest struct {
		RideID string  `json:"ride_id" binding:"required"`
		Amount float64 `json:"amount" binding:"required"`
	}
	var req chargeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	pbResp, err := h.client.ChargeRide(c.Request.Context(), &pb.ChargeRideRequest{
		RideId: req.RideID,
		Amount: req.Amount,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapPBPayment(pbResp.Payment))
}

// GetPayment retrieves payment status for a ride.
func (h *Handler) GetPayment(c *gin.Context) {
	rideID := c.Param("ride_id")

	pbResp, err := h.client.GetPayment(c.Request.Context(), &pb.GetPaymentRequest{
		RideId: rideID,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapPBPayment(pbResp.Payment))
}

func mapPBPayment(p *pb.Payment) *models.Payment {
	return &models.Payment{}
}

func handleError(c *gin.Context, err error) {
	log.Error().Err(err).Msg("gRPC call failed")
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "internal server error",
	})
}
