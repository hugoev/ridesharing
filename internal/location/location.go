// Package location implements the location tracking service.
package location

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/hugovillarreal/ridesharing/internal/models"
	"github.com/hugovillarreal/ridesharing/pkg/db"
	"github.com/hugovillarreal/ridesharing/pkg/geo"
)

const (
	// RedisGeoKey is the key used for the Redis geo set of driver locations.
	RedisGeoKey = "drivers:locations"
)

// RepositoryInterface defines the contract for location data access.
type RepositoryInterface interface {
	Insert(ctx context.Context, userID uuid.UUID, lat, long float64) error
	FindNearby(ctx context.Context, lat, long, radiusMeters float64) ([]models.NearbyDriver, error)
}

// CacheInterface defines the contract for location cache operations.
type CacheInterface interface {
	UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error
	RemoveDriver(ctx context.Context, userID uuid.UUID) error
	NearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error)
}

// Repository handles location data persistence.
type Repository struct {
	pool *pgxpool.Pool
	q    db.DBTX
}

// NewRepository creates a new location repository.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: pool}
}

// Insert stores a location record in PostgreSQL.
func (r *Repository) Insert(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO locations (user_id, lat, long, geom, timestamp)
		 VALUES ($1, $2, $3, ST_SetSRID(ST_MakePoint($3, $2), 4326), NOW())`,
		userID, lat, long,
	)
	if err != nil {
		return fmt.Errorf("insert location: %w", err)
	}
	return nil
}

// FindNearby queries PostgreSQL for nearby drivers using PostGIS.
func (r *Repository) FindNearby(ctx context.Context, lat, long, radiusMeters float64) ([]models.NearbyDriver, error) {
	rows, err := r.q.Query(ctx,
		`SELECT driver_id, driver_lat, driver_long, distance_meters
		 FROM find_nearby_drivers($1, $2, $3)
		 ORDER BY distance_meters ASC`,
		lat, long, radiusMeters,
	)
	if err != nil {
		return nil, fmt.Errorf("find nearby drivers: %w", err)
	}
	defer rows.Close()

	var drivers []models.NearbyDriver
	for rows.Next() {
		var d models.NearbyDriver
		var distMeters float64
		if err := rows.Scan(&d.DriverID, &d.Lat, &d.Long, &distMeters); err != nil {
			return nil, fmt.Errorf("scan nearby driver: %w", err)
		}
		d.DistanceKm = distMeters / 1000.0
		eta := geo.EstimateETA(d.DistanceKm)
		d.EstimatedETA = int(eta.Seconds())
		drivers = append(drivers, d)
	}
	return drivers, nil
}

// Cache manages driver locations in Redis for fast geospatial queries.
type Cache struct {
	client *redis.Client
}

// NewCache creates a new location cache.
func NewCache(client *redis.Client) *Cache {
	return &Cache{client: client}
}

// UpdateLocation sets a driver's position in the Redis geo set.
func (c *Cache) UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	return c.client.GeoAdd(ctx, RedisGeoKey, &redis.GeoLocation{
		Name:      userID.String(),
		Longitude: long,
		Latitude:  lat,
	}).Err()
}

// RemoveDriver removes a driver from the geo set (e.g., when going offline).
func (c *Cache) RemoveDriver(ctx context.Context, userID uuid.UUID) error {
	return c.client.ZRem(ctx, RedisGeoKey, userID.String()).Err()
}

// NearbyDrivers finds drivers within a radius using Redis GEOSEARCH.
func (c *Cache) NearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error) {
	results, err := c.client.GeoSearchLocation(ctx, RedisGeoKey, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  long,
			Latitude:   lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      20,
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("redis geosearch: %w", err)
	}

	drivers := make([]models.NearbyDriver, 0, len(results))
	for _, r := range results {
		driverID, err := uuid.Parse(r.Name)
		if err != nil {
			log.Warn().Str("name", r.Name).Msg("invalid UUID in geo set, skipping")
			continue
		}
		distKm := r.Dist
		eta := geo.EstimateETA(distKm)
		drivers = append(drivers, models.NearbyDriver{
			DriverID:     driverID,
			Lat:          r.Latitude,
			Long:         r.Longitude,
			DistanceKm:   distKm,
			EstimatedETA: int(eta.Seconds()),
		})
	}
	return drivers, nil
}

// ServiceInterface defines the contract for location service operations.
type ServiceInterface interface {
	UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error
	GetNearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error)
	ActiveDriverCount(ctx context.Context, lat, long, radiusKm float64) (int, error)
}

// Service ties together the cache and repository for location operations.
type Service struct {
	repo  RepositoryInterface
	cache CacheInterface
}

// NewService creates a new location service.
func NewService(repo RepositoryInterface, cache CacheInterface) *Service {
	return &Service{repo: repo, cache: cache}
}

// UpdateLocation updates a driver's position in both Redis and PostgreSQL.
func (s *Service) UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	// Update Redis (fast path)
	if err := s.cache.UpdateLocation(ctx, userID, lat, long); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("failed to update cache location")
		// Don't fail — still persist to DB
	}

	// Persist to PostgreSQL (durable)
	if err := s.repo.Insert(ctx, userID, lat, long); err != nil {
		return fmt.Errorf("persist location: %w", err)
	}
	return nil
}

// GetNearbyDrivers finds available drivers near a point.
// Tries Redis first, falls back to PostGIS.
func (s *Service) GetNearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error) {
	// Try Redis cache first
	drivers, err := s.cache.NearbyDrivers(ctx, lat, long, radiusKm)
	if err == nil && len(drivers) > 0 {
		return drivers, nil
	}
	if err != nil {
		log.Warn().Err(err).Msg("redis geosearch failed, falling back to PostGIS")
	}

	// Fallback to PostGIS
	radiusMeters := radiusKm * 1000
	drivers, err = s.repo.FindNearby(ctx, lat, long, radiusMeters)
	if err != nil {
		return nil, fmt.Errorf("find nearby drivers: %w", err)
	}
	return drivers, nil
}

// ActiveDriverCount counts the number of drivers in the Redis geo set
// within a given radius (used for surge pricing calculation).
func (s *Service) ActiveDriverCount(ctx context.Context, lat, long, radiusKm float64) (int, error) {
	drivers, err := s.cache.NearbyDrivers(ctx, lat, long, radiusKm)
	if err != nil {
		return 0, err
	}
	return len(drivers), nil
}

// ParseRadius converts a radius string (e.g., "5") to float64 in km.
func ParseRadius(s string) (float64, error) {
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid radius: %w", err)
	}
	if val <= 0 || val > 100 {
		return 0, fmt.Errorf("radius must be between 0 and 100 km")
	}
	return val, nil
}
