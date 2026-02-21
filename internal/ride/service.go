package ride

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/location"
	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	apperrors "github.com/hugovillarreal/ridesharing/pkg/errors"
	"github.com/hugovillarreal/ridesharing/pkg/geo"
	"github.com/hugovillarreal/ridesharing/pkg/kafka"
	redisclient "github.com/hugovillarreal/ridesharing/pkg/redis"
	"github.com/hugovillarreal/ridesharing/pkg/tracing"
)

// Service implements the ride dispatch business logic.
type Service struct {
	repo        RepositoryInterface
	rawRepo     *Repository // needed for WithTx
	pool        *pgxpool.Pool
	locationSvc location.ServiceInterface
	surge       *SurgeEngine
	producer    *kafka.Producer
	rdb         *redis.Client
}

// NewService creates a new ride service.
func NewService(repo *Repository, pool *pgxpool.Pool, locationSvc location.ServiceInterface, producer *kafka.Producer, rdb *redis.Client) *Service {
	return &Service{
		repo:        repo,
		rawRepo:     repo,
		pool:        pool,
		locationSvc: locationSvc,
		surge:       NewSurgeEngine(),
		producer:    producer,
		rdb:         rdb,
	}
}

// RideRequest is the input for requesting a ride.
type RideRequest struct {
	PickupLat   float64 `json:"pickup_lat" binding:"required,min=-90,max=90"`
	PickupLong  float64 `json:"pickup_long" binding:"required,min=-180,max=180"`
	DropoffLat  float64 `json:"dropoff_lat" binding:"required,min=-90,max=90"`
	DropoffLong float64 `json:"dropoff_long" binding:"required,min=-180,max=180"`
}

// RideResponse is the enriched response for a ride request.
type RideResponse struct {
	*models.Ride
	MatchedDriver *models.NearbyDriver `json:"matched_driver,omitempty"`
}

// RequestRide creates a new ride, calculates fare/ETA, and attempts automatic matching.
func (s *Service) RequestRide(ctx context.Context, riderID uuid.UUID, req RideRequest) (*RideResponse, error) {
	ctx, span := tracing.Tracer("ride").Start(ctx, "RequestRide")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if rider already has an active ride
	active, err := s.repo.GetActiveByRider(ctx, riderID)
	if err != nil {
		return nil, apperrors.Internal("failed to check active rides", err)
	}
	if active != nil {
		return nil, apperrors.Conflict("you already have an active ride")
	}

	// Calculate distance
	distKm := geo.Haversine(req.PickupLat, req.PickupLong, req.DropoffLat, req.DropoffLong)

	// Calculate surge
	activeCount, _ := s.repo.CountActiveInArea(ctx, 10*time.Minute)
	driverCount, _ := s.locationSvc.ActiveDriverCount(ctx, req.PickupLat, req.PickupLong, 10)
	surgeMultiplier := s.surge.Calculate(activeCount, driverCount)

	// Calculate fare and ETA
	fare := geo.EstimateFare(distKm, surgeMultiplier)
	eta := geo.EstimateETA(distKm)
	etaSeconds := int(eta.Seconds())

	// Create ride record
	ride := &models.Ride{
		RiderID:         riderID,
		PickupLat:       req.PickupLat,
		PickupLong:      req.PickupLong,
		DropoffLat:      req.DropoffLat,
		DropoffLong:     req.DropoffLong,
		Status:          models.RideStatusRequested,
		Fare:            fare,
		SurgeMultiplier: surgeMultiplier,
		EstimatedETA:    &etaSeconds,
		DistanceKm:      &distKm,
	}

	if err := s.repo.Create(ctx, ride); err != nil {
		return nil, apperrors.Internal("failed to create ride", err)
	}

	response := &RideResponse{Ride: ride}

	// Attempt automatic matching with distributed lock
	matchedDriver, err := s.matchDriver(ctx, ride)
	if err != nil {
		log.Warn().Err(err).Str("ride_id", ride.ID.String()).Msg("automatic matching failed")
		// Ride is still created as 'requested' — matching can be retried
	} else if matchedDriver != nil {
		response.MatchedDriver = matchedDriver
	}

	// Publish ride event
	s.publishEvent("ride.requested", ride)

	return response, nil
}

// matchDriver finds the nearest available driver and assigns them to the ride.
// Uses a Redis distributed lock to prevent double-matching across multiple instances.
func (s *Service) matchDriver(ctx context.Context, ride *models.Ride) (*models.NearbyDriver, error) {
	ctx, span := tracing.Tracer("ride").Start(ctx, "matchDriver")
	defer span.End()

	// Acquire distributed lock for matching (requires Redis)
	if s.rdb == nil {
		return nil, fmt.Errorf("redis not available for distributed locking")
	}
	lock := redisclient.NewDistLock(s.rdb, "ride:match", 5*time.Second)
	acquired, err := lock.AcquireWithRetry(ctx, 3, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("acquire matching lock: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("could not acquire matching lock")
	}
	defer lock.Release(ctx)

	// Find nearby drivers within 10km
	drivers, err := s.locationSvc.GetNearbyDrivers(ctx, ride.PickupLat, ride.PickupLong, 10)
	if err != nil {
		return nil, fmt.Errorf("find nearby drivers: %w", err)
	}

	if len(drivers) == 0 {
		log.Info().Str("ride_id", ride.ID.String()).Msg("no nearby drivers found")
		return nil, nil
	}

	// Take the closest driver
	nearest := drivers[0]

	// Update ride status to matched and assign driver inside a transaction
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := s.rawRepo.WithTx(tx)
		return txRepo.UpdateStatus(ctx, ride.ID, models.RideStatusMatched, &nearest.DriverID)
	})
	if err != nil {
		return nil, fmt.Errorf("update ride status: %w", err)
	}

	ride.Status = models.RideStatusMatched
	ride.DriverID = &nearest.DriverID

	// Publish match event
	s.publishEvent("ride.matched", ride)

	log.Info().
		Str("ride_id", ride.ID.String()).
		Str("driver_id", nearest.DriverID.String()).
		Float64("distance_km", nearest.DistanceKm).
		Msg("ride matched with driver")

	return &nearest, nil
}

// AcceptRide allows a driver to accept a matched ride.
// Uses SELECT FOR UPDATE to prevent concurrent state changes.
func (s *Service) AcceptRide(ctx context.Context, rideID, driverID uuid.UUID) (*models.Ride, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var ride *models.Ride
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := s.rawRepo.WithTx(tx)

		var txErr error
		ride, txErr = txRepo.GetByIDForUpdate(ctx, rideID)
		if txErr != nil {
			return txErr
		}
		if ride == nil {
			return apperrors.NotFound("ride not found")
		}

		// Verify the driver is assigned to this ride
		if ride.DriverID == nil || *ride.DriverID != driverID {
			return apperrors.Forbidden("you are not assigned to this ride")
		}

		// Transition: matched → enroute
		if err := ValidateTransition(ride.Status, models.RideStatusEnroute); err != nil {
			return apperrors.BadRequest(err.Error())
		}

		if err := txRepo.SetStarted(ctx, rideID); err != nil {
			return err
		}

		ride.Status = models.RideStatusEnroute
		return nil
	})
	if err != nil {
		if _, ok := err.(*apperrors.AppError); ok {
			return nil, err
		}
		return nil, apperrors.Internal("failed to accept ride", err)
	}

	s.publishEvent("ride.started", ride)
	return ride, nil
}

// CompleteRide marks a ride as completed and triggers payment.
// Uses SELECT FOR UPDATE to prevent concurrent state changes.
func (s *Service) CompleteRide(ctx context.Context, rideID, driverID uuid.UUID) (*models.Ride, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var ride *models.Ride
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := s.rawRepo.WithTx(tx)

		var txErr error
		ride, txErr = txRepo.GetByIDForUpdate(ctx, rideID)
		if txErr != nil {
			return txErr
		}
		if ride == nil {
			return apperrors.NotFound("ride not found")
		}

		if ride.DriverID == nil || *ride.DriverID != driverID {
			return apperrors.Forbidden("you are not assigned to this ride")
		}

		if err := ValidateTransition(ride.Status, models.RideStatusCompleted); err != nil {
			return apperrors.BadRequest(err.Error())
		}

		if err := txRepo.SetCompleted(ctx, rideID, ride.Fare); err != nil {
			return err
		}

		ride.Status = models.RideStatusCompleted
		return nil
	})
	if err != nil {
		if _, ok := err.(*apperrors.AppError); ok {
			return nil, err
		}
		return nil, apperrors.Internal("failed to complete ride", err)
	}

	s.publishEvent("ride.completed", ride)
	return ride, nil
}

// CancelRide cancels a ride (by rider or driver).
// Uses SELECT FOR UPDATE to prevent concurrent state changes.
func (s *Service) CancelRide(ctx context.Context, rideID, userID uuid.UUID) (*models.Ride, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var ride *models.Ride
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := s.rawRepo.WithTx(tx)

		var txErr error
		ride, txErr = txRepo.GetByIDForUpdate(ctx, rideID)
		if txErr != nil {
			return txErr
		}
		if ride == nil {
			return apperrors.NotFound("ride not found")
		}

		// Verify the user is the rider or the assigned driver
		isRider := ride.RiderID == userID
		isDriver := ride.DriverID != nil && *ride.DriverID == userID
		if !isRider && !isDriver {
			return apperrors.Forbidden("you are not part of this ride")
		}

		if err := ValidateTransition(ride.Status, models.RideStatusCancelled); err != nil {
			return apperrors.BadRequest(err.Error())
		}

		if err := txRepo.UpdateStatus(ctx, rideID, models.RideStatusCancelled, nil); err != nil {
			return err
		}

		ride.Status = models.RideStatusCancelled
		return nil
	})
	if err != nil {
		if _, ok := err.(*apperrors.AppError); ok {
			return nil, err
		}
		return nil, apperrors.Internal("failed to cancel ride", err)
	}

	s.publishEvent("ride.cancelled", ride)
	return ride, nil
}

// GetRide retrieves ride details by ID.
func (s *Service) GetRide(ctx context.Context, rideID uuid.UUID) (*models.Ride, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ride, err := s.repo.GetByID(ctx, rideID)
	if err != nil {
		return nil, apperrors.Internal("failed to get ride", err)
	}
	if ride == nil {
		return nil, apperrors.NotFound("ride not found")
	}
	return ride, nil
}

// GetRideHistory returns paginated ride history for a user.
func (s *Service) GetRideHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Ride, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rides, err := s.repo.GetHistory(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to get ride history", err)
	}

	total, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to count rides", err)
	}

	return rides, total, nil
}

// publishEvent sends a ride event to Kafka asynchronously.
func (s *Service) publishEvent(eventType string, ride *models.Ride) {
	if s.producer == nil {
		return
	}

	data, err := json.Marshal(map[string]interface{}{
		"event":     eventType,
		"ride":      ride,
		"timestamp": time.Now().UTC(),
	})
	if err != nil {
		log.Error().Err(err).Str("event", eventType).Msg("failed to marshal ride event")
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.producer.Publish(ctx, []byte(ride.ID.String()), data); err != nil {
			log.Error().Err(err).Str("event", eventType).Msg("failed to publish ride event")
		}
	}()
}
