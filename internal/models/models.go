// Package models defines shared domain models used across services.
package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a platform user (rider or driver).
type User struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never expose
	Name         string    `json:"name"`
	Phone        string    `json:"phone"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Driver holds driver-specific profile information.
type Driver struct {
	UserID       uuid.UUID `json:"user_id"`
	VehicleType  string    `json:"vehicle_type"`
	LicensePlate string    `json:"license_plate"`
	IsAvailable  bool      `json:"is_available"`
	Rating       float64   `json:"rating"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserWithDriver combines user and driver profiles.
type UserWithDriver struct {
	User
	Driver *Driver `json:"driver,omitempty"`
}

// RideStatus enumerates the possible ride states.
type RideStatus string

const (
	RideStatusRequested RideStatus = "requested"
	RideStatusMatched   RideStatus = "matched"
	RideStatusEnroute   RideStatus = "enroute"
	RideStatusCompleted RideStatus = "completed"
	RideStatusCancelled RideStatus = "cancelled"
)

// Ride represents a ride request and its lifecycle.
type Ride struct {
	ID              uuid.UUID  `json:"id"`
	RiderID         uuid.UUID  `json:"rider_id"`
	DriverID        *uuid.UUID `json:"driver_id,omitempty"`
	PickupLat       float64    `json:"pickup_lat"`
	PickupLong      float64    `json:"pickup_long"`
	DropoffLat      float64    `json:"dropoff_lat"`
	DropoffLong     float64    `json:"dropoff_long"`
	Status          RideStatus `json:"status"`
	Fare            float64    `json:"fare"`
	SurgeMultiplier float64    `json:"surge_multiplier"`
	EstimatedETA    *int       `json:"estimated_eta,omitempty"` // seconds
	DistanceKm      *float64   `json:"distance_km,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Location represents a GPS position update.
type Location struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Lat       float64   `json:"lat"`
	Long      float64   `json:"long"`
	Timestamp time.Time `json:"timestamp"`
}

// NearbyDriver represents a driver found within a search radius.
type NearbyDriver struct {
	DriverID     uuid.UUID `json:"driver_id"`
	Lat          float64   `json:"lat"`
	Long         float64   `json:"long"`
	DistanceKm   float64   `json:"distance_km"`
	EstimatedETA int       `json:"estimated_eta"` // seconds
}

// Payment represents a payment transaction.
type Payment struct {
	ID          uuid.UUID `json:"id"`
	RideID      uuid.UUID `json:"ride_id"`
	Amount      float64   `json:"amount"`
	Currency    string    `json:"currency"`
	Status      string    `json:"status"` // pending, completed, refunded
	StripeTxnID string    `json:"stripe_txn_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
