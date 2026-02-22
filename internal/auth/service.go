package auth

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	jwtutil "github.com/hugovillarreal/ridesharing/pkg/jwt"
	"github.com/hugovillarreal/ridesharing/pkg/security"
)

// Service implements the auth business logic.
type Service struct {
	repo      RepositoryInterface
	pool      *pgxpool.Pool
	rawRepo   *Repository // needed for WithTx
	jwtSecret string
	jwtExp    time.Duration
	lockout   *security.LoginAttemptTracker
}

// NewService creates a new auth service.
func NewService(repo *Repository, pool *pgxpool.Pool, jwtSecret string, jwtExp time.Duration) *Service {
	return &Service{
		repo:      repo,
		pool:      pool,
		rawRepo:   repo,
		jwtSecret: jwtSecret,
		jwtExp:    jwtExp,
		lockout:   security.NewLoginAttemptTracker(1000, 15*time.Minute),
	}
}

// RegisterRequest is the input for user registration.
type RegisterRequest struct {
	Email        string `json:"email" binding:"required,email"`
	Password     string `json:"password" binding:"required,min=8"`
	Name         string `json:"name" binding:"required,min=1,max=100"`
	Phone        string `json:"phone" binding:"max=20"`
	Role         string `json:"role" binding:"required,oneof=rider driver"`
	VehicleType  string `json:"vehicle_type"`  // required if role=driver
	LicensePlate string `json:"license_plate"` // required if role=driver
}

// AuthResponse is returned after successful registration or login.
type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	User         *models.User `json:"user"`
}

// Register creates a new user and returns a JWT token.
// Uses a database transaction to ensure user and driver records are created atomically.
func (s *Service) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user already exists
	existing, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Internal("failed to check existing user", err)
	}
	if existing != nil {
		return nil, apperrors.Conflict("user with this email already exists")
	}

	// Validate driver fields
	if req.Role == "driver" {
		if req.VehicleType == "" || req.LicensePlate == "" {
			return nil, apperrors.BadRequest("vehicle_type and license_plate are required for drivers")
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperrors.Internal("failed to hash password", err)
	}

	// Create user + driver inside a transaction
	var user *models.User
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := s.rawRepo.WithTx(tx)

		var txErr error
		user, txErr = txRepo.CreateUser(ctx, req.Email, string(hash), req.Name, req.Phone, req.Role)
		if txErr != nil {
			return txErr
		}

		// Create driver profile if role is driver
		if req.Role == "driver" {
			_, txErr = txRepo.CreateDriver(ctx, user.ID, req.VehicleType, req.LicensePlate)
			if txErr != nil {
				return txErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, apperrors.Internal("failed to create user", err)
	}

	// Generate token pair
	pair, err := jwtutil.GenerateTokenPair(user.ID, user.Role, s.jwtSecret, s.jwtExp)
	if err != nil {
		return nil, apperrors.Internal("failed to generate tokens", err)
	}

	return &AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		User:         user,
	}, nil
}

// LoginRequest is the input for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user and returns a JWT token pair.
func (s *Service) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check lockout
	if s.lockout.IsLocked(req.Email) {
		return nil, apperrors.TooManyRequests("account temporarily locked due to too many failed attempts")
	}

	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, apperrors.Internal("failed to look up user", err)
	}
	if user == nil {
		s.lockout.RecordFailure(req.Email)
		return nil, apperrors.Unauthorized("invalid email or password")
	}

	// Compare password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		s.lockout.RecordFailure(req.Email)
		return nil, apperrors.Unauthorized("invalid email or password")
	}

	// Clear lockout on success
	s.lockout.RecordSuccess(req.Email)

	// Generate token pair
	pair, err := jwtutil.GenerateTokenPair(user.ID, user.Role, s.jwtSecret, s.jwtExp)
	if err != nil {
		return nil, apperrors.Internal("failed to generate tokens", err)
	}

	return &AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		User:         user,
	}, nil
}

// RefreshToken validates a refresh token and returns a new token pair.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	claims, err := jwtutil.ValidateRefreshToken(refreshToken, s.jwtSecret)
	if err != nil {
		return nil, apperrors.Unauthorized("invalid refresh token")
	}

	// Look up user to ensure they still exist
	user, err := s.repo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, apperrors.Internal("failed to look up user", err)
	}
	if user == nil {
		return nil, apperrors.Unauthorized("user no longer exists")
	}

	// Generate new token pair
	pair, err := jwtutil.GenerateTokenPair(user.ID, user.Role, s.jwtSecret, s.jwtExp)
	if err != nil {
		return nil, apperrors.Internal("failed to generate tokens", err)
	}

	return &AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresIn:    pair.ExpiresIn,
		User:         user,
	}, nil
}
