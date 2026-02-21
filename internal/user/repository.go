// Package user implements the user profile service.
package user

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
)

// RepositoryInterface defines the contract for user data access.
type RepositoryInterface interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithDriver, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, name, phone string) error
	UpdateDriver(ctx context.Context, userID uuid.UUID, vehicleType, licensePlate string) error
	SetAvailability(ctx context.Context, userID uuid.UUID, available bool) error
}

// Repository handles database operations for user profiles.
type Repository struct {
	pool *pgxpool.Pool
	q    db.DBTX
}

// NewRepository creates a new user repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: pool}
}

// WithTx returns a copy of the repository that runs queries inside tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, q: tx}
}

// GetByID retrieves a user with optional driver details by performing two queries.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithDriver, error) {
	result := &models.UserWithDriver{}

	// Fetch base user
	err := r.q.QueryRow(ctx,
		`SELECT id, email, name, phone, role, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&result.ID, &result.Email, &result.Name, &result.Phone,
		&result.Role, &result.CreatedAt, &result.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Fetch driver profile if user is a driver
	if result.Role == "driver" {
		driver := &models.Driver{}
		err := r.q.QueryRow(ctx,
			`SELECT user_id, vehicle_type, license_plate, is_available, rating, created_at, updated_at
			 FROM drivers WHERE user_id = $1`, id,
		).Scan(&driver.UserID, &driver.VehicleType, &driver.LicensePlate,
			&driver.IsAvailable, &driver.Rating, &driver.CreatedAt, &driver.UpdatedAt)

		if err != nil && err != pgx.ErrNoRows {
			return nil, fmt.Errorf("get driver: %w", err)
		}
		if err == nil {
			result.Driver = driver
		}
	}

	return result, nil
}

// UpdateProfile updates a user's profile fields.
func (r *Repository) UpdateProfile(ctx context.Context, id uuid.UUID, name, phone string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE users SET name = $2, phone = $3, updated_at = NOW() WHERE id = $1`,
		id, name, phone,
	)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

// UpdateDriver updates driver-specific fields.
func (r *Repository) UpdateDriver(ctx context.Context, userID uuid.UUID, vehicleType, licensePlate string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE drivers SET vehicle_type = $2, license_plate = $3, updated_at = NOW() WHERE user_id = $1`,
		userID, vehicleType, licensePlate,
	)
	if err != nil {
		return fmt.Errorf("update driver: %w", err)
	}
	return nil
}

// SetAvailability updates a driver's availability status.
func (r *Repository) SetAvailability(ctx context.Context, userID uuid.UUID, available bool) error {
	_, err := r.q.Exec(ctx,
		`UPDATE drivers SET is_available = $2, updated_at = NOW() WHERE user_id = $1`,
		userID, available,
	)
	if err != nil {
		return fmt.Errorf("set availability: %w", err)
	}
	return nil
}
