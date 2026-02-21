package ride

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
)

// RepositoryInterface defines the contract for ride data access.
type RepositoryInterface interface {
	Create(ctx context.Context, ride *models.Ride) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Ride, error)
	UpdateStatus(ctx context.Context, rideID uuid.UUID, status models.RideStatus, driverID *uuid.UUID) error
	SetStarted(ctx context.Context, rideID uuid.UUID) error
	SetCompleted(ctx context.Context, rideID uuid.UUID, fare float64) error
	CountActiveInArea(ctx context.Context, since time.Duration) (int, error)
	GetActiveByRider(ctx context.Context, riderID uuid.UUID) (*models.Ride, error)
	GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Ride, error)
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
}

// Repository handles database operations for rides.
type Repository struct {
	pool *pgxpool.Pool
	q    db.DBTX
}

// NewRepository creates a new ride repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: pool}
}

// WithTx returns a copy of the repository that runs queries inside tx.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return &Repository{pool: r.pool, q: tx}
}

// Create inserts a new ride request.
func (r *Repository) Create(ctx context.Context, ride *models.Ride) error {
	err := r.q.QueryRow(ctx,
		`INSERT INTO rides (rider_id, pickup_lat, pickup_long, dropoff_lat, dropoff_long,
		                     status, fare, surge_multiplier, estimated_eta, distance_km)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, created_at, updated_at`,
		ride.RiderID, ride.PickupLat, ride.PickupLong, ride.DropoffLat, ride.DropoffLong,
		ride.Status, ride.Fare, ride.SurgeMultiplier, ride.EstimatedETA, ride.DistanceKm,
	).Scan(&ride.ID, &ride.CreatedAt, &ride.UpdatedAt)

	if err != nil {
		return fmt.Errorf("create ride: %w", err)
	}
	return nil
}

// GetByID retrieves a ride by its UUID.
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*models.Ride, error) {
	ride := &models.Ride{}
	err := r.q.QueryRow(ctx,
		`SELECT id, rider_id, driver_id, pickup_lat, pickup_long, dropoff_lat, dropoff_long,
		        status, fare, surge_multiplier, estimated_eta, distance_km,
		        started_at, ended_at, created_at, updated_at
		 FROM rides WHERE id = $1`, id,
	).Scan(&ride.ID, &ride.RiderID, &ride.DriverID,
		&ride.PickupLat, &ride.PickupLong, &ride.DropoffLat, &ride.DropoffLong,
		&ride.Status, &ride.Fare, &ride.SurgeMultiplier,
		&ride.EstimatedETA, &ride.DistanceKm,
		&ride.StartedAt, &ride.EndedAt, &ride.CreatedAt, &ride.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get ride: %w", err)
	}
	return ride, nil
}

// GetByIDForUpdate retrieves a ride with a row-level lock for safe concurrent updates.
func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Ride, error) {
	ride := &models.Ride{}
	err := r.q.QueryRow(ctx,
		`SELECT id, rider_id, driver_id, pickup_lat, pickup_long, dropoff_lat, dropoff_long,
		        status, fare, surge_multiplier, estimated_eta, distance_km,
		        started_at, ended_at, created_at, updated_at
		 FROM rides WHERE id = $1 FOR UPDATE`, id,
	).Scan(&ride.ID, &ride.RiderID, &ride.DriverID,
		&ride.PickupLat, &ride.PickupLong, &ride.DropoffLat, &ride.DropoffLong,
		&ride.Status, &ride.Fare, &ride.SurgeMultiplier,
		&ride.EstimatedETA, &ride.DistanceKm,
		&ride.StartedAt, &ride.EndedAt, &ride.CreatedAt, &ride.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get ride for update: %w", err)
	}
	return ride, nil
}

// UpdateStatus transitions a ride's status and optionally sets the driver.
func (r *Repository) UpdateStatus(ctx context.Context, rideID uuid.UUID, status models.RideStatus, driverID *uuid.UUID) error {
	var err error
	if driverID != nil {
		_, err = r.q.Exec(ctx,
			`UPDATE rides SET status = $2, driver_id = $3, updated_at = NOW() WHERE id = $1`,
			rideID, status, *driverID,
		)
	} else {
		_, err = r.q.Exec(ctx,
			`UPDATE rides SET status = $2, updated_at = NOW() WHERE id = $1`,
			rideID, status,
		)
	}
	if err != nil {
		return fmt.Errorf("update ride status: %w", err)
	}
	return nil
}

// SetStarted marks a ride as started.
func (r *Repository) SetStarted(ctx context.Context, rideID uuid.UUID) error {
	_, err := r.q.Exec(ctx,
		`UPDATE rides SET status = 'enroute', started_at = NOW(), updated_at = NOW() WHERE id = $1`,
		rideID,
	)
	return err
}

// SetCompleted marks a ride as completed with final fare.
func (r *Repository) SetCompleted(ctx context.Context, rideID uuid.UUID, fare float64) error {
	_, err := r.q.Exec(ctx,
		`UPDATE rides SET status = 'completed', fare = $2, ended_at = NOW(), updated_at = NOW() WHERE id = $1`,
		rideID, fare,
	)
	return err
}

// CountActiveInArea counts rides with status 'requested' created in the last N minutes.
// Used for surge pricing calculations.
func (r *Repository) CountActiveInArea(ctx context.Context, since time.Duration) (int, error) {
	var count int
	err := r.q.QueryRow(ctx,
		`SELECT COUNT(*) FROM rides WHERE status = 'requested' AND created_at > $1`,
		time.Now().Add(-since),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active rides: %w", err)
	}
	return count, nil
}

// GetActiveByRider returns the current active ride for a rider (if any).
func (r *Repository) GetActiveByRider(ctx context.Context, riderID uuid.UUID) (*models.Ride, error) {
	ride := &models.Ride{}
	err := r.q.QueryRow(ctx,
		`SELECT id, rider_id, driver_id, pickup_lat, pickup_long, dropoff_lat, dropoff_long,
		        status, fare, surge_multiplier, estimated_eta, distance_km,
		        started_at, ended_at, created_at, updated_at
		 FROM rides
		 WHERE rider_id = $1 AND status IN ('requested', 'matched', 'enroute')
		 ORDER BY created_at DESC LIMIT 1`, riderID,
	).Scan(&ride.ID, &ride.RiderID, &ride.DriverID,
		&ride.PickupLat, &ride.PickupLong, &ride.DropoffLat, &ride.DropoffLong,
		&ride.Status, &ride.Fare, &ride.SurgeMultiplier,
		&ride.EstimatedETA, &ride.DistanceKm,
		&ride.StartedAt, &ride.EndedAt, &ride.CreatedAt, &ride.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active ride: %w", err)
	}
	return ride, nil
}

// GetHistory returns paginated ride history for a user (as rider or driver).
func (r *Repository) GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Ride, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, rider_id, driver_id, pickup_lat, pickup_long, dropoff_lat, dropoff_long,
		        status, fare, surge_multiplier, estimated_eta, distance_km,
		        started_at, ended_at, created_at, updated_at
		 FROM rides
		 WHERE rider_id = $1 OR driver_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get ride history: %w", err)
	}
	defer rows.Close()

	var rides []models.Ride
	for rows.Next() {
		var ride models.Ride
		if err := rows.Scan(&ride.ID, &ride.RiderID, &ride.DriverID,
			&ride.PickupLat, &ride.PickupLong, &ride.DropoffLat, &ride.DropoffLong,
			&ride.Status, &ride.Fare, &ride.SurgeMultiplier,
			&ride.EstimatedETA, &ride.DistanceKm,
			&ride.StartedAt, &ride.EndedAt, &ride.CreatedAt, &ride.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ride: %w", err)
		}
		rides = append(rides, ride)
	}
	return rides, nil
}

// CountByUser returns the total count of rides for a user.
func (r *Repository) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.q.QueryRow(ctx,
		`SELECT COUNT(*) FROM rides WHERE rider_id = $1 OR driver_id = $1`, userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count rides: %w", err)
	}
	return count, nil
}
