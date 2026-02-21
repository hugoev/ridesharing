package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUser_JSONOmitsPasswordHash(t *testing.T) {
	user := User{
		ID:           uuid.New(),
		Email:        "test@example.com",
		PasswordHash: "secret_hash_should_not_appear",
		Name:         "Test User",
		Phone:        "1234567890",
		Role:         "rider",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal user: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "secret_hash") {
		t.Error("password hash was exposed in JSON output")
	}
	if !contains(jsonStr, "test@example.com") {
		t.Error("expected email in JSON output")
	}
}

func TestRideStatus_Constants(t *testing.T) {
	statuses := []RideStatus{
		RideStatusRequested,
		RideStatusMatched,
		RideStatusEnroute,
		RideStatusCompleted,
		RideStatusCancelled,
	}

	// Verify all statuses are unique
	seen := make(map[RideStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}

	if len(statuses) != 5 {
		t.Errorf("expected 5 ride statuses, got %d", len(statuses))
	}
}

func TestRide_JSONDriverIDOmitEmpty(t *testing.T) {
	ride := Ride{
		ID:       uuid.New(),
		RiderID:  uuid.New(),
		DriverID: nil, // no driver assigned
		Status:   RideStatusRequested,
	}

	data, err := json.Marshal(ride)
	if err != nil {
		t.Fatalf("failed to marshal ride: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "driver_id") {
		t.Error("expected driver_id to be omitted when nil")
	}
}

func TestRide_JSONDriverIDPresent(t *testing.T) {
	driverID := uuid.New()
	ride := Ride{
		ID:       uuid.New(),
		RiderID:  uuid.New(),
		DriverID: &driverID,
		Status:   RideStatusMatched,
	}

	data, err := json.Marshal(ride)
	if err != nil {
		t.Fatalf("failed to marshal ride: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, "driver_id") {
		t.Error("expected driver_id in JSON when set")
	}
}

func TestUserWithDriver_EmbeddedFields(t *testing.T) {
	uwd := UserWithDriver{
		User: User{
			ID:    uuid.New(),
			Email: "driver@test.com",
			Name:  "Driver",
			Role:  "driver",
		},
		Driver: &Driver{
			VehicleType:  "sedan",
			LicensePlate: "ABC-123",
			IsAvailable:  true,
			Rating:       4.8,
		},
	}

	data, err := json.Marshal(uwd)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, "sedan") {
		t.Error("expected vehicle_type in JSON")
	}
	if !contains(jsonStr, "ABC-123") {
		t.Error("expected license_plate in JSON")
	}
}

func TestUserWithDriver_NoDriver(t *testing.T) {
	uwd := UserWithDriver{
		User: User{
			ID:    uuid.New(),
			Email: "rider@test.com",
			Role:  "rider",
		},
		Driver: nil,
	}

	data, err := json.Marshal(uwd)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "vehicle_type") {
		t.Error("expected driver fields to be omitted for rider")
	}
}

func TestPayment_JSON(t *testing.T) {
	p := Payment{
		ID:          uuid.New(),
		RideID:      uuid.New(),
		Amount:      25.50,
		Currency:    "USD",
		Status:      "completed",
		StripeTxnID: "pi_test123",
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal payment: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed["currency"] != "USD" {
		t.Errorf("expected currency USD, got %v", parsed["currency"])
	}
	if parsed["amount"] != 25.50 {
		t.Errorf("expected amount 25.50, got %v", parsed["amount"])
	}
}

func TestNearbyDriver_JSON(t *testing.T) {
	nd := NearbyDriver{
		DriverID:     uuid.New(),
		Lat:          40.71,
		Long:         -74.00,
		DistanceKm:   2.5,
		EstimatedETA: 300,
	}

	data, err := json.Marshal(nd)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if !contains(jsonStr, "distance_km") {
		t.Error("expected distance_km in JSON")
	}
	if !contains(jsonStr, "estimated_eta") {
		t.Error("expected estimated_eta in JSON")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
