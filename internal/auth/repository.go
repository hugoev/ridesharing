// Package auth implements the authentication service.
package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
)

// RepositoryInterface defines the contract for auth data access.
type RepositoryInterface interface {
	CreateUser(ctx context.Context, email, passwordHash, name, phone, role string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	CreateDriver(ctx context.Context, userID uuid.UUID, vehicleType, licensePlate string) (*models.Driver, error)
}

// Repository handles database operations for the auth service.
type Repository struct {
	pool *pgxpool.Pool
	q    db.DBTX
}

// NewRepository creates a new auth repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: pool}
}

// WithTx returns a copy of the repository that runs queries inside tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, q: tx}
}

// CreateUser inserts a new user into the database and returns the created user.
func (r *Repository) CreateUser(ctx context.Context, email, passwordHash, name, phone, role string) (*models.User, error) {
	user := &models.User{}
	err := r.q.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name, phone, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, email, password_hash, name, phone, role, created_at, updated_at`,
		email, passwordHash, name, phone, role,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone,
		&user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

// GetByEmail retrieves a user by their email address.
func (r *Repository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := r.q.QueryRow(ctx,
		`SELECT id, email, password_hash, name, phone, role, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone,
		&user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

// GetByID retrieves a user by their UUID.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	err := r.q.QueryRow(ctx,
		`SELECT id, email, password_hash, name, phone, role, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Phone,
		&user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

// CreateDriver inserts a driver profile for an existing user.
func (r *Repository) CreateDriver(ctx context.Context, userID uuid.UUID, vehicleType, licensePlate string) (*models.Driver, error) {
	driver := &models.Driver{}
	err := r.q.QueryRow(ctx,
		`INSERT INTO drivers (user_id, vehicle_type, license_plate, is_available)
		 VALUES ($1, $2, $3, true)
		 RETURNING user_id, vehicle_type, license_plate, is_available, rating, created_at, updated_at`,
		userID, vehicleType, licensePlate,
	).Scan(&driver.UserID, &driver.VehicleType, &driver.LicensePlate,
		&driver.IsAvailable, &driver.Rating, &driver.CreatedAt, &driver.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("create driver: %w", err)
	}
	return driver, nil
}
