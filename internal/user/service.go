package user

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/hugovillarreal/ridesharing/internal/models"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
)

// Service implements the user profile business logic.
type Service struct {
	repo RepositoryInterface
}

// NewService creates a new user service.
func NewService(repo RepositoryInterface) *Service {
	return &Service{repo: repo}
}

// GetProfile retrieves a user profile by ID.
func (s *Service) GetProfile(ctx context.Context, id uuid.UUID) (*models.UserWithDriver, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Internal("failed to get user profile", err)
	}
	if user == nil {
		return nil, apperrors.NotFound("user not found")
	}
	return user, nil
}

// UpdateProfileRequest is the input for updating a user profile.
type UpdateProfileRequest struct {
	Name         string `json:"name" binding:"max=100"`
	Phone        string `json:"phone" binding:"max=20"`
	VehicleType  string `json:"vehicle_type" binding:"max=50"`
	LicensePlate string `json:"license_plate" binding:"max=20"`
}

// UpdateProfile updates a user's profile information.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, req UpdateProfileRequest) (*models.UserWithDriver, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Verify user exists
	existing, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, apperrors.Internal("failed to get user", err)
	}
	if existing == nil {
		return nil, apperrors.NotFound("user not found")
	}

	// Update base profile
	if req.Name != "" || req.Phone != "" {
		name := req.Name
		if name == "" {
			name = existing.Name
		}
		phone := req.Phone
		if phone == "" {
			phone = existing.Phone
		}
		if err := s.repo.UpdateProfile(ctx, userID, name, phone); err != nil {
			return nil, apperrors.Internal("failed to update profile", err)
		}
	}

	// Update driver-specific fields
	if existing.Role == "driver" && (req.VehicleType != "" || req.LicensePlate != "") {
		vt := req.VehicleType
		if vt == "" && existing.Driver != nil {
			vt = existing.Driver.VehicleType
		}
		lp := req.LicensePlate
		if lp == "" && existing.Driver != nil {
			lp = existing.Driver.LicensePlate
		}
		if err := s.repo.UpdateDriver(ctx, userID, vt, lp); err != nil {
			return nil, apperrors.Internal("failed to update driver profile", err)
		}
	}

	// Return updated profile
	return s.repo.GetByID(ctx, userID)
}

// SetAvailability toggles a driver's availability.
func (s *Service) SetAvailability(ctx context.Context, userID uuid.UUID, available bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return apperrors.Internal("failed to get user", err)
	}
	if user == nil {
		return apperrors.NotFound("user not found")
	}
	if user.Role != "driver" {
		return apperrors.BadRequest("only drivers can set availability")
	}

	if err := s.repo.SetAvailability(ctx, userID, available); err != nil {
		return apperrors.Internal("failed to set availability", err)
	}
	return nil
}
