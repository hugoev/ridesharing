package location

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/hugovillarreal/ridesharing/internal/models"
)

// --- Mock Location Repository ---

type mockLocRepo struct {
	insertErr error
	drivers   []models.NearbyDriver
}

func (m *mockLocRepo) Insert(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	return m.insertErr
}

func (m *mockLocRepo) FindNearby(ctx context.Context, lat, long, radiusMeters float64) ([]models.NearbyDriver, error) {
	return m.drivers, nil
}

// --- Mock Location Cache ---

type mockLocCache struct {
	drivers   []models.NearbyDriver
	updateErr error
	nearbyErr error
}

func (m *mockLocCache) UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	return m.updateErr
}

func (m *mockLocCache) RemoveDriver(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (m *mockLocCache) NearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error) {
	if m.nearbyErr != nil {
		return nil, m.nearbyErr
	}
	return m.drivers, nil
}

// --- Tests ---

func TestUpdateLocation_Success(t *testing.T) {
	repo := &mockLocRepo{}
	cache := &mockLocCache{}
	svc := NewService(repo, cache)

	err := svc.UpdateLocation(context.Background(), uuid.New(), 40.71, -74.00)
	if err != nil {
		t.Fatalf("UpdateLocation failed: %v", err)
	}
}

func TestUpdateLocation_CacheFailStillPersists(t *testing.T) {
	repo := &mockLocRepo{}
	cache := &mockLocCache{updateErr: errors.New("redis down")}
	svc := NewService(repo, cache)

	// Should not fail — cache errors are non-fatal
	err := svc.UpdateLocation(context.Background(), uuid.New(), 40.71, -74.00)
	if err != nil {
		t.Fatalf("expected success even with cache failure, got: %v", err)
	}
}

func TestUpdateLocation_RepoFailReturnsError(t *testing.T) {
	repo := &mockLocRepo{insertErr: errors.New("db error")}
	cache := &mockLocCache{}
	svc := NewService(repo, cache)

	err := svc.UpdateLocation(context.Background(), uuid.New(), 40.71, -74.00)
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

func TestGetNearbyDrivers_FromCache(t *testing.T) {
	driverID := uuid.New()
	cache := &mockLocCache{
		drivers: []models.NearbyDriver{
			{DriverID: driverID, Lat: 40.71, Long: -74.00, DistanceKm: 1.5, EstimatedETA: 180},
		},
	}
	repo := &mockLocRepo{} // should not be called
	svc := NewService(repo, cache)

	drivers, err := svc.GetNearbyDrivers(context.Background(), 40.71, -74.00, 5)
	if err != nil {
		t.Fatalf("GetNearbyDrivers failed: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver, got %d", len(drivers))
	}
	if drivers[0].DriverID != driverID {
		t.Error("unexpected driver ID")
	}
}

func TestGetNearbyDrivers_FallbackToPostGIS(t *testing.T) {
	driverID := uuid.New()
	cache := &mockLocCache{nearbyErr: errors.New("redis down")} // cache fails
	repo := &mockLocRepo{
		drivers: []models.NearbyDriver{
			{DriverID: driverID, DistanceKm: 2.0},
		},
	}
	svc := NewService(repo, cache)

	drivers, err := svc.GetNearbyDrivers(context.Background(), 40.71, -74.00, 5)
	if err != nil {
		t.Fatalf("GetNearbyDrivers fallback failed: %v", err)
	}
	if len(drivers) != 1 {
		t.Fatalf("expected 1 driver from fallback, got %d", len(drivers))
	}
}

func TestGetNearbyDrivers_NoDrivers(t *testing.T) {
	cache := &mockLocCache{drivers: nil}
	repo := &mockLocRepo{drivers: nil}
	svc := NewService(repo, cache)

	drivers, err := svc.GetNearbyDrivers(context.Background(), 40.71, -74.00, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(drivers) != 0 {
		t.Errorf("expected 0 drivers, got %d", len(drivers))
	}
}

func TestActiveDriverCount(t *testing.T) {
	cache := &mockLocCache{
		drivers: []models.NearbyDriver{
			{DriverID: uuid.New()},
			{DriverID: uuid.New()},
			{DriverID: uuid.New()},
		},
	}
	svc := NewService(&mockLocRepo{}, cache)

	count, err := svc.ActiveDriverCount(context.Background(), 40.71, -74.00, 10)
	if err != nil {
		t.Fatalf("ActiveDriverCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestParseRadius_Valid(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"5", 5.0},
		{"0.5", 0.5},
		{"100", 100.0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRadius(tt.input)
			if err != nil {
				t.Fatalf("ParseRadius(%q) error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseRadius(%q) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRadius_Invalid(t *testing.T) {
	invalids := []string{"", "abc", "0", "-5", "101", "999"}
	for _, s := range invalids {
		t.Run(s, func(t *testing.T) {
			_, err := ParseRadius(s)
			if err == nil {
				t.Errorf("ParseRadius(%q) expected error", s)
			}
		})
	}
}
