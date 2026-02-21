// Package ride implements the ride dispatch service with FSM, matching, and surge pricing.
package ride

import (
	"fmt"

	"github.com/hugovillarreal/ridesharing/internal/models"
)

// FSM implements a finite state machine for ride status transitions.
// Valid transitions:
//   requested → matched, cancelled
//   matched   → enroute, cancelled
//   enroute   → completed, cancelled
//   completed → (terminal)
//   cancelled → (terminal)

var validTransitions = map[models.RideStatus][]models.RideStatus{
	models.RideStatusRequested: {models.RideStatusMatched, models.RideStatusCancelled},
	models.RideStatusMatched:   {models.RideStatusEnroute, models.RideStatusCancelled},
	models.RideStatusEnroute:   {models.RideStatusCompleted, models.RideStatusCancelled},
	// Terminal states have no outgoing transitions
	models.RideStatusCompleted: {},
	models.RideStatusCancelled: {},
}

// CanTransition checks if a status transition is valid.
func CanTransition(from, to models.RideStatus) bool {
	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the transition is invalid.
func ValidateTransition(from, to models.RideStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid transition from %s to %s", from, to)
	}
	return nil
}
