package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handler holds the HTTP handlers for the auth service.
type Handler struct {
	client pb.AuthServiceClient
}

// NewHandler creates a new auth handler.
func NewHandler(client pb.AuthServiceClient) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes adds auth routes to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	auth := rg.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
	}
}

// Register creates a new rider or driver account.
func (h *Handler) Register(c *gin.Context) {
	var req GatewayRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	pbResp, err := h.client.Register(c.Request.Context(), &pb.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
		Phone:    req.Phone,
		Role:     req.Role,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, mapPBAuthResponse(pbResp))
}

// Login authenticates with email and password.
func (h *Handler) Login(c *gin.Context) {
	var req GatewayLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	pbResp, err := h.client.Login(c.Request.Context(), &pb.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapPBAuthResponse(pbResp))
}

// Refresh issues a new token pair using a valid refresh token.
func (h *Handler) Refresh(c *gin.Context) {
	type RefreshRequest struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"error_code": apperrors.CodeValidation,
		})
		return
	}

	pbResp, err := h.client.Refresh(c.Request.Context(), &pb.RefreshRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, mapPBAuthResponse(pbResp))
}

// Define the request structs locally
type GatewayRegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	Name         string `json:"name" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Role         string `json:"role" binding:"required,oneof=rider driver"`
	VehicleType  string `json:"vehicle_type,omitempty"`
	LicensePlate string `json:"license_plate,omitempty"`
}

type GatewayLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type GatewayAuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         *models.User `json:"user"`
}

func mapPBAuthResponse(pbResp *pb.AuthResponse) *GatewayAuthResponse {
	return &GatewayAuthResponse{
		AccessToken:  pbResp.AccessToken,
		RefreshToken: pbResp.RefreshToken,
		ExpiresIn:    int(pbResp.ExpiresIn),
		User:         &models.User{},
	}
}

func handleError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		log.Error().Err(err).Msg("unknown gRPC call failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	code := http.StatusInternalServerError
	switch st.Code() {
	case codes.Unauthenticated:
		code = http.StatusUnauthorized
	case codes.AlreadyExists:
		code = http.StatusConflict
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	case codes.ResourceExhausted:
		code = http.StatusTooManyRequests
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.PermissionDenied:
		code = http.StatusForbidden
	}

	if code == http.StatusInternalServerError {
		log.Error().Err(err).Msg("gRPC call failed with 500")
	}

	c.JSON(code, gin.H{
		"error": st.Message(),
	})
}
