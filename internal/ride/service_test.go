package ride

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/hugovillarreal/ridesharing/internal/models"
)

// --- Mock Ride Repository ---

type mockRideRepo struct {
	rides         map[uuid.UUID]*models.Ride
	activeByRider map[uuid.UUID]*models.Ride
	history       []models.Ride
	activeCount   int
	createErr     error
	updateErr     error
}

func newMockRideRepo() *mockRideRepo {
	return &mockRideRepo{
		rides:         make(map[uuid.UUID]*models.Ride),
		activeByRider: make(map[uuid.UUID]*models.Ride),
	}
}

func (m *mockRideRepo) Create(ctx context.Context, ride *models.Ride) error {
	if m.createErr != nil {
		return m.createErr
	}
	ride.ID = uuid.New()
	ride.CreatedAt = time.Now()
	ride.UpdatedAt = time.Now()
	m.rides[ride.ID] = ride
	return nil
}

func (m *mockRideRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Ride, error) {
	ride, ok := m.rides[id]
	if !ok {
		return nil, nil
	}
	return ride, nil
}

func (m *mockRideRepo) UpdateStatus(ctx context.Context, rideID uuid.UUID, status models.RideStatus, driverID *uuid.UUID) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if ride, ok := m.rides[rideID]; ok {
		ride.Status = status
		if driverID != nil {
			ride.DriverID = driverID
		}
	}
	return nil
}

func (m *mockRideRepo) SetStarted(ctx context.Context, rideID uuid.UUID) error {
	if ride, ok := m.rides[rideID]; ok {
		ride.Status = models.RideStatusEnroute
		now := time.Now()
		ride.StartedAt = &now
	}
	return nil
}

func (m *mockRideRepo) SetCompleted(ctx context.Context, rideID uuid.UUID, fare float64) error {
	if ride, ok := m.rides[rideID]; ok {
		ride.Status = models.RideStatusCompleted
		ride.Fare = fare
		now := time.Now()
		ride.EndedAt = &now
	}
	return nil
}

func (m *mockRideRepo) CountActiveInArea(ctx context.Context, since time.Duration) (int, error) {
	return m.activeCount, nil
}

func (m *mockRideRepo) GetActiveByRider(ctx context.Context, riderID uuid.UUID) (*models.Ride, error) {
	ride, ok := m.activeByRider[riderID]
	if !ok {
		return nil, nil
	}
	return ride, nil
}

func (m *mockRideRepo) GetHistory(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Ride, error) {
	if offset >= len(m.history) {
		return nil, nil
	}
	end := offset + limit
	if end > len(m.history) {
		end = len(m.history)
	}
	return m.history[offset:end], nil
}

func (m *mockRideRepo) CountByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	return len(m.history), nil
}

// --- Mock Location Service ---

type mockLocationSvc struct {
	drivers     []models.NearbyDriver
	driverCount int
}

func (m *mockLocationSvc) UpdateLocation(ctx context.Context, userID uuid.UUID, lat, long float64) error {
	return nil
}

func (m *mockLocationSvc) GetNearbyDrivers(ctx context.Context, lat, long, radiusKm float64) ([]models.NearbyDriver, error) {
	return m.drivers, nil
}

func (m *mockLocationSvc) ActiveDriverCount(ctx context.Context, lat, long, radiusKm float64) (int, error) {
	return m.driverCount, nil
}

// --- Tests ---

func TestRequestRide_Success(t *testing.T) {
	repo := newMockRideRepo()
	locSvc := &mockLocationSvc{driverCount: 5}

	svc := &Service{
		repo:        repo,
		locationSvc: locSvc,
		surge:       NewSurgeEngine(),
	}

	riderID := uuid.New()
	req := RideRequest{
		PickupLat:   40.7128,
		PickupLong:  -74.0060,
		DropoffLat:  40.7580,
		DropoffLong: -73.9855,
	}

	resp, err := svc.RequestRide(context.Background(), riderID, req)
	if err != nil {
		t.Fatalf("RequestRide failed: %v", err)
	}

	if resp.Ride.RiderID != riderID {
		t.Errorf("expected rider ID %s, got %s", riderID, resp.Ride.RiderID)
	}
	if resp.Ride.Status != models.RideStatusRequested {
		t.Errorf("expected status requested, got %s", resp.Ride.Status)
	}
	if resp.Ride.Fare <= 0 {
		t.Error("expected positive fare")
	}
	if resp.Ride.SurgeMultiplier < 1.0 {
		t.Error("expected surge >= 1.0")
	}
}

func TestRequestRide_ActiveRideConflict(t *testing.T) {
	repo := newMockRideRepo()
	riderID := uuid.New()

	// Pre-set an active ride
	repo.activeByRider[riderID] = &models.Ride{
		ID:      uuid.New(),
		RiderID: riderID,
		Status:  models.RideStatusRequested,
	}

	svc := &Service{
		repo:        repo,
		locationSvc: &mockLocationSvc{},
		surge:       NewSurgeEngine(),
	}

	_, err := svc.RequestRide(context.Background(), riderID, RideRequest{
		PickupLat: 40.71, PickupLong: -74.00,
		DropoffLat: 40.75, DropoffLong: -73.98,
	})
	if err == nil {
		t.Fatal("expected error for active ride conflict")
	}
}

func TestRequestRide_CreateError(t *testing.T) {
	repo := newMockRideRepo()
	repo.createErr = errTestDB

	svc := &Service{
		repo:        repo,
		locationSvc: &mockLocationSvc{driverCount: 5},
		surge:       NewSurgeEngine(),
	}

	_, err := svc.RequestRide(context.Background(), uuid.New(), RideRequest{
		PickupLat: 40.71, PickupLong: -74.00,
		DropoffLat: 40.75, DropoffLong: -73.98,
	})
	if err == nil {
		t.Fatal("expected error when repo fails")
	}
}

func TestGetRide_Found(t *testing.T) {
	rideID := uuid.New()
	repo := newMockRideRepo()
	repo.rides[rideID] = &models.Ride{
		ID:     rideID,
		Status: models.RideStatusRequested,
		Fare:   20.0,
	}

	svc := &Service{repo: repo}

	ride, err := svc.GetRide(context.Background(), rideID)
	if err != nil {
		t.Fatalf("GetRide failed: %v", err)
	}
	if ride.Fare != 20.0 {
		t.Errorf("expected fare 20.0, got %f", ride.Fare)
	}
}

func TestGetRide_NotFound(t *testing.T) {
	svc := &Service{repo: newMockRideRepo()}

	_, err := svc.GetRide(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected error for missing ride")
	}
}

func TestGetRideHistory_Pagination(t *testing.T) {
	repo := newMockRideRepo()
	riderID := uuid.New()

	// Create 5 historical rides
	for i := 0; i < 5; i++ {
		repo.history = append(repo.history, models.Ride{
			ID:      uuid.New(),
			RiderID: riderID,
			Status:  models.RideStatusCompleted,
			Fare:    float64(10 + i),
		})
	}

	svc := &Service{repo: repo}

	// Page 1: limit 2
	rides, total, err := svc.GetRideHistory(context.Background(), riderID, 2, 0)
	if err != nil {
		t.Fatalf("GetRideHistory failed: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(rides) != 2 {
		t.Errorf("expected 2 rides on page 1, got %d", len(rides))
	}

	// Page 3: limit 2, offset 4 (only 1 result)
	rides, _, err = svc.GetRideHistory(context.Background(), riderID, 2, 4)
	if err != nil {
		t.Fatalf("GetRideHistory page 3 failed: %v", err)
	}
	if len(rides) != 1 {
		t.Errorf("expected 1 ride on page 3, got %d", len(rides))
	}
}

func TestFSM_ValidTransitions(t *testing.T) {
	tests := []struct {
		from, to models.RideStatus
		valid    bool
	}{
		{models.RideStatusRequested, models.RideStatusMatched, true},
		{models.RideStatusRequested, models.RideStatusCancelled, true},
		{models.RideStatusRequested, models.RideStatusCompleted, false},
		{models.RideStatusMatched, models.RideStatusEnroute, true},
		{models.RideStatusMatched, models.RideStatusCancelled, true},
		{models.RideStatusMatched, models.RideStatusRequested, false},
		{models.RideStatusEnroute, models.RideStatusCompleted, true},
		{models.RideStatusEnroute, models.RideStatusCancelled, true},
		{models.RideStatusCompleted, models.RideStatusCancelled, false},
		{models.RideStatusCancelled, models.RideStatusRequested, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			got := CanTransition(tt.from, tt.to)
			if got != tt.valid {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.valid)
			}
		})
	}
}

// sentinel error for tests
var errTestDB = context.DeadlineExceeded
